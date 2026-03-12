package http

import (
	"strconv"

	"openbadge/internal/model"

	"github.com/gofiber/fiber/v2"
)

func (h *HttpHandler) CreateIssuer(c *fiber.Ctx) error {
	var req struct {
		Name        string `json:"name"`
		URL         string `json:"url"`
		Email       string `json:"email"`
		Description string `json:"description"`
		ImageURL    string `json:"image"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"resultCode": "400", "resultMessage": "FAIL",
			"error": "invalid request body: " + err.Error(),
		})
	}

	issuer := &model.Issuer{
		Name:        req.Name,
		URL:         req.URL,
		Email:       req.Email,
		Description: req.Description,
		ImageURL:    req.ImageURL,
	}

	if err := h.issuerSvc.Create(c.Context(), issuer); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"resultCode": "400", "resultMessage": "FAIL",
			"error": err.Error(),
		})
	}

	return c.Status(201).JSON(fiber.Map{
		"resultCode": "201", "resultMessage": "CREATED",
		"data": issuer,
	})
}

func (h *HttpHandler) GetIssuer(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{
			"resultCode": "400", "resultMessage": "FAIL",
			"error": "invalid id",
		})
	}

	issuer, err := h.issuerSvc.Get(c.Context(), id)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"resultCode": "500", "resultMessage": "ERROR",
			"error": err.Error(),
		})
	}
	if issuer == nil {
		return c.Status(404).JSON(fiber.Map{
			"resultCode": "404", "resultMessage": "NOT_FOUND",
			"error": "issuer not found",
		})
	}

	return c.JSON(fiber.Map{
		"resultCode": "200", "resultMessage": "SUCCESS",
		"data": issuer,
	})
}

func (h *HttpHandler) ListIssuers(c *fiber.Ctx) error {
	limit := c.QueryInt("limit", 20)
	offset := c.QueryInt("offset", 0)

	issuers, err := h.issuerSvc.List(c.Context(), limit, offset)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"resultCode": "500", "resultMessage": "ERROR",
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"resultCode": "200", "resultMessage": "SUCCESS",
		"totalCount": len(issuers),
		"data":       issuers,
	})
}

func (h *HttpHandler) UpdateIssuer(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{
			"resultCode": "400", "resultMessage": "FAIL",
			"error": "invalid id",
		})
	}

	var req struct {
		Name        string `json:"name"`
		URL         string `json:"url"`
		Email       string `json:"email"`
		Description string `json:"description"`
		ImageURL    string `json:"image"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"resultCode": "400", "resultMessage": "FAIL",
			"error": "invalid request body: " + err.Error(),
		})
	}

	issuer := &model.Issuer{
		ID:          id,
		Name:        req.Name,
		URL:         req.URL,
		Email:       req.Email,
		Description: req.Description,
		ImageURL:    req.ImageURL,
	}

	if err := h.issuerSvc.Update(c.Context(), issuer); err != nil {
		return c.Status(500).JSON(fiber.Map{
			"resultCode": "500", "resultMessage": "ERROR",
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"resultCode": "200", "resultMessage": "SUCCESS",
		"data": issuer,
	})
}

func (h *HttpHandler) DeleteIssuer(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{
			"resultCode": "400", "resultMessage": "FAIL",
			"error": "invalid id",
		})
	}

	if err := h.issuerSvc.Delete(c.Context(), id); err != nil {
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
