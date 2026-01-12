package observability

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type Publisher interface {
	PublishJSON(ctx context.Context, routingKey string, message interface{}, headers map[string]string) error
}

type AMQPPublisher struct {
	conn     *amqp.Connection
	channel  *amqp.Channel
	exchange string
}

func NewAMQPPublisher(url, exchange string) (*AMQPPublisher, error) {
	if url == "" {
		return nil, errors.New("amqp url is empty")
	}

	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, err
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, err
	}

	if err := ch.ExchangeDeclare(exchange, "topic", true, false, false, false, nil); err != nil {
		ch.Close()
		conn.Close()
		return nil, err
	}

	return &AMQPPublisher{conn: conn, channel: ch, exchange: exchange}, nil
}

func (p *AMQPPublisher) PublishJSON(ctx context.Context, routingKey string, message interface{}, headers map[string]string) error {
	body, err := json.Marshal(message)
	if err != nil {
		return err
	}

	amqpHeaders := amqp.Table{}
	for key, value := range headers {
		amqpHeaders[key] = value
	}

	return p.channel.PublishWithContext(ctx, p.exchange, routingKey, false, false, amqp.Publishing{
		ContentType:  "application/json",
		Body:         body,
		DeliveryMode: amqp.Persistent,
		Timestamp:    time.Now(),
		Headers:      amqpHeaders,
	})
}

func (p *AMQPPublisher) Close() error {
	if p.channel != nil {
		_ = p.channel.Close()
	}
	if p.conn != nil {
		return p.conn.Close()
	}
	return nil
}

var defaultPublisher Publisher

func SetPublisher(publisher Publisher) {
	defaultPublisher = publisher
}

func PublishEvent(ctx context.Context, routingKey string, message interface{}, headers map[string]string) error {
	if defaultPublisher == nil {
		return nil
	}

	err := defaultPublisher.PublishJSON(ctx, routingKey, message, headers)
	if err != nil {
		IncAMQPPublishError()
	}
	return err
}
