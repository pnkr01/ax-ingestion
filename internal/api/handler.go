package api

import (
	"ax-ingestion/internal/config"
	"ax-ingestion/pkg/models"
	"context"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type TelemetryProducer interface {
	Publish(ctx context.Context, payload models.TelemetryPayload) error
}

type Handler struct {
	producer  TelemetryProducer
	validator *validator.Validate
}

func NewHandler(producer TelemetryProducer) *Handler {
	return &Handler{
		producer:  producer,
		validator: validator.New(), // Initialize struct validator
	}
}

func (h *Handler) Ingest(c *fiber.Ctx) error {
	var payload models.TelemetryPayload

	if err := c.BodyParser(&payload); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Malformed JSON payload"})
	}

	if err := h.validator.Struct(payload); err != nil {
		config.Logger.Warn("Invalid payload rejected", zap.Error(err))
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Validation failed: " + err.Error(),
		})
	}

	payload.Timestamp = time.Now()

	// 🚨 ENTERPRISE UPGRADE: Generate a UUID if missing
	if payload.TraceID == "" {
		traceHeader := c.Get("X-Trace-ID")
		if traceHeader != "" {
			payload.TraceID = traceHeader
		} else {
			// Generate a brand new v4 UUID for this request
			payload.TraceID = uuid.New().String()
		}
	}

	payload.TenantID = c.Locals("tenantID").(string)

	if err := h.producer.Publish(c.UserContext(), payload); err != nil {
		config.Logger.Error("Failed to publish to producer", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Internal infrastructure error"})
	}

	config.Logger.Info("Telemetry event queued",
		zap.String("trace_id", payload.TraceID),
		zap.String("tenant_id", payload.TenantID),
		zap.String("tool_name", payload.ToolName),
		zap.Int("payload_size", payload.PayloadSize),
	)

	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{
		"message": "Event queued",
		"traceId": payload.TraceID, // Now this will always return a valid UUID
	})
}
