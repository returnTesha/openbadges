package http

import (
	"log/slog"

	"openbadge/internal/service"

	"github.com/gofiber/fiber/v2"
)

type HttpHandler struct {
	issuerSvc    *service.IssuerService
	badgeSvc     *service.BadgeService
	assertionSvc *service.AssertionService
	logger       *slog.Logger
}

func NewHttpHandler(
	issuerSvc *service.IssuerService,
	badgeSvc *service.BadgeService,
	assertionSvc *service.AssertionService,
	logger *slog.Logger,
) *HttpHandler {
	return &HttpHandler{
		issuerSvc:    issuerSvc,
		badgeSvc:     badgeSvc,
		assertionSvc: assertionSvc,
		logger:       logger.With("component", "http"),
	}
}

func (h *HttpHandler) RegisterRoutes(app *fiber.App) {
	app.Get("/health", h.HealthCheck)

	api := app.Group("/api/v1")

	// Issuer CRUD
	api.Post("/issuers", h.CreateIssuer)
	api.Get("/issuers", h.ListIssuers)
	api.Get("/issuers/:id", h.GetIssuer)
	api.Put("/issuers/:id", h.UpdateIssuer)
	api.Delete("/issuers/:id", h.DeleteIssuer)

	// BadgeClass CRUD
	api.Post("/badges", h.CreateBadge)
	api.Get("/badges", h.ListBadges)
	api.Get("/badges/:id", h.GetBadge)
	api.Put("/badges/:id", h.UpdateBadge)
	api.Delete("/badges/:id", h.DeleteBadge)

	// Assertion (발급/검증)
	api.Post("/assertions", h.IssueAssertion)
	api.Get("/assertions", h.ListAssertions)
	api.Get("/assertions/:id", h.GetAssertion)
	api.Post("/assertions/:id/revoke", h.RevokeAssertion)
	api.Get("/assertions/:id/verify", h.VerifyAssertion)
}

func (h *HttpHandler) HealthCheck(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{"status": "ok"})
}
