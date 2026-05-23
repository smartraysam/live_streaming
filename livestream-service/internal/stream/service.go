package stream

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/yourorg/livestream-service/internal/db"
)

type Service struct {
	store *db.Store
	ivs   IVSClient
}

func NewService(store *db.Store, ivs IVSClient) *Service {
	return &Service{store: store, ivs: ivs}
}

func (s *Service) Create(ctx context.Context, creatorID string, req CreateStreamRequest) (*Stream, error) {
	if req.StreamType != "broadcast" && req.StreamType != "private" {
		return nil, fmt.Errorf("invalid stream_type")
	}
	if req.StreamType == "private" && req.InvitedViewerID == "" {
		return nil, fmt.Errorf("invited_viewer_id required for private stream")
	}

	channelType := "STANDARD"
	if req.StreamType == "private" {
		channelType = "LOW_LATENCY"
	}
	ch, err := s.ivs.CreateChannel(ctx, req.Title, channelType)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	stream := &Stream{
		StreamID:        uuid.NewString(),
		CreatorID:       creatorID,
		StreamType:      req.StreamType,
		Title:           req.Title,
		Description:     req.Description,
		ChannelARN:      ch.ChannelARN,
		PlaybackURL:     ch.PlaybackURL,
		StreamKeyARN:    ch.StreamKeyARN,
		IsPaid:          req.IsPaid,
		TicketPriceUSD:  req.TicketPriceUSD,
		InvitedViewerID: req.InvitedViewerID,
		Status:          "IDLE",
		CreatedAt:       now,
	}
	if err := s.store.PutStream(ctx, stream.StreamID, toMap(stream)); err != nil {
		return nil, err
	}
	return stream, nil
}

func (s *Service) Get(ctx context.Context, streamID string) (*Stream, error) {
	item, err := s.store.GetStream(ctx, streamID)
	if err != nil {
		return nil, err
	}
	return fromMap(item), nil
}

func (s *Service) ListLive(ctx context.Context) ([]*Stream, error) {
	items, err := s.store.ListStreams(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*Stream, 0, len(items))
	for _, it := range items {
		st := fromMap(it)
		if st.StreamType == "broadcast" && st.Status == "LIVE" {
			out = append(out, st)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ViewerCount > out[j].ViewerCount })
	return out, nil
}

func (s *Service) ListByCreator(ctx context.Context, creatorID string) ([]*Stream, error) {
	items, err := s.store.ListStreams(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*Stream, 0)
	for _, it := range items {
		st := fromMap(it)
		if st.CreatorID == creatorID {
			out = append(out, st)
		}
	}
	return out, nil
}

func (s *Service) ListAll(ctx context.Context) ([]*Stream, error) {
	items, err := s.store.ListStreams(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*Stream, 0, len(items))
	for _, it := range items {
		out = append(out, fromMap(it))
	}
	return out, nil
}

func (s *Service) Update(ctx context.Context, streamID string, req UpdateStreamRequest) (*Stream, error) {
	st, err := s.Get(ctx, streamID)
	if err != nil {
		return nil, err
	}
	st.Title = req.Title
	st.Description = req.Description
	st.IsPaid = req.IsPaid
	st.TicketPriceUSD = req.TicketPriceUSD
	if err := s.store.PutStream(ctx, streamID, toMap(st)); err != nil {
		return nil, err
	}
	return st, nil
}

func (s *Service) Delete(ctx context.Context, streamID string) error {
	st, err := s.Get(ctx, streamID)
	if err != nil {
		return err
	}
	if err := s.ivs.DeleteChannel(ctx, st.ChannelARN); err != nil {
		return err
	}
	return s.store.DeleteStream(ctx, streamID)
}

func (s *Service) CanViewPlayback(ctx context.Context, streamID, userID string, hasTicket bool) (string, error) {
	st, err := s.Get(ctx, streamID)
	if err != nil {
		return "", err
	}
	if st.StreamType == "private" && userID != st.InvitedViewerID && userID != st.CreatorID {
		return "", fmt.Errorf("invite_required")
	}
	if st.IsPaid && !hasTicket && userID != st.CreatorID {
		return "", fmt.Errorf("ticket_required")
	}
	return st.PlaybackURL, nil
}

func toMap(s *Stream) map[string]interface{} {
	return map[string]interface{}{
		"stream_id":         s.StreamID,
		"creator_id":        s.CreatorID,
		"stream_type":       s.StreamType,
		"title":             s.Title,
		"description":       s.Description,
		"channel_arn":       s.ChannelARN,
		"playback_url":      s.PlaybackURL,
		"stream_key_arn":    s.StreamKeyARN,
		"is_paid":           s.IsPaid,
		"ticket_price_usd":  s.TicketPriceUSD,
		"invited_viewer_id": s.InvitedViewerID,
		"status":            s.Status,
		"viewer_count":      s.ViewerCount,
		"recording_s3_key":  s.RecordingS3Key,
		"created_at":        s.CreatedAt,
		"started_at":        s.StartedAt,
		"ended_at":          s.EndedAt,
	}
}

func fromMap(m map[string]interface{}) *Stream {
	st := &Stream{}
	if v, ok := m["stream_id"].(string); ok {
		st.StreamID = v
	}
	if v, ok := m["creator_id"].(string); ok {
		st.CreatorID = v
	}
	if v, ok := m["stream_type"].(string); ok {
		st.StreamType = v
	}
	if v, ok := m["title"].(string); ok {
		st.Title = v
	}
	if v, ok := m["description"].(string); ok {
		st.Description = v
	}
	if v, ok := m["channel_arn"].(string); ok {
		st.ChannelARN = v
	}
	if v, ok := m["playback_url"].(string); ok {
		st.PlaybackURL = v
	}
	if v, ok := m["stream_key_arn"].(string); ok {
		st.StreamKeyARN = v
	}
	if v, ok := m["is_paid"].(bool); ok {
		st.IsPaid = v
	}
	if v, ok := m["ticket_price_usd"].(float64); ok {
		st.TicketPriceUSD = v
	}
	if v, ok := m["invited_viewer_id"].(string); ok {
		st.InvitedViewerID = v
	}
	if v, ok := m["status"].(string); ok {
		st.Status = v
	}
	if v, ok := m["viewer_count"].(int); ok {
		st.ViewerCount = v
	}
	if v, ok := m["recording_s3_key"].(string); ok {
		st.RecordingS3Key = v
	}
	if v, ok := m["created_at"].(time.Time); ok {
		st.CreatedAt = v
	}
	if v, ok := m["started_at"].(time.Time); ok {
		st.StartedAt = v
	}
	if v, ok := m["ended_at"].(time.Time); ok {
		st.EndedAt = v
	}
	return st
}
