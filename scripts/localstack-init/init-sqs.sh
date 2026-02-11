#!/bin/bash
# Initialize LocalStack SQS queues

echo "Creating default SQS queues..."

# Create a standard queue
awslocal sqs create-queue --queue-name test-queue

# Create a FIFO queue
awslocal sqs create-queue --queue-name test-queue.fifo --attributes FifoQueue=true,ContentBasedDeduplication=true

echo "SQS queues created successfully!"

awslocal sqs list-queues
