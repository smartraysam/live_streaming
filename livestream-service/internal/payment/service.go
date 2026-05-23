package payment

import (
	"context"
	"fmt"
	"time"

	"github.com/yourorg/livestream-service/internal/chat"
	"github.com/yourorg/livestream-service/internal/db"
	"github.com/yourorg/livestream-service/internal/stream"
	"github.com/yourorg/livestream-service/pkg/laravel"
)

type Service struct {
	store   *db.Store
	streams *stream.Service
	api     laravel.Client
	hubs    *chat.HubManager
}

func NewService(store *db.Store, streams *stream.Service, api laravel.Client, hubs *chat.HubManager) *Service {
	return &Service{store: store, streams: streams, api: api, hubs: hubs}
}

func (s *Service) Tip(ctx context.Context, streamID, payerID, msg string, amount float64) (*laravel.ChargeResponse, error) {
	st, err := s.streams.Get(ctx, streamID)
	if err != nil {
		return nil, err
	}
	charge, err := s.api.ChargePayment(ctx, laravel.ChargeRequest{
		Type:        "tip",
		PayerUserID: payerID,
		PayeeUserID: st.CreatorID,
		AmountUSD:   amount,
		Metadata: map[string]string{
			"stream_id": streamID,
			"message":   msg,
		},
	})
	if err != nil {
		return nil, err
	}
	if charge.Status != "success" {
		return charge, fmt.Errorf("payment_%s", charge.Status)
	}
	_ = s.store.PutChatMessage(ctx, streamID, map[string]interface{}{
		"type":           "tip",
		"transaction_id": charge.TransactionID,
		"sender_id":      payerID,
		"creator_id":     st.CreatorID,
		"amount_usd":     amount,
		"message":        msg,
		"sent_at":        time.Now().UTC(),
	})
	if hub := s.hubs.GetOrCreate(streamID, st.StreamType == "private", st.CreatorID); hub != nil {
		hub.Publish(chat.Message{Type: "tip_alert", UserID: payerID, Body: msg, AmountUSD: amount, SentAt: time.Now().UTC()})
	}
	return charge, nil
}
