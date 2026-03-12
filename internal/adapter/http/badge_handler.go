package http

import (
	"strconv"

	"openbadge/internal/model"

	"github.com/gofiber/fiber/v2"
)

func (h *HttpHandler) CreateBadge(c *fiber.Ctx) error {
	var req struct {
		IssuerID    int64  `json:"issuerId"`
		Name        string `json:"name"`
		Description string `json:"description"`
		ImageURL    string `json:"image"`
		CriteriaURL string `json:"criteria"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"resultCode": "400", "resultMessage": "FAIL",
			"error": "invalid request body: " + err.Error(),
		})
	}

	badge := &model.BadgeClass{
		IssuerID:    req.IssuerID,
		Name:        req.Name,
		Description: req.Description,
		ImageURL:    req.ImageURL,
		CriteriaURL: req.CriteriaURL,
	}

	if err := h.badgeSvc.Create(c.Context(), badge); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"resultCode": "400", "resultMessage": "FAIL",
			"error": err.Error(),
		})
	}

	return c.Status(201).JSON(fiber.Map{
		"resultCode": "201", "resultMessage": "CREATED",
		"data": badge,
	})
}

func (h *HttpHandler) GetBadge(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{
			"resultCode": "400", "resultMessage": "FAIL",
			"error": "invalid id",
		})
	}

	badge, err := h.badgeSvc.Get(c.Context(), id)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"resultCode": "500", "resultMessage": "ERROR",
			"error": err.Error(),
		})
	}
	if badge == nil {
		return c.Status(404).JSON(fiber.Map{
			"resultCode": "404", "resultMessage": "NOT_FOUND",
			"error": "badge not found",
		})
	}

	return c.JSON(fiber.Map{
		"resultCode": "200", "resultMessage": "SUCCESS",
		"data": badge,
	})
}

func (h *HttpHandler) ListBadges(c *fiber.Ctx) error {
	issuerID := int64(c.QueryInt("issuer_id", 0))
	limit := c.QueryInt("limit", 20)
	offset := c.QueryInt("offset", 0)

	badges, err := h.badgeSvc.List(c.Context(), issuerID, limit, offset)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"resultCode": "500", "resultMessage": "ERROR",
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"resultCode": "200", "resultMessage": "SUCCESS",
		"totalCount": len(badges),
		"data":       badges,
	})
}

func (h *HttpHandler) UpdateBadge(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{
			"resultCode": "400", "resultMessage": "FAIL",
			"error": "invalid id",
		})
	}

	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		ImageURL    string `json:"image"`
		CriteriaURL string `json:"criteria"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"resultCode": "400", "resultMessage": "FAIL",
			"error": "invalid request body: " + err.Error(),
		})
	}

	badge := &model.BadgeClass{
		ID:          id,
		Name:        req.Name,
		Description: req.Description,
		ImageURL:    req.ImageURL,
		CriteriaURL: req.CriteriaURL,
	}

	if err := h.badgeSvc.Update(c.Context(), badge); err != nil {
		return c.Status(500).JSON(fiber.Map{
			"resultCode": "500", "resultMessage": "ERROR",
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"resultCode": "200", "resultMessage": "SUCCESS",
		"data": badge,
	})
}

func (h *HttpHandler) DeleteBadge(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{
			"resultCode": "400", "resultMessage": "FAIL",
			"error": "invalid id",
		})
	}

	if err := h.badgeSvc.Delete(c.Context(), id); err != nil {
		return c.Status(500).JSON(fiber.Map{
			"resultCode": "500", "resultMessage": "ERROR",
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"resultCode": "200", "resultMessage": "SUCCESS",
		"data": fiber.Map{"id": id, "deleted": true},
	})
}
