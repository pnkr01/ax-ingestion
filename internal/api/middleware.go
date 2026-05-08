package api

import (
	"ax-ingestion/internal/config"

	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// AuthMiddleware takes a Redis client and validates the API key.
func AuthMiddleware(rdb *redis.Client) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// 1. Extract the API key from the header
		apiKey := c.Get("X-API-Key")

		if apiKey == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Missing X-API-Key header",
			})
		}

		// 2. Query Redis for the Tenant ID
		// In Redis, we store keys like: "apikey:{actual_key}" -> "{tenant_id}"
		redisKey := "apikey:" + apiKey

		// Use c.Context() which is the Fasthttp context wrapped for standard library use
		tenantID, err := rdb.Get(c.Context(), redisKey).Result()

		// 3. Handle Redis Responses
		if err == redis.Nil {
			// redis.Nil explicitly means the key does not exist
			config.Logger.Warn("Unauthorized access attempt: Invalid API Key", zap.String("ip", c.IP()))
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Invalid API Key",
			})
		} else if err != nil {
			// A real infrastructure error (e.g., Redis is down)
			config.Logger.Error("Redis connection failure during auth", zap.Error(err))
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Authentication service unavailable",
			})
		}

		// 4. Securely store the Tenant ID in the request context
		c.Locals("tenantID", tenantID)

		// 5. Proceed to the next handler
		return c.Next()
	}
}
