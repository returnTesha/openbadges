// Package response provides RFC 7807 Problem Details helpers and a standard
// success envelope so every API endpoint returns a consistent JSON shape.
package response

import (
	"github.com/gofiber/fiber/v2"
)

// ProblemDetail follows RFC 7807 (application/problem+json).
type ProblemDetail struct {
	Type   string `json:"type"`
	Title  string `json:"title"`
	Status int    `json:"status"`
	Detail string `json:"detail,omitempty"`
	// Instance could carry a request-id in the future.
	Instance string `json:"instance,omitempty"`
}

// ValidationProblem extends ProblemDetail with per-field errors.
type ValidationProblem struct {
	ProblemDetail
	Errors []FieldError `json:"errors,omitempty"`
}

// FieldError describes a single validation failure.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// Envelope is the standard success wrapper.
type Envelope struct {
	Data interface{} `json:"data"`
}

// Success sends a 200 JSON response wrapped in the standard envelope.
func Success(c *fiber.Ctx, data interface{}) error {
	return c.Status(fiber.StatusOK).JSON(Envelope{Data: data})
}

// SuccessWithStatus is like Success but allows a custom HTTP status (e.g. 201).
func SuccessWithStatus(c *fiber.Ctx, status int, data interface{}) error {
	return c.Status(status).JSON(Envelope{Data: data})
}

// Error sends an RFC 7807 problem response.
func Error(c *fiber.Ctx, status int, title, detail string) error {
	return c.Status(status).JSON(ProblemDetail{
		Type:   "about:blank",
		Title:  title,
		Status: status,
		Detail: detail,
	})
}

// ValidationError sends a 422 response with per-field validation errors.
func ValidationError(c *fiber.Ctx, errors []FieldError) error {
	return c.Status(fiber.StatusUnprocessableEntity).JSON(ValidationProblem{
		ProblemDetail: ProblemDetail{
			Type:   "about:blank",
			Title:  "Validation Failed",
			Status: fiber.StatusUnprocessableEntity,
			Detail: "One or more fields failed validation.",
		},
		Errors: errors,
	})
}
