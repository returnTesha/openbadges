// Package handler contains Fiber HTTP handlers for the Badge API.

// issue.go — Badge issuance endpoint.
// Handles both async (worker-pool) and sync badge issuance flows.
package handler

import (
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"the-badge/internal/repository"
	"the-badge/internal/service"
	"the-badge/internal/worker"
	"the-badge/pkg/response"

	"github.com/google/uuid"
)

// IssueBadgeRequest is the JSON body for POST /api/v1/badges.
type IssueBadgeRequest struct {
	// CPS integration fields
	UniversityCode  string `json:"university_code"`
	ProgramID       string `json:"program_id"`
	ProgramCategory string `json:"program_category"`
	StudentID       string `json:"student_id"`

	RecipientName   string `json:"recipient_name"`
	RecipientEmail  string `json:"recipient_email"`
	AchievementID   string `json:"achievement_id,omitempty"`
	AchievementName string `json:"achievement_name"`
	AchievementDesc string `json:"achievement_desc"`
	Criteria        string `json:"criteria"`
	ImageBase64     string `json:"image_base64"`
}

// issueTimeout is the maximum duration to wait for the worker pool to finish
// processing an issuance job before returning a timeout error.
const issueTimeout = 30 * time.Second

// IssueBadge handles POST /api/v1/badges.
//
// Flow:
//  1. Parse and validate the request body.
//  2. Retrieve the worker pool from Fiber locals.
//  3. Submit an "issue" job to the pool.
//  4. Wait for the result (with timeout).
//  5. Return the signed credential JSON with HTTP 201.
func IssueBadge(c *fiber.Ctx) error {
	// 1. Parse request body.
	var req IssueBadgeRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Bad Request", "Invalid JSON body: "+err.Error())
	}

	// 2. Normalize CPS fields to uppercase.
	req.UniversityCode = strings.ToUpper(strings.TrimSpace(req.UniversityCode))
	req.ProgramCategory = strings.ToUpper(strings.TrimSpace(req.ProgramCategory))

	// 3. Validate required fields.
	var fieldErrors []response.FieldError
	if req.UniversityCode == "" {
		fieldErrors = append(fieldErrors, response.FieldError{Field: "university_code", Message: "required"})
	}
	if req.ProgramCategory == "" {
		fieldErrors = append(fieldErrors, response.FieldError{Field: "program_category", Message: "required"})
	}
	if req.AchievementName == "" {
		fieldErrors = append(fieldErrors, response.FieldError{Field: "achievement_name", Message: "required"})
	}
	if req.RecipientName == "" {
		fieldErrors = append(fieldErrors, response.FieldError{Field: "recipient_name", Message: "required"})
	}
	if req.RecipientEmail == "" {
		fieldErrors = append(fieldErrors, response.FieldError{Field: "recipient_email", Message: "required"})
	}
	if len(fieldErrors) > 0 {
		return response.ValidationError(c, fieldErrors)
	}

	// 4. Get the worker pool from Fiber locals.
	pool, ok := c.Locals("pool").(*worker.Pool)
	if !ok || pool == nil {
		return response.Error(c, fiber.StatusInternalServerError,
			"Internal Server Error", "Worker pool not available")
	}

	// 4. Submit an issue job. The job payload is the request struct itself;
	//    the worker's process function will cast it and invoke the credential builder.
	jobID := uuid.New().String()
	resultCh := pool.Submit(jobID, worker.JobTypeIssue, &req)

	// 5. Wait for the result with a timeout.
	select {
	case result := <-resultCh:
		if result.Error != nil {
			return response.Error(c, fiber.StatusInternalServerError,
				"Badge Issuance Failed", result.Error.Error())
		}
		// The result data is the signed credential map.
		return response.SuccessWithStatus(c, fiber.StatusCreated, result.Data)

	case <-time.After(issueTimeout):
		return response.Error(c, fiber.StatusGatewayTimeout,
			"Gateway Timeout",
			fmt.Sprintf("Badge issuance timed out after %s", issueTimeout))
	}
}

// IssueBadgeSync handles direct (non-pooled) badge issuance for simpler use cases,
// such as single-badge issuance from admin tools or tests.
// It expects the CredentialBuilder to be stored in Fiber locals under "credentialBuilder".
func IssueBadgeSync(c *fiber.Ctx) error {
	// Parse request body.
	var req IssueBadgeRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Bad Request", "Invalid JSON body: "+err.Error())
	}

	// Normalize CPS fields.
	req.UniversityCode = strings.ToUpper(strings.TrimSpace(req.UniversityCode))
	req.ProgramCategory = strings.ToUpper(strings.TrimSpace(req.ProgramCategory))

	// Validate required fields.
	var fieldErrors []response.FieldError
	if req.UniversityCode == "" {
		fieldErrors = append(fieldErrors, response.FieldError{Field: "university_code", Message: "required"})
	}
	if req.ProgramCategory == "" {
		fieldErrors = append(fieldErrors, response.FieldError{Field: "program_category", Message: "required"})
	}
	if req.AchievementName == "" {
		fieldErrors = append(fieldErrors, response.FieldError{Field: "achievement_name", Message: "required"})
	}
	if req.RecipientName == "" {
		fieldErrors = append(fieldErrors, response.FieldError{Field: "recipient_name", Message: "required"})
	}
	if req.RecipientEmail == "" {
		fieldErrors = append(fieldErrors, response.FieldError{Field: "recipient_email", Message: "required"})
	}
	if len(fieldErrors) > 0 {
		return response.ValidationError(c, fieldErrors)
	}

	// Get the credential builder from Fiber locals.
	builder, ok := c.Locals("credentialBuilder").(*service.CredentialBuilder)
	if !ok || builder == nil {
		return response.Error(c, fiber.StatusInternalServerError,
			"Internal Server Error", "Credential builder not available")
	}

	// Generate credential_id as {CODE}-{CATEGORY}-{year}{seq}.
	repo, _ := c.Locals("repo").(*repository.PostgresRepository)
	if repo == nil {
		return response.Error(c, fiber.StatusInternalServerError,
			"Internal Server Error", "Repository not available")
	}
	seq, err := repo.NextCredentialSequence(c.Context())
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError,
			"Badge Issuance Failed", "failed to generate credential ID: "+err.Error())
	}
	credentialID := fmt.Sprintf("%s-%s-%d%d", req.UniversityCode, req.ProgramCategory, time.Now().Year(), seq)

	// Build credential params from config stored in locals.
	params := buildCredentialParams(c, &req)
	params.CredentialID = credentialID

	// recipient_did: use student_id if available, otherwise credential_id.
	if req.StudentID != "" {
		params.RecipientDID = fmt.Sprintf("%s:users:%s", params.IssuerDID, req.StudentID)
	} else {
		params.RecipientDID = fmt.Sprintf("%s:users:%s", params.IssuerDID, credentialID)
	}

	// Build and sign the credential.
	credential, err := builder.BuildCredential(params)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError,
			"Badge Issuance Failed", err.Error())
	}

	return response.SuccessWithStatus(c, fiber.StatusCreated, credential)
}

// buildCredentialParams constructs CredentialParams from Fiber locals and the request.
func buildCredentialParams(c *fiber.Ctx, req *IssueBadgeRequest) service.CredentialParams {
	// These values are injected by middleware; fall back to sensible defaults.
	issuerDID, _ := c.Locals("issuerDID").(string)
	issuerName, _ := c.Locals("issuerName").(string)
	serverURL, _ := c.Locals("serverURL").(string)
	contractAddress, _ := c.Locals("contractAddress").(string)

	return service.CredentialParams{
		IssuerDID:       issuerDID,
		IssuerName:      issuerName,
		ServerURL:       serverURL,
		ContractAddress: contractAddress,

		AchievementID:   req.AchievementID,
		AchievementName: req.AchievementName,
		AchievementDesc: req.AchievementDesc,
		Criteria:        req.Criteria,
		ImageBase64:     req.ImageBase64,

		UniversityCode:  req.UniversityCode,
		ProgramID:       req.ProgramID,
		ProgramCategory: req.ProgramCategory,
		StudentID:       req.StudentID,

		RecipientName:  req.RecipientName,
		RecipientEmail: req.RecipientEmail,
	}
}
