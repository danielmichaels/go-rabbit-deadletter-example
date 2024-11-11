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
	//Queue string `arg:"" help:"Queue to publish to"`
	MessageCount int `arg:"" help:"Number of messages to publish"`
}

func (p *Publish) Run() error {
	//conn, err := messaging.ConnectRabbitMQ("guest", "guest", "localhost:5672", "/")
	//if err != nil {
	//	slog.Error("Failed to connect to RabbitMQ", "error", err, "type", "publisher")
	//	return err
	//}
	//defer conn.Close()

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
		if err := client.Send(ctx, "customer_events", "customers.created.se", amqp091.Publishing{
			ContentType:  "text/plain",
			DeliveryMode: amqp091.Persistent,
			Body:         []byte("customers.created.se message"),
		}); err != nil {
			slog.Error("error sending message", "error", err)
			return err
		}
		if err := client.Send(ctx, "customer_events", "customers.test", amqp091.Publishing{
			ContentType:  "text/plain",
			DeliveryMode: amqp091.Persistent,
			Body:         []byte("customers.test message"),
		}); err != nil {
			slog.Error("error sending message", "error", err)
			return err
		}
		slog.Info("sent message pair", "count", i+1)
	}

	slog.Info("rabbit client info", "client", client)
	return nil
}
