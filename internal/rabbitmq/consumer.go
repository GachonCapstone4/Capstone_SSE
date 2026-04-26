package rabbitmq

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"

	"capstone_sse/internal/config"
	"capstone_sse/internal/sse"
)

const exchangeName = "x.sse.fanout"

// incomingMessage is the envelope published by service servers to the exchange.
type incomingMessage struct {
	UserID  int64           `json:"user_id"`
	SSEType string          `json:"sse_type"`
	Data    json.RawMessage `json:"data"`
}

// StartConsumer connects to RabbitMQ, declares the temporary queue,
// binds it to x.sse.fanout, and forwards messages to the SSE hub.
// On connection loss it retries with exponential back-off (up to 30 s).
func StartConsumer(hub *sse.Hub, cfg *config.Config) {
	for {
		err := connect(hub, cfg)
		if err != nil {
			log.Printf("RabbitMQ consumer error: %v — retrying...", err)
		}
	}
}

func connect(hub *sse.Hub, cfg *config.Config) error {
	url := fmt.Sprintf("amqp://%s:%s@%s:%s/", cfg.AdminID, cfg.AdminPW, cfg.RabbitMQHost, cfg.RabbitMQPort)

	var conn *amqp.Connection
	backoff := 2 * time.Second
	for {
		var err error
		conn, err = amqp.Dial(url)
		if err == nil {
			break
		}
		log.Printf("RabbitMQ dial failed: %v — retry in %s", err, backoff)
		time.Sleep(backoff)
		if backoff < 30*time.Second {
			backoff *= 2
		}
	}
	defer conn.Close()
	log.Println("RabbitMQ connected")

	ch, err := conn.Channel()
	if err != nil {
		return fmt.Errorf("open channel: %w", err)
	}
	defer ch.Close()

	// x.sse.fanout is managed by the service infrastructure.
	// This gateway has no permission to declare it — bind directly.

	// Declare an exclusive, auto-delete temporary queue for this pod.
	queueName := fmt.Sprintf("q.sse.fanout~%s", uuid.NewString())
	q, err := ch.QueueDeclare(
		queueName,
		false, // durable
		true,  // auto-delete
		true,  // exclusive (deleted when connection closes)
		false, // no-wait
		amqp.Table{
			"x-expires": int32(1800000), // 30 min TTL as a safety net
		},
	)
	if err != nil {
		return fmt.Errorf("declare queue: %w", err)
	}
	log.Printf("Declared queue: %s", q.Name)

	// Bind queue to the fanout exchange (routing key is irrelevant for fanout).
	if err := ch.QueueBind(q.Name, "", exchangeName, false, nil); err != nil {
		return fmt.Errorf("bind queue: %w", err)
	}

	msgs, err := ch.Consume(
		q.Name,
		"",    // consumer tag
		true,  // auto-ack
		true,  // exclusive
		false, // no-local
		false, // no-wait
		nil,
	)
	if err != nil {
		return fmt.Errorf("consume: %w", err)
	}

	connClose := conn.NotifyClose(make(chan *amqp.Error, 1))

	log.Println("RabbitMQ consumer started, waiting for messages...")
	for {
		select {
		case msg, ok := <-msgs:
			if !ok {
				return fmt.Errorf("message channel closed")
			}
			log.Printf("Raw MQ Message: %s", msg.Body)
			dispatch(hub, msg.Body)
		case err := <-connClose:
			return fmt.Errorf("connection closed: %v", err)
		}
	}
}

func dispatch(hub *sse.Hub, body []byte) {
	var incoming incomingMessage
	if err := json.Unmarshal(body, &incoming); err != nil {
		log.Printf("Failed to unmarshal message: %v — body: %s", err, string(body))
		return
	}
	if incoming.UserID == 0 || incoming.SSEType == "" {
		log.Printf("Dropping message: missing user_id or sse_type — body: %s", string(body))
		return
	}
	hub.Broadcast(incoming.UserID, sse.Event{
		Type: incoming.SSEType,
		Data: incoming.Data,
	})
}
