package models

import "time"

// TelemetryPayload represents the machine-to-machine event data.
type TelemetryPayload struct {
	TraceID     string    `json:"trace_id"`
	TenantID    string    `json:"tenant_id"`
	AgentModel  string    `json:"agent_model" validate:"required"`
	ToolName    string    `json:"tool_name" validate:"required"`
	PayloadSize int       `json:"payload_size" validate:"gte=0"`
	Status      string    `json:"status" validate:"required,oneof=SUCCESS ERROR HALLUCINATION"`
	Timestamp   time.Time `json:"timestamp"`
	RawRequest  string    `json:"raw_request,omitempty"`
	RawResponse string    `json:"raw_response,omitempty"`
}
