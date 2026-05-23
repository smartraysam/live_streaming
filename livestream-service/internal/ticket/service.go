package ticket

import (
	"context"
	"fmt"
	"time"

	"github.com/yourorg/livestream-service/internal/db"
	"github.com/yourorg/livestream-service/internal/stream"
	"github.com/yourorg/livestream-service/pkg/laravel"
)

type Service struct {
	store   *db.Store
	streams *stream.Service
	api     laravel.Client
}

func NewService(store *db.Store, streams *stream.Service, api laravel.Client) *Service {
	return &Service{store: store, streams: streams, api: api}
}

func (s *Service) Purchase(ctx context.Context, streamID, viewerID string) (*laravel.ChargeResponse, error) {
	st, err := s.streams.Get(ctx, streamID)
	if err != nil {
		return nil, err
	}
	charge, err := s.api.ChargePayment(ctx, laravel.ChargeRequest{
		Type:        "ticket",
		PayerUserID: viewerID,
		PayeeUserID: st.CreatorID,
		AmountUSD:   st.TicketPriceUSD,
		Metadata: map[string]string{
			"stream_id":   streamID,
			"stream_type": st.StreamType,
		},
	})
	if err != nil {
		return nil, err
	}
	if charge.Status != "success" {
		return charge, fmt.Errorf("payment_%s", charge.Status)
	}
	if err := s.store.GrantTicket(ctx, streamID, viewerID, time.Now().Add(48*time.Hour)); err != nil {
		return nil, err
	}
	return charge, nil
}

func (s *Service) Verify(ctx context.Context, streamID, viewerID string) bool {
	return s.store.HasTicket(ctx, streamID, viewerID)
}
