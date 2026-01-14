package telemetry

import (
	"context"
	"log"
	"time"

	"github.com/google/uuid"
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

type Envelope struct {
	SchemaVersion int          `json:"schema_version"`
	EventID       string       `json:"event_id"`
	EventType     string       `json:"event_type"`
	OccurredAt    string       `json:"occurred_at"`
	Service       string       `json:"service"`
	Environment   string       `json:"environment"`
	RequestID     string       `json:"request_id"`
	UserID        *int64       `json:"user_id,omitempty"`
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

func (e *AuditEmitter) Emit(ctx context.Context, level, text, requestID string, userID *int64) {
	if e == nil || e.publisher == nil {
		return
	}

	if requestID == "" {
		requestID = uuid.NewString()
	}

	log.Printf("audit emit: level=%s request_id=%s user_id=%v text=%q", level, requestID, userID, text)
	envelope := Envelope{
		SchemaVersion: 1,
		EventID:       uuid.NewString(),
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
