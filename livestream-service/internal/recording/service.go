package recording

import (
	"context"
	"time"

	"github.com/smartraysam/livestream-service/internal/chat"
	"github.com/smartraysam/livestream-service/internal/db"
	"github.com/smartraysam/livestream-service/internal/stream"
)

type Service struct {
	store   *db.Store
	streams *stream.Service
	hubs    *chat.HubManager
}

func NewService(store *db.Store, streams *stream.Service, hubs *chat.HubManager) *Service {
	return &Service{store: store, streams: streams, hubs: hubs}
}

func (s *Service) HandleEvent(ctx context.Context, ev IVSEvent) error {
	st, err := s.streams.Get(ctx, ev.StreamID)
	if err != nil {
		return err
	}
	switch ev.Type {
	case "Stream Start":
		st.Status = "LIVE"
		st.StartedAt = time.Now().UTC()
	case "Stream End":
		st.Status = "ENDED"
		st.EndedAt = time.Now().UTC()
		if st.StreamType == "private" {
			hub := s.hubs.GetOrCreate(st.StreamID, true, st.CreatorID)
			hub.Publish(chat.Message{Type: "session_ended", SentAt: time.Now().UTC()})
		}
	case "Recording End":
		st.RecordingS3Key = ev.RecordingS3Key
	}
	return s.store.PutStream(ctx, st.StreamID, map[string]interface{}{
		"stream_id":         st.StreamID,
		"creator_id":        st.CreatorID,
		"stream_type":       st.StreamType,
		"title":             st.Title,
		"description":       st.Description,
		"channel_arn":       st.ChannelARN,
		"playback_url":      st.PlaybackURL,
		"stream_key_arn":    st.StreamKeyARN,
		"is_paid":           st.IsPaid,
		"ticket_price_usd":  st.TicketPriceUSD,
		"invited_viewer_id": st.InvitedViewerID,
		"status":            st.Status,
		"viewer_count":      st.ViewerCount,
		"recording_s3_key":  st.RecordingS3Key,
		"created_at":        st.CreatedAt,
		"started_at":        st.StartedAt,
		"ended_at":          st.EndedAt,
	})
}
