// verify_page.go — Badge verification page handler.
// Serves information about the verification endpoint.
// In the future, this could serve an HTML page for browser-based verification.
package handler

import "github.com/gofiber/fiber/v2"

// VerifyPage serves verification service information.
// For now it returns a JSON response describing the available verify endpoint.
// A future iteration will serve an HTML page where users can upload badge
// files directly in a browser.
func VerifyPage(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"service":         "The Badge Verification",
		"verify_endpoint": "/api/v1/badges/verify",
		"methods": []string{
			"POST JSON body",
			"POST file upload",
		},
		"description": "Upload a badge credential JSON to verify its authenticity. " +
			"The service resolves the issuer's DID, extracts the public key, and " +
			"validates the Ed25519 signature.",
	})
}
