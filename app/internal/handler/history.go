// history.go — Badge and verification history endpoints.
// These handlers read data through the repository interface stored in
// Fiber Locals and return RFC 7807 errors via pkg/response.
package handler

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"the-badge/internal/model"
	"the-badge/internal/repository"
	"the-badge/pkg/response"
)

// getRepo extracts the BadgeRepository from Fiber Locals.
// All handlers in this file share this helper so the cast is in one place.
func getRepo(c *fiber.Ctx) (repository.BadgeRepository, error) {
	repo, ok := c.Locals("repo").(repository.BadgeRepository)
	if !ok || repo == nil {
		return nil, fiber.NewError(
			fiber.StatusInternalServerError,
			"repository not available",
		)
	}
	return repo, nil
}

// parsePagination reads page/per_page query parameters and returns normalised
// PaginationParams.
func parsePagination(c *fiber.Ctx) model.PaginationParams {
	page, _ := strconv.Atoi(c.Query("page"))
	perPage, _ := strconv.Atoi(c.Query("per_page"))

	p := model.PaginationParams{
		Page:    page,
		PerPage: perPage,
	}
	p.Normalize()
	return p
}

// GetBadge handles GET /api/v1/badges/:id — return a single badge.
func GetBadge(c *fiber.Ctx) error {
	repo, err := getRepo(c)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError,
			"Internal Server Error", "Repository is not configured.")
	}

	idParam := c.Params("id")
	badgeID, err := uuid.Parse(idParam)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest,
			"Bad Request", "Invalid badge ID format: must be a valid UUID.")
	}

	badge, err := repo.GetBadge(c.UserContext(), badgeID)
	if err != nil {
		// In a production system we would distinguish "not found" from other
		// errors; for now any repository error is surfaced as 404.
		return response.Error(c, fiber.StatusNotFound,
			"Not Found", "Badge not found.")
	}

	return response.Success(c, model.BadgeResponseFromBadge(badge))
}

// ListBadges handles GET /api/v1/badges — paginated list with optional filters.
// Query parameters:
//   - page      (int, default 1)
//   - per_page  (int, default 20, max 100)
//   - status    (string, optional: active|revoked|expired)
//   - recipient_email (string, optional)
func ListBadges(c *fiber.Ctx) error {
	repo, err := getRepo(c)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError,
			"Internal Server Error", "Repository is not configured.")
	}

	params := parsePagination(c)

	// Optional filters.
	if status := c.Query("status"); status != "" {
		switch status {
		case "active", "revoked", "expired":
			params.Status = status
		default:
			return response.Error(c, fiber.StatusBadRequest,
				"Bad Request",
				"Invalid status filter: must be one of active, revoked, expired.")
		}
	}
	params.RecipientEmail = c.Query("recipient_email")

	badges, total, err := repo.ListBadges(c.UserContext(), params)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError,
			"Internal Server Error", "Failed to list badges.")
	}

	// Convert entities to response DTOs.
	items := make([]model.BadgeResponse, len(badges))
	for i := range badges {
		items[i] = model.BadgeResponseFromBadge(&badges[i])
	}

	return response.Success(c, model.HistoryResponse{
		Items:      items,
		TotalCount: total,
		Page:       params.Page,
		PerPage:    params.PerPage,
	})
}

// IssueHistory handles GET /api/v1/history/issues — paginated issue history.
// This is semantically the same as ListBadges but scoped to the "issue log"
// perspective (chronological order of issuance).
func IssueHistory(c *fiber.Ctx) error {
	repo, err := getRepo(c)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError,
			"Internal Server Error", "Repository is not configured.")
	}

	params := parsePagination(c)
	params.Status = c.Query("status")
	params.RecipientEmail = c.Query("recipient_email")

	badges, total, err := repo.ListBadges(c.UserContext(), params)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError,
			"Internal Server Error", "Failed to retrieve issue history.")
	}

	items := make([]model.BadgeResponse, len(badges))
	for i := range badges {
		items[i] = model.BadgeResponseFromBadge(&badges[i])
	}

	return response.Success(c, model.HistoryResponse{
		Items:      items,
		TotalCount: total,
		Page:       params.Page,
		PerPage:    params.PerPage,
	})
}

// VerificationHistory handles GET /api/v1/history/verifications — paginated
// verification log entries.
func VerificationHistory(c *fiber.Ctx) error {
	repo, err := getRepo(c)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError,
			"Internal Server Error", "Repository is not configured.")
	}

	params := parsePagination(c)

	logs, total, err := repo.ListVerificationLogs(c.UserContext(), params)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError,
			"Internal Server Error", "Failed to retrieve verification history.")
	}

	return response.Success(c, model.HistoryResponse{
		Items:      logs,
		TotalCount: total,
		Page:       params.Page,
		PerPage:    params.PerPage,
	})
}
