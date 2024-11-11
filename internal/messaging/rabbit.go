package messaging

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// RabbitClient is used to keep track of the RabbitMQ connection
type RabbitClient struct {
	// The connection that is used
	conn *amqp.Connection
	// The channel that processes/sends Messages
	ch             *amqp.Channel
	reconnectDelay time.Duration
	connectionInfo ConnectionInfo
	isConnected    bool
	done           chan bool
}

type ConnectionInfo struct {
	Username string
	Password string
	Host     string
	VHost    string
}

func NewRabbitClient(ci ConnectionInfo) (*RabbitClient, error) {
	c := &RabbitClient{
		reconnectDelay: 5 * time.Second,
		connectionInfo: ci,
		done:           make(chan bool),
	}

	if err := c.connect(); err != nil {
		return nil, err
	}

	go c.handleReconnect()

	return c, nil
}

func (rc *RabbitClient) connect() error {
	conn, err := amqp.Dial(fmt.Sprintf("amqp://%s:%s@%s/%s",
		rc.connectionInfo.Username,
		rc.connectionInfo.Password,
		rc.connectionInfo.Host,
		rc.connectionInfo.VHost,
	))
	if err != nil {
		return err
	}
	ch, err := conn.Channel()
	if err != nil {
		return err
	}

	rc.conn = conn
	rc.ch = ch
	rc.isConnected = true

	go func() {
		<-rc.conn.NotifyClose(make(chan *amqp.Error))
		rc.isConnected = false
	}()
	return nil
}

func (rc *RabbitClient) handleReconnect() {
	for {
		if !rc.isConnected {
			slog.Info("RabbitMQ connection lost, attempting to reconnect")

			for !rc.isConnected {
				if err := rc.connect(); err != nil {
					slog.Error("Failed to reconnect to RabbitMQ", "error", err)
					time.Sleep(rc.reconnectDelay)
					continue
				}
				slog.Info("Reconnected to RabbitMQ")
			}
		}
		select {
		case <-rc.done:
			return
		case <-time.After(time.Second):
		}
	}
}

//// ConnectRabbitMQ will spawn a Connection
//func ConnectRabbitMQ(username, password, host, vhost string) (*amqp.Connection, error) {
//	// Setup the Connection to RabbitMQ host using AMQP
//	conn, err := amqp.Dial(fmt.Sprintf("amqp://%s:%s@%s/%s", username, password, host, vhost))
//	if err != nil {
//		return nil, err
//	}
//	return conn, nil
//}
//
//// NewRabbitMQClient will connect and return a Rabbitclient with an open connection
//// Accepts a amqp Connection to be reused, to avoid spawning one TCP connection per concurrent client
//func NewRabbitMQClient(conn *amqp.Connection) (RabbitClient, error) {
//	// Unique, Conncurrent Server Channel to process/send messages
//	// A good rule of thumb is to always REUSE Conn across applications
//	// But spawn a new Channel per routine
//	ch, err := conn.Channel()
//	if err != nil {
//		return RabbitClient{}, err
//	}
//
//	return RabbitClient{
//		conn: conn,
//		ch:   ch,
//	}, nil
//}

// Close will close the channel
func (rc *RabbitClient) Close() error {
	return rc.ch.Close()
}

func (rc *RabbitClient) CreateExchange(
	exchangeName string,
	exchangeType string,
	durable, autodelete bool,
) error {
	return rc.ch.ExchangeDeclare(exchangeName, exchangeType, durable, autodelete, false, false, nil)

}

func (rc *RabbitClient) CreateQueueWithArgs(
	queueName string,
	durable, autodelete bool,
	args amqp.Table,
) error {
	_, err := rc.ch.QueueDeclare(queueName, durable, autodelete, false, false, args)
	return err
}
func (rc *RabbitClient) CreateQueue(queueName string, durable, autodelete bool) error {
	_, err := rc.ch.QueueDeclare(queueName, durable, autodelete, false, false, nil)
	return err
}

// CreateBinding is used to connect a queue to an Exchange using the binding rule
func (rc *RabbitClient) CreateBinding(name, binding, exchange string) error {
	// leaving nowait false, having nowait set to false wctxill cause the channel to return an error and close if it cannot bind
	// the final argument is the extra headers, but we wont be doing that now
	return rc.ch.QueueBind(name, binding, exchange, false, nil)
}

// Send is used to publish a payload onto an exchange with a given routingkey
func (rc *RabbitClient) Send(
	ctx context.Context,
	exchange, routingKey string,
	options amqp.Publishing,
) error {
	return rc.ch.PublishWithContext(ctx,
		exchange,   // exchange
		routingKey, // routing key
		// Mandatory is used when we HAVE to have the message return an error, if there is no route or queue then
		// setting this to true will make the message bounce back
		// If this is False, and the message fails to deliver, it will be dropped
		true, // mandatory
		// immediate Removed in MQ 3 or up https://blog.rabbitmq.com/posts/2012/11/breaking-things-with-rabbitmq-3-0§
		false,   // immediate
		options, // amqp publishing struct
	)
}

// Consume is a wrapper around consume, it will return a Channel that can be used to digest messages
// Queue is the name of the queue to Consume
// Consumer is a unique identifier for the service instance that is consuming, can be used to cancel etc
// autoAck is important to understand, if set to true, it will automatically Acknowledge that processing is done
// This is good, but remember that if the Process fails before completion, then an ACK is already sent, making a message lost
// if not handled properly
func (rc *RabbitClient) Consume(queue, consumer string, autoAck bool) (<-chan amqp.Delivery, error) {
	return rc.ch.Consume(queue, consumer, autoAck, false, false, false, nil)
}

// ApplyQos is used to apply qouality of service to the channel
// Prefetch count - How many messages the server will try to keep on the Channel
// prefetch Size - How many Bytes the server will try to keep on the channel
// global -- Any other Consumers on the connection in the future will apply the same rules if TRUE
func (rc *RabbitClient) ApplyQos(count, size int, global bool) error {
	// Apply Quality of Serivce
	return rc.ch.Qos(
		count,
		size,
		global,
	)
}
