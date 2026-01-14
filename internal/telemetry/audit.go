package telemetry

import (
	"context"
	"log"
	"time"
)

type Publisher interface {
	Publish(ctx context.Context, routingKey string, event any) error
	Close() error
}

type AuditEmitter struct {
	publisher   Publisher
	routingKey  string
	service     string
	environment string
}

type AuditEnvelope struct {
	SchemaVersion int          `json:"schema_version"`
	EventType     string       `json:"event_type"`
	OccurredAt    string       `json:"occurred_at"`
	Service       string       `json:"service"`
	Environment   string       `json:"environment"`
	RequestID     string       `json:"request_id"`
	UserID        *string      `json:"user_id,omitempty"`
	Payload       AuditPayload `json:"payload"`
}

type AuditPayload struct {
	Level string `json:"level"`
	Text  string `json:"text"`
}

func NewAuditEmitter(publisher Publisher, routingKey, service, environment string) *AuditEmitter {
	return &AuditEmitter{
		publisher:   publisher,
		routingKey:  routingKey,
		service:     service,
		environment: environment,
	}
}

func (e *AuditEmitter) Emit(ctx context.Context, level, text, requestID string, userID *string) {
	if e == nil || e.publisher == nil {
		return
	}

	log.Printf("audit emit: level=%s request_id=%s user_id=%v text=%q", level, requestID, userID, text)
	envelope := AuditEnvelope{
		SchemaVersion: 1,
		EventType:     "audit_log",
		OccurredAt:    time.Now().UTC().Format(time.RFC3339Nano),
		Service:       e.service,
		Environment:   e.environment,
		RequestID:     requestID,
		UserID:        userID,
		Payload: AuditPayload{
			Level: level,
			Text:  text,
		},
	}

	if err := e.publisher.Publish(ctx, e.routingKey, envelope); err != nil {
		log.Printf("audit publish failed: %v", err)
	}
}
