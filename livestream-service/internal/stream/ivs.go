package stream

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

type IVSChannel struct {
	ChannelARN   string
	PlaybackURL  string
	StreamKeyARN string
}

type IVSClient interface {
	CreateChannel(ctx context.Context, name, channelType string) (*IVSChannel, error)
	DeleteChannel(ctx context.Context, channelARN string) error
	GetStreamKey(ctx context.Context, channelARN string) (string, error)
	StopStream(ctx context.Context, channelARN string) error
}

type MockIVS struct{}

func (m *MockIVS) CreateChannel(ctx context.Context, name, channelType string) (*IVSChannel, error) {
	_ = ctx
	id := uuid.NewString()
	return &IVSChannel{
		ChannelARN:   fmt.Sprintf("arn:aws:ivs:local:000000000000:channel/%s", id),
		PlaybackURL:  fmt.Sprintf("https://playback.local/%s.m3u8", id),
		StreamKeyARN: fmt.Sprintf("arn:aws:ivs:local:000000000000:stream-key/%s", id),
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
