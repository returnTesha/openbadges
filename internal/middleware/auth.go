package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"
)

// AuthConfig 인증 미들웨어 설정
type AuthConfig struct {
	SecretKey string
	Skip      []string // 인증 스킵 경로
}

// NewAuth API Key 기반 인증 미들웨어
func NewAuth(cfg AuthConfig) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// 스킵 경로 체크
		path := c.Path()
		for _, skip := range cfg.Skip {
			if strings.HasPrefix(path, skip) {
				return c.Next()
			}
		}

		// API Key 검증
		apiKey := c.Get("X-API-Key")
		if apiKey == "" {
			apiKey = c.Query("api_key")
		}

		if apiKey == "" || apiKey != cfg.SecretKey {
			return c.Status(401).JSON(fiber.Map{
				"resultCode":    "401",
				"resultMessage": "UNAUTHORIZED",
				"error":         "invalid or missing API key",
			})
		}

		return c.Next()
	}
}
