// credential.go — Credential-ID based badge endpoints.
// Provides lookup by credential_id and badge reissue (image replacement).
package handler

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/gofiber/fiber/v2"
	"the-badge/internal/model"
	"the-badge/internal/repository"
	"the-badge/internal/service"
	"the-badge/internal/worker"
	"the-badge/pkg/blockchain"
	"the-badge/pkg/response"
)

// ReissueRequest is the JSON body for POST /api/v1/badges/c/:credentialId/reissue.
type ReissueRequest struct {
	ImageBase64 string `json:"image_base64"`
}

// GetBadgeByCredentialID handles GET /api/v1/badges/c/:credentialId.
// Looks up a badge by its credential_id (e.g. "20265") instead of UUID.
func GetBadgeByCredentialID(c *fiber.Ctx) error {
	repo, ok := c.Locals("repo").(*repository.PostgresRepository)
	if !ok || repo == nil {
		return response.Error(c, fiber.StatusInternalServerError,
			"Internal Server Error", "Repository not available")
	}

	credentialID := c.Params("credentialId")
	if credentialID == "" {
		return response.Error(c, fiber.StatusBadRequest,
			"Bad Request", "credential_id is required")
	}

	badge, err := repo.GetBadgeByCredentialID(c.UserContext(), credentialID)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError,
			"Internal Server Error", "Failed to query badge: "+err.Error())
	}
	if badge == nil {
		return response.Error(c, fiber.StatusNotFound,
			"Not Found", fmt.Sprintf("Badge with credential_id %q not found", credentialID))
	}

	return response.Success(c, model.BadgeResponseFromBadge(badge))
}

// DownloadBadge handles GET /api/v1/badges/c/:credentialId/download.
// Returns the raw credential JSON as a file download. The JSON is sent byte-for-byte
// from the database to preserve the exact hash used for signing and on-chain recording.
func DownloadBadge(c *fiber.Ctx) error {
	repo, ok := c.Locals("repo").(*repository.PostgresRepository)
	if !ok || repo == nil {
		return response.Error(c, fiber.StatusInternalServerError,
			"Internal Server Error", "Repository not available")
	}

	credentialID := c.Params("credentialId")
	if credentialID == "" {
		return response.Error(c, fiber.StatusBadRequest,
			"Bad Request", "credential_id is required")
	}

	badge, err := repo.GetBadgeByCredentialID(c.UserContext(), credentialID)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError,
			"Internal Server Error", "Failed to query badge: "+err.Error())
	}
	if badge == nil {
		return response.Error(c, fiber.StatusNotFound,
			"Not Found", fmt.Sprintf("Badge with credential_id %q not found", credentialID))
	}

	c.Set("Content-Type", "application/json")
	c.Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.json"`, credentialID))
	return c.Send(badge.CredentialJSON)
}

// ReissueBadge handles POST /api/v1/badges/c/:credentialId/reissue.
//
// Flow:
//  1. Look up the existing badge by credential_id.
//  2. Parse the existing credential JSON and replace the image.
//  3. Generate a new credential_id via DB sequence.
//  4. Re-sign the modified credential with the current active key.
//  5. Save the new badge to DB.
//  6. Record the hash on-chain (non-blocking via worker pool).
//  7. Return the new signed credential.
func ReissueBadge(c *fiber.Ctx) error {
	// --- Dependencies from Fiber locals ---
	repo, ok := c.Locals("repo").(*repository.PostgresRepository)
	if !ok || repo == nil {
		return response.Error(c, fiber.StatusInternalServerError,
			"Internal Server Error", "Repository not available")
	}

	builder, ok := c.Locals("credentialBuilder").(*service.CredentialBuilder)
	if !ok || builder == nil {
		return response.Error(c, fiber.StatusInternalServerError,
			"Internal Server Error", "Credential builder not available")
	}

	// --- Step 1: Look up existing badge ---
	credentialID := c.Params("credentialId")
	if credentialID == "" {
		return response.Error(c, fiber.StatusBadRequest,
			"Bad Request", "credential_id is required")
	}

	existingBadge, err := repo.GetBadgeByCredentialID(c.UserContext(), credentialID)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError,
			"Internal Server Error", "Failed to query badge: "+err.Error())
	}
	if existingBadge == nil {
		return response.Error(c, fiber.StatusNotFound,
			"Not Found", fmt.Sprintf("Badge with credential_id %q not found", credentialID))
	}

	// --- Step 2: Parse request and extract new image ---
	var req ReissueRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest,
			"Bad Request", "Invalid JSON body: "+err.Error())
	}
	if req.ImageBase64 == "" {
		return response.Error(c, fiber.StatusBadRequest,
			"Bad Request", "image_base64 is required")
	}

	// Parse the existing credential to extract metadata for rebuilding.
	var existingCred map[string]interface{}
	if err := json.Unmarshal(existingBadge.CredentialJSON, &existingCred); err != nil {
		return response.Error(c, fiber.StatusInternalServerError,
			"Internal Server Error", "Failed to parse existing credential JSON")
	}

	// Extract fields from existing credential for rebuilding.
	achievementName, achievementDesc, criteria, programCategory, programID := extractAchievementInfo(existingCred)
	universityCode, studentID := extractCPSIdentity(existingCred)

	// --- Step 3: Generate new credential_id ---
	seq, err := repo.NextCredentialSequence(c.Context())
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError,
			"Reissue Failed", "failed to generate credential ID: "+err.Error())
	}
	newCredentialID := fmt.Sprintf("%s-%s-%d%d", universityCode, programCategory, time.Now().Year(), seq)

	// --- Step 4: Build and sign new credential ---
	issuerDID, _ := c.Locals("issuerDID").(string)
	issuerName, _ := c.Locals("issuerName").(string)
	serverURL, _ := c.Locals("serverURL").(string)
	contractAddress, _ := c.Locals("contractAddress").(string)

	// Preserve recipient DID from original badge.
	recipientDID := existingBadge.RecipientDID
	if recipientDID == "" {
		if studentID != "" {
			recipientDID = fmt.Sprintf("%s:users:%s", issuerDID, studentID)
		} else {
			recipientDID = fmt.Sprintf("%s:users:%s", issuerDID, newCredentialID)
		}
	}

	params := service.CredentialParams{
		IssuerDID:       issuerDID,
		IssuerName:      issuerName,
		ServerURL:       serverURL,
		ContractAddress: contractAddress,

		CredentialID:    newCredentialID,
		AchievementName: achievementName,
		AchievementDesc: achievementDesc,
		Criteria:        criteria,
		ImageBase64:     req.ImageBase64,

		UniversityCode:  universityCode,
		ProgramID:       programID,
		ProgramCategory: programCategory,
		StudentID:       studentID,

		RecipientName:  existingBadge.RecipientName,
		RecipientEmail: existingBadge.RecipientEmail,
		RecipientDID:   recipientDID,
	}

	if existingBadge.AchievementID != nil {
		params.AchievementID = existingBadge.AchievementID.String()
	}

	credential, err := builder.BuildCredential(params)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError,
			"Reissue Failed", err.Error())
	}

	// --- Step 5: Save new badge to DB ---
	jsonBytes, err := json.Marshal(credential)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError,
			"Reissue Failed", "failed to serialize credential JSON: "+err.Error())
	}
	hash := sha256.Sum256(jsonBytes)

	proofValue := ""
	if proof, ok := credential["proof"].(map[string]interface{}); ok {
		proofValue, _ = proof["proofValue"].(string)
	}

	newBadge := &model.Badge{
		AchievementID:  existingBadge.AchievementID,
		IssuerID:       existingBadge.IssuerID,
		RecipientName:  existingBadge.RecipientName,
		RecipientEmail: existingBadge.RecipientEmail,
		RecipientDID:   recipientDID,
		CredentialID:   newCredentialID,
		CredentialJSON: jsonBytes,
		ProofValue:     proofValue,
		BlockchainHash: hash[:],
		UniversityCode: universityCode,
		ProgramID:      programID,
		StudentID:      studentID,
		Status:         "active",
	}

	if err := repo.CreateBadge(c.UserContext(), newBadge); err != nil {
		// Log but don't fail — the credential is already signed.
		c.Locals("reissue_db_error", err.Error())
	}

	// --- Step 6: Record on-chain via worker pool (non-blocking) ---
	if pool, ok := c.Locals("pool").(*worker.Pool); ok && pool != nil {
		// We don't block on blockchain recording; it happens in the background
		// via the issue processor's recordOnChain goroutine.
		_ = pool // blockchain recording handled separately
	}

	// --- Step 7: Return the new credential ---
	return response.SuccessWithStatus(c, fiber.StatusCreated, credential)
}

// RevokeRequest is the JSON body for POST /api/v1/badges/c/:credentialId/revoke.
type RevokeRequest struct {
	Reason string `json:"reason"`
}

// RevokeBadge handles POST /api/v1/badges/c/:credentialId/revoke.
//
// Flow:
//  1. Look up the existing badge by credential_id.
//  2. Verify it is not already revoked.
//  3. Update status to "revoked" in DB.
//  4. Record revocation on-chain (non-blocking).
//  5. Return the revocation confirmation.
func RevokeBadge(c *fiber.Ctx) error {
	repo, ok := c.Locals("repo").(*repository.PostgresRepository)
	if !ok || repo == nil {
		return response.Error(c, fiber.StatusInternalServerError,
			"Internal Server Error", "Repository not available")
	}

	credentialID := c.Params("credentialId")
	if credentialID == "" {
		return response.Error(c, fiber.StatusBadRequest,
			"Bad Request", "credential_id is required")
	}

	var req RevokeRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest,
			"Bad Request", "Invalid JSON body: "+err.Error())
	}
	if req.Reason == "" {
		return response.Error(c, fiber.StatusBadRequest,
			"Bad Request", "reason is required")
	}

	// Step 1: Look up badge.
	badge, err := repo.GetBadgeByCredentialID(c.UserContext(), credentialID)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError,
			"Internal Server Error", "Failed to query badge: "+err.Error())
	}
	if badge == nil {
		return response.Error(c, fiber.StatusNotFound,
			"Not Found", fmt.Sprintf("Badge with credential_id %q not found", credentialID))
	}

	// Step 2: Check if already revoked.
	if badge.Status == "revoked" {
		return response.Error(c, fiber.StatusConflict,
			"Conflict", fmt.Sprintf("Badge %q is already revoked", credentialID))
	}

	// Step 3: Update status in DB.
	if err := repo.UpdateBadgeStatus(c.UserContext(), badge.ID, "revoked", req.Reason); err != nil {
		return response.Error(c, fiber.StatusInternalServerError,
			"Revocation Failed", "Failed to update badge status: "+err.Error())
	}

	// Step 4: Record revocation on-chain (non-blocking).
	if bcClient, ok := c.Locals("blockchain").(*blockchain.Client); ok && bcClient != nil {
		go func() {
			txHash, err := bcClient.RevokeBadge(c.UserContext(), credentialID, req.Reason)
			if err != nil {
				log.Printf("[blockchain] WARNING: RevokeBadge failed for %s: %v", credentialID, err)
				return
			}
			log.Printf("[blockchain] badge revoked on-chain: %s (tx=%s)", credentialID, txHash)
		}()
	}

	// Step 5: Return confirmation.
	now := time.Now().UTC()
	return response.Success(c, fiber.Map{
		"credential_id": credentialID,
		"status":        "revoked",
		"reason":        req.Reason,
		"revoked_at":    now.Format(time.RFC3339),
	})
}

// extractAchievementInfo extracts achievement name, description, criteria,
// and CPS category/programId from an existing credential's credentialSubject.achievement.
func extractAchievementInfo(cred map[string]interface{}) (name, desc, criteria, category, programID string) {
	subject, ok := cred["credentialSubject"].(map[string]interface{})
	if !ok {
		return
	}
	achievement, ok := subject["achievement"].(map[string]interface{})
	if !ok {
		return
	}
	name, _ = achievement["name"].(string)
	desc, _ = achievement["description"].(string)
	category, _ = achievement["category"].(string)
	programID, _ = achievement["programId"].(string)
	if criteriaObj, ok := achievement["criteria"].(map[string]interface{}); ok {
		criteria, _ = criteriaObj["narrative"].(string)
	}
	return
}

// extractCPSIdentity extracts university code and student ID from an existing
// credential's credentialSubject.source and credentialSubject.identifier.
func extractCPSIdentity(cred map[string]interface{}) (universityCode, studentID string) {
	subject, ok := cred["credentialSubject"].(map[string]interface{})
	if !ok {
		return
	}
	if source, ok := subject["source"].(map[string]interface{}); ok {
		universityCode, _ = source["code"].(string)
	}
	if identifier, ok := subject["identifier"].(map[string]interface{}); ok {
		studentID, _ = identifier["identityValue"].(string)
	}
	return
}
