//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/Yeba-Technologies/go-api-foundry/integration/testhelpers"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/suite"
)

// RabbitMQIntegrationTestSuite validates AMQP operations against a real
// RabbitMQ container. This proves the message broker infrastructure works
// end-to-end before any application-level consumer/producer code is built.
type RabbitMQIntegrationTestSuite struct {
	suite.Suite
	conn *amqp.Connection
	ch   *amqp.Channel
}

func (suite *RabbitMQIntegrationTestSuite) SetupSuite() {
	ctx := context.Background()
	rmqContainer := testhelpers.StartRabbitMQ(ctx, suite.T())

	var err error
	suite.conn, err = amqp.Dial(rmqContainer.AmqpURL())
	suite.Require().NoError(err, "should connect to RabbitMQ")

	suite.ch, err = suite.conn.Channel()
	suite.Require().NoError(err, "should open AMQP channel")
}

func (suite *RabbitMQIntegrationTestSuite) TearDownSuite() {
	if suite.ch != nil {
		suite.ch.Close()
	}
	if suite.conn != nil {
		suite.conn.Close()
	}
}

// TestPublishAndConsume validates the full publish → consume cycle on a
// declared queue.
func (suite *RabbitMQIntegrationTestSuite) TestPublishAndConsume() {
	// Declare a test queue.
	q, err := suite.ch.QueueDeclare(
		"integration-test-queue", // name
		false,                    // durable
		true,                     // autoDelete
		false,                    // exclusive
		false,                    // noWait
		nil,                      // args
	)
	suite.Require().NoError(err)

	// Publish a message.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = suite.ch.PublishWithContext(ctx,
		"",     // exchange (default)
		q.Name, // routing key = queue name
		false,  // mandatory
		false,  // immediate
		amqp.Publishing{
			ContentType: "application/json",
			Body:        []byte(`{"event":"test","data":"hello-rabbit"}`),
		},
	)
	suite.Require().NoError(err)

	// Consume.
	msgs, err := suite.ch.Consume(
		q.Name, // queue
		"",     // consumer tag
		true,   // autoAck
		false,  // exclusive
		false,  // noLocal
		false,  // noWait
		nil,    // args
	)
	suite.Require().NoError(err)

	select {
	case msg := <-msgs:
		suite.Equal("application/json", msg.ContentType)
		suite.Equal(`{"event":"test","data":"hello-rabbit"}`, string(msg.Body))
	case <-time.After(10 * time.Second):
		suite.Fail("timed out waiting for message from RabbitMQ")
	}
}

// TestExchangeDeclareAndBind validates exchange creation and queue binding.
func (suite *RabbitMQIntegrationTestSuite) TestExchangeDeclareAndBind() {
	err := suite.ch.ExchangeDeclare(
		"test-exchange", // name
		"topic",         // kind
		false,           // durable
		true,            // autoDelete
		false,           // internal
		false,           // noWait
		nil,             // args
	)
	suite.Require().NoError(err)

	q, err := suite.ch.QueueDeclare("bound-queue", false, true, false, false, nil)
	suite.Require().NoError(err)

	err = suite.ch.QueueBind(q.Name, "events.#", "test-exchange", false, nil)
	suite.Require().NoError(err)

	// Publish via the exchange.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = suite.ch.PublishWithContext(ctx,
		"test-exchange",
		"events.user.created",
		false,
		false,
		amqp.Publishing{
			ContentType: "application/json",
			Body:        []byte(`{"user":"alice"}`),
		},
	)
	suite.Require().NoError(err)

	// Consume from bound queue.
	msgs, err := suite.ch.Consume(q.Name, "", true, false, false, false, nil)
	suite.Require().NoError(err)

	select {
	case msg := <-msgs:
		suite.Equal(`{"user":"alice"}`, string(msg.Body))
		suite.Equal("events.user.created", msg.RoutingKey)
	case <-time.After(10 * time.Second):
		suite.Fail("timed out waiting for routed message")
	}
}

func TestRabbitMQIntegrationSuite(t *testing.T) {
	suite.Run(t, new(RabbitMQIntegrationTestSuite))
}
