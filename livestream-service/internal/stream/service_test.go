package stream

import (
	"context"
	"testing"

	"github.com/smartraysam/livestream-service/internal/db"
)

func TestCreateStream(t *testing.T) {
	svc := NewService(db.NewStore("streams", "chat", "tickets"), &MockIVS{})

	_, err := svc.Create(context.Background(), "creator_1", CreateStreamRequest{StreamType: "invalid"})
	if err == nil {
		t.Fatalf("expected error for invalid stream type")
	}

	created, err := svc.Create(context.Background(), "creator_1", CreateStreamRequest{
		StreamType: "broadcast",
		Title:      "hello",
	})
	if err != nil {
		t.Fatalf("create stream: %v", err)
	}
	if created.StreamID == "" || created.PlaybackURL == "" {
		t.Fatalf("expected generated stream fields")
	}
}
