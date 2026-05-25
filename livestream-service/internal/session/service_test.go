package session

import (
	"context"
	"testing"

	"github.com/smartraysam/livestream-service/internal/db"
	"github.com/smartraysam/livestream-service/internal/stream"
	"github.com/smartraysam/livestream-service/pkg/laravel"
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

func (f *fakeLaravel) CheckStreamAccess(ctx context.Context, req laravel.StreamAccessRequest) (*laravel.StreamAccessResponse, error) {
	return &laravel.StreamAccessResponse{CanAccess: true, Reason: "following"}, nil
}

func (f *fakeLaravel) SyncStream(ctx context.Context, req laravel.SyncStreamRequest) (*laravel.SyncStreamResponse, error) {
	return &laravel.SyncStreamResponse{Synced: true, LaravelStreamID: "lv-1"}, nil
}

func TestAcceptSessionInvite(t *testing.T) {
	store := db.NewStore("streams", "chat", "tickets")
	streamSvc := stream.NewService(store, &stream.MockIVS{})
	svc := NewService(store, streamSvc, &fakeLaravel{}, nil)

	st, err := svc.Create(context.Background(), "creator_1", CreateSessionRequest{
		Title:           "private",
		InvitedViewerID: "viewer_1",
		PriceUSD:        10,
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	if err := svc.Accept(context.Background(), st.StreamID, "viewer_1"); err != nil {
		t.Fatalf("accept invite: %v", err)
	}
	if !store.HasTicket(context.Background(), st.StreamID, "viewer_1") {
		t.Fatalf("expected ticket to be granted")
	}
}
