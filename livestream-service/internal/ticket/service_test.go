package ticket

import (
	"context"
	"testing"

	"github.com/yourorg/livestream-service/internal/db"
	"github.com/yourorg/livestream-service/internal/stream"
	"github.com/yourorg/livestream-service/pkg/laravel"
)

type fakeLaravel struct{}

func (f *fakeLaravel) VerifyToken(ctx context.Context, token string) (*laravel.UserInfo, error) {
	return &laravel.UserInfo{UserID: "u1", Role: "viewer", Username: "demo"}, nil
}

func (f *fakeLaravel) ChargePayment(ctx context.Context, req laravel.ChargeRequest) (*laravel.ChargeResponse, error) {
	return &laravel.ChargeResponse{TransactionID: "tx_1", Status: "success"}, nil
}

func (f *fakeLaravel) NotifyUser(ctx context.Context, userID, eventType string, payload map[string]string) error {
	return nil
}

func TestPurchaseTicket(t *testing.T) {
	store := db.NewStore("streams", "chat", "tickets")
	streamSvc := stream.NewService(store, &stream.MockIVS{})
	_, err := streamSvc.Create(context.Background(), "creator_1", stream.CreateStreamRequest{
		StreamType:     "broadcast",
		Title:          "paid",
		IsPaid:         true,
		TicketPriceUSD: 5,
	})
	if err != nil {
		t.Fatalf("create stream: %v", err)
	}
	all, _ := streamSvc.ListAll(context.Background())
	svc := NewService(store, streamSvc, &fakeLaravel{})
	if _, err := svc.Purchase(context.Background(), all[0].StreamID, "viewer_1"); err != nil {
		t.Fatalf("purchase ticket: %v", err)
	}
	if !svc.Verify(context.Background(), all[0].StreamID, "viewer_1") {
		t.Fatalf("expected valid ticket")
	}
}
