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
		validator: validator.New(),
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

	if payload.TraceID == "" {
		traceHeader := c.Get("X-Trace-ID")
		if traceHeader != "" {
			payload.TraceID = traceHeader
		} else {
			payload.TraceID = uuid.New().String()
		}
	}

	// ==========================================
	// 🚨 FIX: Safe Context Extraction (Panic-Proof)
	// ==========================================
	tenantLocal := c.Locals("tenantSlug")
	if tenantLocal == nil {
		config.Logger.Error("CRITICAL: tenantSlug missing from request context. Did the middleware run?")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Internal infrastructure error: Missing context",
		})
	}

	tenantSlug, ok := tenantLocal.(string)
	if !ok {
		config.Logger.Error("CRITICAL: tenantSlug in context is not a string")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Internal infrastructure error: Invalid context type",
		})
	}

	// Map the extracted slug to the payload (assuming your struct still calls it TenantID for now)
	payload.TenantID = tenantSlug

	if err := h.producer.Publish(c.UserContext(), payload); err != nil {
		config.Logger.Error("Failed to publish to producer", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Internal infrastructure error"})
	}

	config.Logger.Info("Telemetry event queued",
		zap.String("trace_id", payload.TraceID),
		zap.String("tenant_slug", payload.TenantID),
		zap.String("tool_name", payload.ToolName),
		zap.Int("payload_size", payload.PayloadSize),
	)

	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{
		"message": "Event queued",
		"traceId": payload.TraceID,
	})
}

func (h *Handler) IngestBatch(c *fiber.Ctx) error {
	var batch models.BatchRequest

	if err := c.BodyParser(&batch); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Malformed JSON batch payload"})
	}

	if err := h.validator.Struct(batch); err != nil {
		config.Logger.Warn("Invalid batch payload rejected", zap.Error(err))
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Validation failed: " + err.Error(),
		})
	}

	// ==========================================
	// 🚨 FIX: Safe Context Extraction for Batch
	// ==========================================
	tenantLocal := c.Locals("tenantSlug")
	if tenantLocal == nil {
		config.Logger.Error("CRITICAL: tenantSlug missing from request context in batch.")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Internal infrastructure error: Missing context",
		})
	}

	tenantSlug, ok := tenantLocal.(string)
	if !ok {
		config.Logger.Error("CRITICAL: tenantSlug in context is not a string in batch.")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Internal infrastructure error: Invalid context type",
		})
	}

	now := time.Now()
	queuedCount := 0

	// Loop through the events and send them to Kafka
	for _, event := range batch.Events {
		event.Timestamp = now
		event.TenantID = tenantSlug

		if event.TraceID == "" {
			event.TraceID = uuid.New().String()
		}

		if err := h.producer.Publish(c.UserContext(), event); err != nil {
			config.Logger.Error("Failed to publish event from batch", zap.Error(err), zap.String("trace_id", event.TraceID))
			continue
		}
		queuedCount++
	}

	config.Logger.Info("Telemetry batch processed",
		zap.String("tenant_slug", tenantSlug),
		zap.Int("events_received", len(batch.Events)),
		zap.Int("events_queued", queuedCount),
	)

	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{
		"message": "Batch queued",
		"count":   queuedCount,
	})
}
