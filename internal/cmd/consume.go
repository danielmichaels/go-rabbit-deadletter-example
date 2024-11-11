package cmd

import (
	"context"
	"fmt"
	"github.com/danielmichaels/go-rabbit/internal/messaging"
	amqp "github.com/rabbitmq/amqp091-go"
	"golang.org/x/sync/errgroup"
	"log"
	"log/slog"
	"math/rand/v2"
	"time"
)

type Consume struct {
	Globals
}

func initCustomersQueues(client *messaging.RabbitClient) error {
	args := amqp.Table{
		"x-dead-letter-exchange":    "retry_customer_events",
		"x-dead-letter-routing-key": "retry_customer_events",
	}
	if err := client.CreateQueueWithArgs("customers_created", false, false, args); err != nil {
		slog.Error("error creating queue", "error", err, "queue", "customers_created")
		return err
	}
	if err := client.CreateQueue("customers_test", false, false); err != nil {
		slog.Error("error creating queue", "error", err, "queue", "customers_test")
		return err
	}
	if err := client.CreateBinding("customers_created", "customers.created.*", "customer_events"); err != nil {
		slog.Error(
			"error creating binding",
			"error",
			err,
			"binding",
			"customers.created.*",
			"exchange",
			"customer_events",
		)
		return err
	}
	// Create binding between the customer_events exchange and the customers-test queue
	if err := client.CreateBinding("customers_test", "customers.*", "customer_events"); err != nil {
		slog.Error(
			"error creating binding",
			"error",
			err,
			"binding",
			"customers.*",
			"exchange",
			"customer_events",
		)
		return err
	}
	return nil
}

func initDeadLetter(client *messaging.RabbitClient) error {
	// Create the dead letter exchange
	if err := client.CreateExchange("retry_customer_events", "direct", false, false); err != nil {
		slog.Error("error creating DLX exchange", "error", err)
		return err
	}

	// Create the dead letter queue
	args := amqp.Table{
		"x-dead-letter-exchange":    "retry_customer_events",
		"x-dead-letter-routing-key": "retry_customer_events",
		"x-message-ttl":             20000,
	}
	if err := client.CreateQueueWithArgs("retry_customer_events", false, false, args); err != nil {
		slog.Error("error creating DLQ", "error", err)
		return err
	}

	// Bind the DLQ to the DLX
	if err := client.CreateBinding("retry_customer_events", "retry_customer_events", "retry_customer_events"); err != nil {
		slog.Error("error creating DLQ binding", "error", err)
		return err
	}
	return nil
}

func (c *Consume) Run() error {
	logHandler := slog.NewJSONHandler(log.Writer(), &slog.HandlerOptions{
		AddSource: true,
	})
	logger := slog.New(logHandler)

	ci := messaging.ConnectionInfo{
		Username: "guest",
		Password: "guest",
		Host:     "localhost:5672",
		VHost:    "/",
	}

	client, err := messaging.NewRabbitClient(ci)
	if err != nil {
		logger.Error("Failed to connect to RabbitMQ", "error", err, "type", "consumer")
		return err
	}
	defer client.Close()
	//client, err := messaging.NewRabbitMQClient(conn)
	//if err != nil {
	//	slog.Error("Failed to create to RabbitMQ client", "error", err, "type", "consumer")
	//	return err
	//}
	//defer client.Close()

	if err := initCustomersQueues(client); err != nil {
		slog.Error("Failed to initialize queues", "error", err, "type", "consumer")
		return err
	}
	if err := initDeadLetter(client); err != nil {
		slog.Error("Failed to initialize dead letter", "error", err, "type", "consumer")
		return err
	}

	mb, err := client.Consume("customers_created", "email-service", false)
	if err != nil {
		slog.Error("Failed to consume from RabbitMQ", "error", err, "type", "consumer")
		return err
	}
	// blocking is used to block forever
	var blocking chan struct{}
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	// Create a new errgroup to handle the goroutines
	g, ctx := errgroup.WithContext(ctx)
	// set number of goroutines
	qosLimit := 10
	g.SetLimit(qosLimit)
	// Apply Qos to limit amount of messages to consume
	if err := client.ApplyQos(qosLimit, 0, true); err != nil {
		panic(err)
	}
	go func() {
		for message := range mb {
			msg := message
			g.Go(func() error {
				// Simulate some processing that might fail
				err := processMessage(msg)
				if err != nil {
					logger.Error("Failed to process message", "error", err)
					// Reject the message and don't requeue - this sends it to DLQ
					if err := msg.Reject(false); err != nil {
						logger.Error("Failed to reject message", "error", err)
						return err
					}
					return err
				}

				logger.Info("Message processed successfully", "body", string(msg.Body), "type", msg.Type, "routingKey", msg.RoutingKey)
				if err := msg.Ack(false); err != nil {
					logger.Error("Failed to ack message", "error", err)
					return err
				}
				return nil
			})
		}
	}()

	log.Println("Consuming, to close the program press CTRL+C")
	// This will block forever
	<-blocking

	return nil
}
func processMessage(msg amqp.Delivery) error {
	// Simulate processing time
	time.Sleep(500 * time.Millisecond)

	// Simulate random failures (30% chance of failure)
	if rand.Float32() < 0.3 {
		return fmt.Errorf("failed to process message: %s", string(msg.Body))
	}

	return nil
}
