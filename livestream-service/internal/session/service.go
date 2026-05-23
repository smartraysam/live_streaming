package session

import (
	"context"
	"fmt"
	"time"

	"github.com/yourorg/livestream-service/internal/db"
	"github.com/yourorg/livestream-service/internal/events"
	"github.com/yourorg/livestream-service/internal/stream"
	"github.com/yourorg/livestream-service/pkg/laravel"
)

type Service struct {
	store   *db.Store
	streams *stream.Service
	api     laravel.Client
	events  events.Publisher
}

func NewService(store *db.Store, streams *stream.Service, api laravel.Client, eventPublisher events.Publisher) *Service {
	if eventPublisher == nil {
		eventPublisher = events.NoopPublisher{}
	}
	return &Service{store: store, streams: streams, api: api, events: eventPublisher}
}

func (s *Service) Create(ctx context.Context, creatorID string, req CreateSessionRequest) (*stream.Stream, error) {
	return s.streams.Create(ctx, creatorID, stream.CreateStreamRequest{
		StreamType:      "private",
		Title:           req.Title,
		Description:     req.Description,
		IsPaid:          true,
		TicketPriceUSD:  req.PriceUSD,
		InvitedViewerID: req.InvitedViewerID,
	})
}

func (s *Service) Invite(ctx context.Context, sessionID, viewerID string) error {
	return s.api.NotifyUser(ctx, viewerID, "session_invite", map[string]string{"session_id": sessionID})
}

func (s *Service) Incoming(ctx context.Context, viewerID string) ([]*stream.Stream, error) {
	all, err := s.streams.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*stream.Stream, 0)
	for _, st := range all {
		if st.StreamType == "private" && st.InvitedViewerID == viewerID && st.Status == "IDLE" {
			out = append(out, st)
		}
	}
	return out, nil
}

func (s *Service) Accept(ctx context.Context, sessionID, viewerID string) error {
	st, err := s.streams.Get(ctx, sessionID)
	if err != nil {
		return err
	}
	if st.InvitedViewerID != viewerID {
		return fmt.Errorf("invite_required")
	}
	charge, err := s.api.ChargePayment(ctx, laravel.ChargeRequest{
		Type:        "session",
		PayerUserID: viewerID,
		PayeeUserID: st.CreatorID,
		AmountUSD:   st.TicketPriceUSD,
		Metadata: map[string]string{
			"stream_id":   st.StreamID,
			"stream_type": st.StreamType,
		},
	})
	if err != nil {
		return err
	}
	if charge.Status != "success" {
		return fmt.Errorf("payment_%s", charge.Status)
	}
	if err := s.store.GrantTicket(ctx, sessionID, viewerID, time.Now().Add(48*time.Hour)); err != nil {
		return err
	}
	_ = s.events.Publish(ctx, "session_accepted", map[string]string{
		"stream_id":      sessionID,
		"viewer_user_id": viewerID,
		"transaction_id": charge.TransactionID,
	})
	return nil
}
