// verify.go — Badge verification endpoint.
// Accepts a badge credential (as JSON body or file upload), resolves the
// issuer's DID, and verifies the Ed25519 signature.
package handler

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"the-badge/internal/model"
	"the-badge/internal/repository"
	"the-badge/internal/service"
	"the-badge/internal/worker"
	"the-badge/pkg/response"
)

// VerifyRequest is the expected JSON body for badge verification.
type VerifyRequest struct {
	CredentialJSON json.RawMessage `json:"credential_json,omitempty"`
	BadgeID        string          `json:"badge_id,omitempty"`
}

// verifyTimeout is the maximum time to wait for a worker pool verification job.
const verifyTimeout = 30 * time.Second

// VerifyBadge handles POST /api/v1/badges/verify.
//
// It accepts the credential in two formats:
//  1. JSON body with credential_json field (full credential object).
//  2. File upload (badge JSON file as multipart form data, field name "badge_file").
//
// The handler submits the verification job to the worker pool for async
// processing and waits for the result with a timeout.
func VerifyBadge(c *fiber.Ctx) error {
	// Step 1: Extract credential JSON from the request.
	credentialJSON, err := extractCredentialJSON(c)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Bad Request", err.Error())
	}

	// Step 2: Get the worker pool from Fiber Locals.
	poolRaw := c.Locals("pool")
	if poolRaw == nil {
		// Fall back to synchronous verification if no pool is available.
		return verifySynchronously(c, credentialJSON)
	}

	pool, ok := poolRaw.(*worker.Pool)
	if !ok {
		return verifySynchronously(c, credentialJSON)
	}

	// Step 3: Submit the verification job to the worker pool.
	jobID := "verify-" + time.Now().Format("20060102-150405.000")
	resultChan := pool.Submit(jobID, worker.JobTypeVerify, credentialJSON)

	// Step 4: Wait for the result with a timeout.
	select {
	case result := <-resultChan:
		if result.Error != nil {
			return response.Error(c, fiber.StatusInternalServerError,
				"Verification Failed", result.Error.Error())
		}
		return response.Success(c, result.Data)

	case <-time.After(verifyTimeout):
		return response.Error(c, fiber.StatusGatewayTimeout,
			"Verification Timeout", "badge verification timed out after "+verifyTimeout.String())
	}
}

// VerifyBadgeSync handles direct synchronous verification without the worker pool.
// This can be used for testing or when the worker pool is not available.
func VerifyBadgeSync(c *fiber.Ctx) error {
	credentialJSON, err := extractCredentialJSON(c)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Bad Request", err.Error())
	}

	return verifySynchronously(c, credentialJSON)
}

// extractCredentialJSON extracts the credential JSON from either a JSON body
// or a file upload in the request.
func extractCredentialJSON(c *fiber.Ctx) ([]byte, error) {
	contentType := string(c.Request().Header.ContentType())

	// Check for multipart file upload first.
	if isMultipart(contentType) {
		return extractFromFileUpload(c)
	}

	// Try JSON body.
	return extractFromJSONBody(c)
}

// extractFromJSONBody parses the JSON body and extracts the credential_json field.
func extractFromJSONBody(c *fiber.Ctx) ([]byte, error) {
	body := c.Body()
	if len(body) == 0 {
		return nil, errNoCredential
	}

	var req VerifyRequest
	if err := json.Unmarshal(body, &req); err != nil {
		// The body might be the raw credential itself (not wrapped in credential_json).
		// Validate it's valid JSON and use it directly.
		var raw json.RawMessage
		if jsonErr := json.Unmarshal(body, &raw); jsonErr != nil {
			return nil, errInvalidJSON
		}
		return body, nil
	}

	if len(req.CredentialJSON) > 0 {
		return []byte(req.CredentialJSON), nil
	}

	if req.BadgeID != "" {
		// TODO: Look up badge by ID from the repository and retrieve its credential JSON.
		return nil, errBadgeIDNotSupported
	}

	// The body might be the raw credential itself.
	return body, nil
}

// extractFromFileUpload extracts credential JSON from an uploaded file.
func extractFromFileUpload(c *fiber.Ctx) ([]byte, error) {
	fileHeader, err := c.FormFile("badge_file")
	if err != nil {
		return nil, errNoFileUploaded
	}

	file, err := fileHeader.Open()
	if err != nil {
		return nil, errFileOpenFailed
	}
	defer file.Close()

	const maxBadgeFileSize = 10 << 20 // 10 MB
	data, err := io.ReadAll(io.LimitReader(file, maxBadgeFileSize+1))
	if err != nil {
		return nil, errFileReadFailed
	}
	if len(data) > maxBadgeFileSize {
		return nil, errFileTooLarge
	}

	// Validate that the uploaded file is valid JSON.
	var raw json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, errInvalidFileJSON
	}

	return data, nil
}

// isMultipart checks if the content type indicates a multipart form upload.
func isMultipart(contentType string) bool {
	return len(contentType) >= 19 && contentType[:19] == "multipart/form-data"
}

// verifySynchronously runs verification directly without the worker pool.
func verifySynchronously(c *fiber.Ctx, credentialJSON []byte) error {
	var result *service.VerificationResult
	var err error

	// Try to use the injected verifier from Fiber locals.
	if verifierRaw := c.Locals("verifier"); verifierRaw != nil {
		if verifier, ok := verifierRaw.(*service.VerificationService); ok {
			result, err = verifier.VerifyCredential(c.Context(), credentialJSON)
		}
	}

	// Fallback: create a minimal resolver without blockchain integration.
	if result == nil && err == nil {
		resolver := service.NewDIDResolver("", "", nil)
		verifier := service.NewVerificationService(resolver, nil)
		result, err = verifier.VerifyCredential(c.Context(), credentialJSON)
	}

	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError,
			"Verification Error", err.Error())
	}

	// Best-effort: save verification log to DB.
	if repoRaw := c.Locals("repo"); repoRaw != nil {
		if repo, ok := repoRaw.(*repository.PostgresRepository); ok {
			saveVerificationLogSync(c.Context(), repo, result)
		}
	}

	return response.Success(c, result)
}

// saveVerificationLogSync persists a verification result to the database
// from the synchronous verification path. Errors are logged but do not
// affect the response.
func saveVerificationLogSync(ctx context.Context, repo *repository.PostgresRepository, result *service.VerificationResult) {
	logResult := "valid"
	failureReason := ""
	if !result.Valid {
		logResult = "invalid"
		if len(result.Errors) > 0 {
			failureReason = result.Errors[0]
			for _, e := range result.Errors {
				if e == "credential has been revoked" {
					logResult = "revoked"
				}
				if strings.HasPrefix(e, "credential has expired at ") {
					logResult = "expired"
				}
			}
		}
	}

	var detail json.RawMessage
	if len(result.Errors) > 0 {
		detail, _ = json.Marshal(map[string]interface{}{
			"errors": result.Errors,
		})
	}

	credentialID := ""
	if result.Credential != nil {
		credentialID, _ = result.Credential["id"].(string)
	}

	vlog := &model.VerificationLog{
		CredentialID:  credentialID,
		IssuerDID:     result.IssuerDID,
		Result:        logResult,
		FailureReason: failureReason,
		Detail:        detail,
	}

	saveCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := repo.CreateVerificationLog(saveCtx, vlog); err != nil {
		log.Printf("[verify] WARNING: failed to save verification log: %v", err)
	}
}

// Error sentinel values for credential extraction.
var (
	errNoCredential        = &verifyError{"no credential provided: send JSON body with credential_json or upload a badge_file"}
	errInvalidJSON         = &verifyError{"request body is not valid JSON"}
	errBadgeIDNotSupported = &verifyError{"badge_id lookup not yet implemented: provide the full credential_json instead"}
	errNoFileUploaded      = &verifyError{"no badge_file found in multipart upload"}
	errFileOpenFailed      = &verifyError{"failed to open uploaded file"}
	errFileReadFailed      = &verifyError{"failed to read uploaded file"}
	errFileTooLarge        = &verifyError{"uploaded file exceeds 10 MB limit"}
	errInvalidFileJSON     = &verifyError{"uploaded file is not valid JSON"}
)

// verifyError is a simple error type for verification request errors.
type verifyError struct {
	msg string
}

func (e *verifyError) Error() string {
	return e.msg
}
