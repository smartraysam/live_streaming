#!/usr/bin/env sh
set -eu

AWS_ENDPOINT_URL=${AWS_ENDPOINT_URL:-http://localhost:4566}
AWS_REGION=${AWS_REGION:-us-east-1}
DYNAMODB_TABLE_STREAMS=${DYNAMODB_TABLE_STREAMS:-streams}
DYNAMODB_TABLE_CHAT=${DYNAMODB_TABLE_CHAT:-chat_messages}
DYNAMODB_TABLE_TICKETS=${DYNAMODB_TABLE_TICKETS:-tickets}
SQS_QUEUE_NAME=${SQS_QUEUE_NAME:-livestream-events}

aws_local() {
  aws --endpoint-url "$AWS_ENDPOINT_URL" --region "$AWS_REGION" "$@"
}

echo "Creating DynamoDB table: $DYNAMODB_TABLE_STREAMS"
aws_local dynamodb create-table \
  --table-name "$DYNAMODB_TABLE_STREAMS" \
  --attribute-definitions AttributeName=stream_id,AttributeType=S \
  --key-schema AttributeName=stream_id,KeyType=HASH \
  --billing-mode PAY_PER_REQUEST >/dev/null 2>&1 || true

echo "Creating DynamoDB table: $DYNAMODB_TABLE_CHAT"
aws_local dynamodb create-table \
  --table-name "$DYNAMODB_TABLE_CHAT" \
  --attribute-definitions AttributeName=stream_id,AttributeType=S AttributeName=message_id,AttributeType=S \
  --key-schema AttributeName=stream_id,KeyType=HASH AttributeName=message_id,KeyType=RANGE \
  --billing-mode PAY_PER_REQUEST >/dev/null 2>&1 || true

echo "Creating DynamoDB table: $DYNAMODB_TABLE_TICKETS"
aws_local dynamodb create-table \
  --table-name "$DYNAMODB_TABLE_TICKETS" \
  --attribute-definitions AttributeName=stream_id,AttributeType=S AttributeName=viewer_user_id,AttributeType=S \
  --key-schema AttributeName=stream_id,KeyType=HASH AttributeName=viewer_user_id,KeyType=RANGE \
  --billing-mode PAY_PER_REQUEST >/dev/null 2>&1 || true

echo "Creating SQS queue: $SQS_QUEUE_NAME"
QUEUE_URL=$(aws_local sqs create-queue --queue-name "$SQS_QUEUE_NAME" --query QueueUrl --output text)

echo "LocalStack bootstrap complete"
echo "SQS_QUEUE_URL=$QUEUE_URL"
