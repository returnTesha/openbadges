package middleware

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"the-badge/pkg/response"
)

// RateLimit returns a Fiber rate-limiter middleware.
// Default: 100 requests per minute per IP.
func RateLimit(max int) fiber.Handler {
	if max <= 0 {
		max = 100
	}

	return limiter.New(limiter.Config{
		Max:        max,
		Expiration: 1 * time.Minute,
		LimitReached: func(c *fiber.Ctx) error {
			return response.Error(c, fiber.StatusTooManyRequests,
				"Rate Limit Exceeded",
				"You have exceeded the allowed number of requests. Try again later.",
			)
		},
	})
}
