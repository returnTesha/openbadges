package http

import (
	"strconv"
	"time"

	"openbadge/internal/model"

	"github.com/gofiber/fiber/v2"
)

func (h *HttpHandler) IssueAssertion(c *fiber.Ctx) error {
	var req struct {
		BadgeClassID int64  `json:"badgeClassId"`
		RecipientID  int64  `json:"recipientId"`
		ExpiresAt    string `json:"expiresAt,omitempty"`
		Evidence     string `json:"evidence,omitempty"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"resultCode": "400", "resultMessage": "FAIL",
			"error": "invalid request body: " + err.Error(),
		})
	}

	assertion := &model.Assertion{
		BadgeClassID: req.BadgeClassID,
		RecipientID:  req.RecipientID,
		Evidence:     req.Evidence,
	}

	if req.ExpiresAt != "" {
		t, err := time.Parse(time.RFC3339, req.ExpiresAt)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{
				"resultCode": "400", "resultMessage": "FAIL",
				"error": "invalid expiresAt format (use ISO8601)",
			})
		}
		assertion.ExpiresAt = &t
	}

	if err := h.assertionSvc.Issue(c.Context(), assertion); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"resultCode": "400", "resultMessage": "FAIL",
			"error": err.Error(),
		})
	}

	return c.Status(201).JSON(fiber.Map{
		"resultCode": "201", "resultMessage": "CREATED",
		"data": assertion,
	})
}

func (h *HttpHandler) GetAssertion(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{
			"resultCode": "400", "resultMessage": "FAIL",
			"error": "invalid id",
		})
	}

	assertion, err := h.assertionSvc.Get(c.Context(), id)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"resultCode": "500", "resultMessage": "ERROR",
			"error": err.Error(),
		})
	}
	if assertion == nil {
		return c.Status(404).JSON(fiber.Map{
			"resultCode": "404", "resultMessage": "NOT_FOUND",
			"error": "assertion not found",
		})
	}

	return c.JSON(fiber.Map{
		"resultCode": "200", "resultMessage": "SUCCESS",
		"data": assertion,
	})
}

func (h *HttpHandler) ListAssertions(c *fiber.Ctx) error {
	badgeClassID := int64(c.QueryInt("badge_class_id", 0))
	recipientID := int64(c.QueryInt("recipient_id", 0))
	limit := c.QueryInt("limit", 20)
	offset := c.QueryInt("offset", 0)

	assertions, err := h.assertionSvc.List(c.Context(), badgeClassID, recipientID, limit, offset)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"resultCode": "500", "resultMessage": "ERROR",
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"resultCode": "200", "resultMessage": "SUCCESS",
		"totalCount": len(assertions),
		"data":       assertions,
	})
}

func (h *HttpHandler) RevokeAssertion(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{
			"resultCode": "400", "resultMessage": "FAIL",
			"error": "invalid id",
		})
	}

	var req struct {
		Reason string `json:"reason"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"resultCode": "400", "resultMessage": "FAIL",
			"error": "invalid request body: " + err.Error(),
		})
	}

	if err := h.assertionSvc.Revoke(c.Context(), id, req.Reason); err != nil {
		return c.Status(500).JSON(fiber.Map{
			"resultCode": "500", "resultMessage": "ERROR",
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"resultCode": "200", "resultMessage": "SUCCESS",
		"data": fiber.Map{"id": id, "revoked": true},
	})
}

func (h *HttpHandler) VerifyAssertion(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{
			"resultCode": "400", "resultMessage": "FAIL",
			"error": "invalid id",
		})
	}

	result, err := h.assertionSvc.Verify(c.Context(), id)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"resultCode": "500", "resultMessage": "ERROR",
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"resultCode": "200", "resultMessage": "SUCCESS",
		"data": result,
	})
}
