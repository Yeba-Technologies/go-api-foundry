//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/Yeba-Technologies/go-api-foundry/integration/testhelpers"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/stretchr/testify/suite"
)

// SQSIntegrationTestSuite validates SQS operations against a real LocalStack
// container. This proves the queue infrastructure works end-to-end before any
// application-level producer/consumer code is built on top.
type SQSIntegrationTestSuite struct {
	suite.Suite
	client   *sqs.Client
	queueURL string
}

func (suite *SQSIntegrationTestSuite) SetupSuite() {
	ctx := context.Background()
	lsContainer := testhelpers.StartLocalStack(ctx, suite.T())

	suite.client = sqs.New(sqs.Options{
		Region:      "us-east-1",
		BaseEndpoint: aws.String(lsContainer.Endpoint()),
		Credentials: credentials.NewStaticCredentialsProvider("test", "test", ""),
	})

	// Create a test queue.
	out, err := suite.client.CreateQueue(ctx, &sqs.CreateQueueInput{
		QueueName: aws.String("integration-test-queue"),
	})
	suite.Require().NoError(err)
	suite.queueURL = *out.QueueUrl
}

// TestSendAndReceiveMessage validates the full send → receive → delete cycle.
func (suite *SQSIntegrationTestSuite) TestSendAndReceiveMessage() {
	ctx := context.Background()

	// Send
	_, err := suite.client.SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl:    aws.String(suite.queueURL),
		MessageBody: aws.String(`{"event":"test","data":"hello"}`),
	})
	suite.Require().NoError(err)

	// Receive
	recv, err := suite.client.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
		QueueUrl:            aws.String(suite.queueURL),
		MaxNumberOfMessages: 1,
		WaitTimeSeconds:     5,
	})
	suite.Require().NoError(err)
	suite.Require().Len(recv.Messages, 1)
	suite.Equal(`{"event":"test","data":"hello"}`, *recv.Messages[0].Body)

	// Delete
	_, err = suite.client.DeleteMessage(ctx, &sqs.DeleteMessageInput{
		QueueUrl:      aws.String(suite.queueURL),
		ReceiptHandle: recv.Messages[0].ReceiptHandle,
	})
	suite.Require().NoError(err)
}

// TestCreateFIFOQueue validates FIFO queue creation with content-based dedup.
func (suite *SQSIntegrationTestSuite) TestCreateFIFOQueue() {
	ctx := context.Background()

	out, err := suite.client.CreateQueue(ctx, &sqs.CreateQueueInput{
		QueueName: aws.String("test-fifo-queue.fifo"),
		Attributes: map[string]string{
			"FifoQueue":                 "true",
			"ContentBasedDeduplication": "true",
		},
	})
	suite.Require().NoError(err)
	suite.Contains(*out.QueueUrl, "test-fifo-queue.fifo")
}

// TestListQueues verifies the queue created in SetupSuite is discoverable.
func (suite *SQSIntegrationTestSuite) TestListQueues() {
	ctx := context.Background()

	out, err := suite.client.ListQueues(ctx, &sqs.ListQueuesInput{})
	suite.Require().NoError(err)
	suite.GreaterOrEqual(len(out.QueueUrls), 1, "should have at least the test queue")
}

func TestSQSIntegrationSuite(t *testing.T) {
	suite.Run(t, new(SQSIntegrationTestSuite))
}
