// Package router registers all API routes and associates them with handlers.
package router

import (
	"github.com/gofiber/fiber/v2"
	"the-badge/internal/handler"
	"the-badge/pkg/response"
)

// Setup wires every route into the Fiber app.
// Auth middleware is applied at the app level, so this function only needs to
// define the route->handler mapping. Public endpoints are whitelisted in the
// middleware configuration (see main.go).
func Setup(app *fiber.App) {
	// --- Top-level public endpoints ---

	app.Get("/health", healthCheck)

	// DID Document — served at the well-known endpoint for did:web resolution.
	// Uses the real handler that reads the signer from Fiber locals.
	app.Get("/.well-known/did.json", handler.DIDDocument)

	// --- Versioned API group ---

	v1 := app.Group("/api/v1")

	// Badges
	v1.Post("/badges", handler.IssueBadge)
	v1.Post("/badges/verify", handler.VerifyBadge)
	v1.Post("/badges/verify-sync", handler.VerifyBadgeSync)
	v1.Get("/badges/verify", handler.VerifyPage)
	v1.Get("/badges/c/:credentialId", handler.GetBadgeByCredentialID)
	v1.Get("/badges/c/:credentialId/download", handler.DownloadBadge)
	v1.Post("/badges/c/:credentialId/reissue", handler.ReissueBadge)
	v1.Post("/badges/c/:credentialId/revoke", handler.RevokeBadge)
	v1.Get("/badges/:id", handler.GetBadge)
	v1.Get("/badges", handler.ListBadges)

	// History
	v1.Get("/history/issues", handler.IssueHistory)
	v1.Get("/history/verifications", handler.VerificationHistory)
}

// healthCheck returns a simple liveness probe.
func healthCheck(c *fiber.Ctx) error {
	return response.Success(c, fiber.Map{"status": "ok"})
}
