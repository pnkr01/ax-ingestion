package main

import (
	"ax-ingestion/internal/api"
	"ax-ingestion/internal/config"
	"ax-ingestion/internal/kafka"
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ansrivas/fiberprometheus/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

func main() {
	cfg := config.InitConfigAndLogger()
	defer config.Logger.Sync()

	// 🚨 INITIALIZE REDIS
	rdb := redis.NewClient(&redis.Options{
		Addr: cfg.RedisURL,
	})

	// Fail fast if Redis is down
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		config.Logger.Fatal("Failed to connect to Redis", zap.Error(err))
	}
	config.Logger.Info("Connected to Redis successfully", zap.String("url", cfg.RedisURL))

	// Initialize Kafka & Handlers
	producer := kafka.NewProducer(cfg)
	handler := api.NewHandler(producer)

	app := fiber.New(fiber.Config{
		AppName:               "AX-Ingestion-Edge",
		ReadTimeout:           10 * time.Second,
		WriteTimeout:          10 * time.Second,
		DisableStartupMessage: true,
	})

	prometheus := fiberprometheus.New("ax_ingestion_edge")
	prometheus.RegisterAt(app, "/metrics")
	app.Use(prometheus.Middleware)

	app.Get("/health", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	apiLimiter := limiter.New(limiter.Config{
		Max:        200,
		Expiration: 10 * time.Second,
		KeyGenerator: func(c *fiber.Ctx) string {
			return c.Get("X-API-Key")
		},
		LimitReached: func(c *fiber.Ctx) error {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error": "Too many requests. Please slow down.",
			})
		},
	})

	v1 := app.Group("/v1")

	// 🚨 INJECT REDIS INTO THE MIDDLEWARE HERE
	v1.Post("/ingest", apiLimiter, api.AuthMiddleware(rdb), handler.Ingest)

	go func() {
		config.Logger.Info("Starting server", zap.String("port", cfg.AppPort))
		if err := app.Listen(":" + cfg.AppPort); err != nil {
			config.Logger.Fatal("Failed to start server", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	config.Logger.Info("Shutting down server gracefully...")
	_, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := producer.Close(); err != nil {
		config.Logger.Error("Error closing Kafka producer", zap.Error(err))
	}

	// 🚨 CLOSE REDIS GRACEFULLY
	if err := rdb.Close(); err != nil {
		config.Logger.Error("Error closing Redis connection", zap.Error(err))
	}

	if err := app.Shutdown(); err != nil {
		config.Logger.Fatal("Server forced to shutdown", zap.Error(err))
	}
	config.Logger.Info("Server stopped successfully.")
}
