package rabbitmq

import (
	"context"
	"encoding/json"
	"log"

	amqp "github.com/rabbitmq/amqp091-go"

	"chat-service/internal/telemetry"
)

// Publisher publishes audit events.
type Publisher interface {
	Publish(ctx context.Context, routingKey string, event any) error
	Close() error
}

// NewPublisher builds a RabbitMQ publisher or a noop publisher when AMQP is disabled.
func NewPublisher(amqpURL, exchange string) Publisher {
	if amqpURL == "" {
		log.Printf("rabbitmq disabled, using noop: empty amqp url")
		return noopPublisher{reason: "empty amqp url"}
	}

	conn, err := amqp.Dial(amqpURL)
	if err != nil {
		log.Printf("rabbitmq disabled, using noop: %v", err)
		return noopPublisher{reason: err.Error()}
	}

	ch, err := conn.Channel()
	if err != nil {
		log.Printf("rabbitmq disabled, using noop: %v", err)
		_ = conn.Close()
		return noopPublisher{reason: err.Error()}
	}

	if err := ch.ExchangeDeclare(
		exchange,
		"topic",
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		log.Printf("rabbitmq disabled, using noop: %v", err)
		_ = ch.Close()
		_ = conn.Close()
		return noopPublisher{reason: err.Error()}
	}

	log.Printf("rabbitmq connected exchange=%s", exchange)
	return &amqpPublisher{conn: conn, ch: ch, exchange: exchange}
}

type amqpPublisher struct {
	conn     *amqp.Connection
	ch       *amqp.Channel
	exchange string
}

func (p *amqpPublisher) Publish(ctx context.Context, routingKey string, event any) error {
	body, err := json.Marshal(event)
	if err != nil {
		return err
	}

	err = p.ch.PublishWithContext(ctx, p.exchange, routingKey, false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Body:         body,
	})
	if err != nil {
		log.Printf("rabbitmq publish failed: %v", err)
	}
	return err
}

func (p *amqpPublisher) Close() error {
	if p.ch != nil {
		_ = p.ch.Close()
	}
	if p.conn != nil {
		return p.conn.Close()
	}
	return nil
}

type noopPublisher struct {
	reason string
}

func (noopPublisher) Publish(ctx context.Context, routingKey string, event any) error {
	switch envelope := event.(type) {
	case telemetry.AuditEnvelope:
		log.Printf("rabbitmq noop publish routing_key=%s event_type=%s service=%s request_id=%s", routingKey, envelope.EventType, envelope.Service, envelope.RequestID)
	case *telemetry.AuditEnvelope:
		log.Printf("rabbitmq noop publish routing_key=%s event_type=%s service=%s request_id=%s", routingKey, envelope.EventType, envelope.Service, envelope.RequestID)
	default:
		log.Printf("rabbitmq noop publish routing_key=%s", routingKey)
	}
	return nil
}

func (noopPublisher) Close() error {
	return nil
}

// PublisherMode reports the publisher mode for logging.
func PublisherMode(p Publisher) string {
	switch p.(type) {
	case *amqpPublisher:
		return "amqp"
	case noopPublisher:
		return "noop"
	case *noopPublisher:
		return "noop"
	default:
		return "unknown"
	}
}

func PublisherNoopReason(p Publisher) string {
	switch publisher := p.(type) {
	case noopPublisher:
		return publisher.reason
	case *noopPublisher:
		return publisher.reason
	default:
		return ""
	}
}
