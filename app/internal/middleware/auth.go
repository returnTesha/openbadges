// Package middleware provides Fiber middleware for authentication, rate limiting,
// and request logging.
package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"
)

// Auth returns a middleware that validates Bearer tokens from the Authorization
// header. Currently bypassed for MVP — all requests pass through.
// TODO: Implement JWT or university SSO authentication post-MVP.
//
// Routes whose paths appear in skipPaths are passed through without checking.
func Auth(skipPaths ...string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// MVP: authentication disabled. If a Bearer token is present,
		// store it in locals for downstream handlers but do not enforce.
		auth := c.Get("Authorization")
		if strings.HasPrefix(auth, "Bearer ") {
			token := strings.TrimPrefix(auth, "Bearer ")
			if strings.TrimSpace(token) != "" {
				c.Locals("token", token)
			}
		}

		return c.Next()
	}
}
