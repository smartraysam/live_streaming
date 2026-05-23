package events

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

type Publisher interface {
	Publish(ctx context.Context, eventType string, payload map[string]string) error
}

type NoopPublisher struct{}

func (n NoopPublisher) Publish(ctx context.Context, eventType string, payload map[string]string) error {
	_ = ctx
	_ = eventType
	_ = payload
	return nil
}

type SQSPublisher struct {
	client   *sqs.Client
	queueURL string
}

func NewSQSPublisher(ctx context.Context, region, endpointURL, queueURL string) (*SQSPublisher, error) {
	opts := []func(*awsconfig.LoadOptions) error{awsconfig.WithRegion(region)}
	if endpointURL != "" {
		opts = append(opts, awsconfig.WithBaseEndpoint(endpointURL))
	}

	cfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}
	return &SQSPublisher{client: sqs.NewFromConfig(cfg), queueURL: queueURL}, nil
}

func (p *SQSPublisher) Publish(ctx context.Context, eventType string, payload map[string]string) error {
	msg := map[string]interface{}{
		"type":    eventType,
		"payload": payload,
	}
	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}
	_, err = p.client.SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl:    aws.String(p.queueURL),
		MessageBody: aws.String(string(body)),
	})
	if err != nil {
		return fmt.Errorf("send sqs message: %w", err)
	}
	return nil
}
