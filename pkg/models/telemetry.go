package models

import "time"

// TelemetryPayload represents the machine-to-machine event data.
// Using specific struct tags for JSON and database compatibility.
type TelemetryPayload struct {
	TraceID     string    `json:"trace_id"`
	TenantID    string    `json:"tenant_id"`
	AgentModel  string    `json:"agent_model"`  // e.g., GPT-4o, Claude 3.5[cite: 1]
	ToolName    string    `json:"tool_name"`    // e.g., search_inventory[cite: 1]
	PayloadSize int       `json:"payload_size"` // Measured in tokens[cite: 1]
	Status      string    `json:"status"`       // SUCCESS, ERROR, HALLUCINATION[cite: 1]
	Timestamp   time.Time `json:"timestamp"`
	RawRequest  string    `json:"raw_request,omitempty"`
	RawResponse string    `json:"raw_response,omitempty"`
}
