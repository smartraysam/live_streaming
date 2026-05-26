package stream

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	awsi "github.com/aws/aws-sdk-go-v2/service/ivs"
	"github.com/aws/aws-sdk-go-v2/service/ivs/types"
	"github.com/aws/smithy-go"
	"github.com/google/uuid"
)

type IVSChannel struct {
	ChannelARN     string
	IngestEndpoint string
	PlaybackURL    string
	StreamKeyARN   string
}

type IVSClient interface {
	CreateChannel(ctx context.Context, name, channelType string) (*IVSChannel, error)
	DeleteChannel(ctx context.Context, channelARN string) error
	GetStreamKey(ctx context.Context, channelARN string) (string, error)
	StopStream(ctx context.Context, channelARN string) error
}

type MockIVS struct{}

type AWSIVS struct {
	client *awsi.Client
}

func NewAWSIVS(ctx context.Context, region string) (*AWSIVS, error) {
	cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}
	return &AWSIVS{client: awsi.NewFromConfig(cfg)}, nil
}

func (a *AWSIVS) CreateChannel(ctx context.Context, name, channelType string) (*IVSChannel, error) {
	latencyMode := types.ChannelLatencyModeNormalLatency
	if channelType == "LOW_LATENCY" {
		latencyMode = types.ChannelLatencyModeLowLatency
	}
	out, err := a.client.CreateChannel(ctx, &awsi.CreateChannelInput{
		Name:        aws.String(name),
		LatencyMode: latencyMode,
	})
	if err != nil {
		return nil, fmt.Errorf("create IVS channel: %w", err)
	}
	return &IVSChannel{
		ChannelARN:     aws.ToString(out.Channel.Arn),
		IngestEndpoint: aws.ToString(out.Channel.IngestEndpoint),
		PlaybackURL:    aws.ToString(out.Channel.PlaybackUrl),
		StreamKeyARN:   aws.ToString(out.StreamKey.Arn),
	}, nil
}

func (a *AWSIVS) DeleteChannel(ctx context.Context, channelARN string) error {
	_, err := a.client.DeleteChannel(ctx, &awsi.DeleteChannelInput{Arn: aws.String(channelARN)})
	if err != nil {
		return fmt.Errorf("delete IVS channel: %w", err)
	}
	return nil
}

func (a *AWSIVS) GetStreamKey(ctx context.Context, channelARN string) (string, error) {
	out, err := a.client.ListStreamKeys(ctx, &awsi.ListStreamKeysInput{
		ChannelArn: aws.String(channelARN),
		MaxResults: aws.Int32(1),
	})
	if err != nil {
		return "", fmt.Errorf("list stream keys: %w", err)
	}
	if len(out.StreamKeys) == 0 {
		return "", fmt.Errorf("stream key not found")
	}
	key, err := a.client.GetStreamKey(ctx, &awsi.GetStreamKeyInput{Arn: out.StreamKeys[0].Arn})
	if err != nil {
		return "", fmt.Errorf("get stream key: %w", err)
	}
	return aws.ToString(key.StreamKey.Value), nil
}

func (a *AWSIVS) StopStream(ctx context.Context, channelARN string) error {
	_, err := a.client.StopStream(ctx, &awsi.StopStreamInput{ChannelArn: aws.String(channelARN)})
	if err != nil {
		return fmt.Errorf("stop stream: %w", err)
	}
	return nil
}

func (a *AWSIVS) IsChannelLive(ctx context.Context, channelARN string) (bool, error) {
	_, err := a.client.GetStream(ctx, &awsi.GetStreamInput{ChannelArn: aws.String(channelARN)})
	if err == nil {
		return true, nil
	}

	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "ChannelNotBroadcasting", "ResourceNotFoundException":
			return false, nil
		}
	}

	return false, fmt.Errorf("check channel live state: %w", err)
}

func (m *MockIVS) CreateChannel(ctx context.Context, name, channelType string) (*IVSChannel, error) {
	_ = ctx
	id := uuid.NewString()
	return &IVSChannel{
		ChannelARN:     fmt.Sprintf("arn:aws:ivs:local:000000000000:channel/%s", id),
		IngestEndpoint: fmt.Sprintf("%s.global-contribute.live-video.net", id),
		PlaybackURL:    "https://test-streams.mux.dev/x36xhzz/x36xhzz.m3u8",
		StreamKeyARN:   fmt.Sprintf("arn:aws:ivs:local:000000000000:stream-key/%s", id),
	}, nil
}

func (m *MockIVS) DeleteChannel(ctx context.Context, channelARN string) error {
	_ = ctx
	_ = channelARN
	return nil
}

func (m *MockIVS) GetStreamKey(ctx context.Context, channelARN string) (string, error) {
	_ = ctx
	return fmt.Sprintf("sk_%s", channelARN), nil
}

func (m *MockIVS) StopStream(ctx context.Context, channelARN string) error {
	_ = ctx
	_ = channelARN
	return nil
}

func (m *MockIVS) IsChannelLive(ctx context.Context, channelARN string) (bool, error) {
	_ = ctx
	_ = channelARN
	return true, nil
}
