package cmd

import (
	"context"
	"github.com/danielmichaels/go-rabbit/internal/messaging"
	"github.com/rabbitmq/amqp091-go"
	"log/slog"
	"time"
)

type Publish struct {
	Globals
	MessageCount int `arg:"" help:"Number of messages to publish"`
}

func (p *Publish) Run() error {
	ci := messaging.ConnectionInfo{
		Username: "guest",
		Password: "guest",
		Host:     "localhost:5672",
		VHost:    "/",
	}
	client, err := messaging.NewRabbitClient(ci)
	if err != nil {
		slog.Error("Failed to create to RabbitMQ client", "error", err, "type", "publisher")
		return err
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for i := 0; i < p.MessageCount; i++ {
		if err := client.Send(ctx, "customer_events", "customers.created.au", amqp091.Publishing{
			ContentType:  "text/plain",
			DeliveryMode: amqp091.Persistent,
			Body:         []byte("customers.created.au message"),
		}); err != nil {
			slog.Error("error sending message", "error", err)
			return err
		}
		slog.Info("sent message", "count", i+1)
	}

	slog.Info("rabbit client info", "client", client)
	return nil
}
