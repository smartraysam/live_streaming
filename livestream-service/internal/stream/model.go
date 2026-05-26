package stream

import "time"

type Stream struct {
	StreamID        string    `json:"stream_id" example:"1c95b9af-0f52-469b-9fa6-7d7375f26de8"`
	CreatorID       string    `json:"creator_id" example:"user_123"`
	StreamType      string    `json:"stream_type" example:"broadcast"`
	Title           string    `json:"title" example:"Sunset coding session"`
	Description     string    `json:"description" example:"Building livestream service"`
	ChannelARN      string    `json:"channel_arn" example:"arn:aws:ivs:us-east-1:123456789012:channel/abc"`
	IngestEndpoint  string    `json:"ingest_endpoint,omitempty" example:"a1b2c3d4.global-contribute.live-video.net"`
	PlaybackURL     string    `json:"playback_url" example:"https://example.playback.m3u8"`
	StreamKeyARN    string    `json:"stream_key_arn" example:"arn:aws:ivs:us-east-1:123456789012:stream-key/abc"`
	IsPaid          bool      `json:"is_paid" example:"false"`
	TicketPriceUSD  float64   `json:"ticket_price_usd" example:"0"`
	InvitedViewerID string    `json:"invited_viewer_id,omitempty" example:"user_456"`
	Status          string    `json:"status" example:"IDLE"`
	ViewerCount     int       `json:"viewer_count" example:"0"`
	RecordingS3Key  string    `json:"recording_s3_key,omitempty" example:"recordings/stream/file.mp4"`
	CreatedAt       time.Time `json:"created_at" example:"2026-05-23T10:00:00Z"`
	StartedAt       time.Time `json:"started_at,omitempty" example:"2026-05-23T10:05:00Z"`
	EndedAt         time.Time `json:"ended_at,omitempty" example:"2026-05-23T11:00:00Z"`
}

type CreateStreamRequest struct {
	StreamType      string  `json:"stream_type" example:"broadcast"`
	Title           string  `json:"title" example:"My live show"`
	Description     string  `json:"description" example:"Tonight we build together"`
	IsPaid          bool    `json:"is_paid" example:"true"`
	TicketPriceUSD  float64 `json:"ticket_price_usd" example:"5"`
	InvitedViewerID string  `json:"invited_viewer_id,omitempty" example:"user_456"`
}

type UpdateStreamRequest struct {
	Title          string  `json:"title" example:"Updated title"`
	Description    string  `json:"description" example:"Updated description"`
	IsPaid         bool    `json:"is_paid" example:"false"`
	TicketPriceUSD float64 `json:"ticket_price_usd" example:"0"`
}

type PlaybackResponse struct {
	PlaybackURL string `json:"playback_url" example:"https://example.playback.m3u8"`
}

type IVSStatusResponse struct {
	StreamID      string    `json:"stream_id" example:"1c95b9af-0f52-469b-9fa6-7d7375f26de8"`
	ChannelARN    string    `json:"channel_arn" example:"arn:aws:ivs:us-east-1:123456789012:channel/abc"`
	IsLive        bool      `json:"is_live" example:"true"`
	LastCheckedAt time.Time `json:"last_checked_at" example:"2026-05-23T10:05:00Z"`
}
