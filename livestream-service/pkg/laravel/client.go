package laravel

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

/*
POST /internal/auth/verify
  Headers: X-Internal-Secret
  Body:    { "token": "..." }
  Returns: { "user_id", "role", "username" }

POST /internal/payments/charge
  Headers: X-Internal-Secret
  Body:    { "type": "tip|ticket|session", "payer_user_id", "payee_user_id",
             "amount_usd", "metadata": {} }
  Returns: { "transaction_id", "status": "success|pending|failed" }

POST /internal/notifications/send
  Headers: X-Internal-Secret
  Body:    { "user_id", "event_type", "payload": {} }
  Returns: { "sent": true }

POST /internal/streams/access-check
  Headers: X-Internal-Secret
  Body:    { "user_id", "creator_id" }
  Returns: { "can_access": bool, "reason": "following|subscribed|not_following|not_subscribed" }

POST /internal/streams/sync
  Headers: X-Internal-Secret
  Body:    stream fields
  Returns: { "synced": bool, "laravel_stream_id": string }
*/

type UserInfo struct {
	UserID   string `json:"user_id"`
	Role     string `json:"role"`
	Username string `json:"username"`
}

type ChargeRequest struct {
	Type        string            `json:"type"`
	PayerUserID string            `json:"payer_user_id"`
	PayeeUserID string            `json:"payee_user_id"`
	AmountUSD   float64           `json:"amount_usd"`
	Metadata    map[string]string `json:"metadata"`
}

type ChargeResponse struct {
	TransactionID string `json:"transaction_id"`
	Status        string `json:"status"`
}

// StreamAccessRequest asks Laravel whether a user (already authenticated) may
// access an unlocked stream by verifying their follow/subscription relationship
// with the creator.
type StreamAccessRequest struct {
	UserID    string `json:"user_id"`
	CreatorID string `json:"creator_id"`
	StreamID  string `json:"stream_id,omitempty"`
	IsPaid    bool   `json:"is_paid,omitempty"`
}

// StreamAccessResponse is returned by Laravel's /internal/streams/access-check.
// Reason values: "following", "subscribed", "not_following", "not_subscribed".
type StreamAccessResponse struct {
	CanAccess bool   `json:"can_access"`
	Reason    string `json:"reason"`
}

// SyncStreamRequest carries stream metadata to be persisted in Laravel.
type SyncStreamRequest struct {
	StreamID       string    `json:"stream_id"`
	CreatorID      string    `json:"creator_id"`
	StreamType     string    `json:"stream_type"`
	Title          string    `json:"title"`
	Description    string    `json:"description"`
	IsPaid         bool      `json:"is_paid"`
	TicketPriceUSD float64   `json:"ticket_price_usd"`
	ChannelARN     string    `json:"channel_arn,omitempty"`
	PlaybackURL    string    `json:"playback_url,omitempty"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
}

// SyncStreamResponse is returned by Laravel's /internal/streams/sync.
type SyncStreamResponse struct {
	Synced          bool   `json:"synced"`
	LaravelStreamID string `json:"laravel_stream_id"`
}

type Client interface {
	VerifyToken(ctx context.Context, token string) (*UserInfo, error)
	ChargePayment(ctx context.Context, req ChargeRequest) (*ChargeResponse, error)
	NotifyUser(ctx context.Context, userID, eventType string, payload map[string]string) error
	// CheckStreamAccess asks Laravel if user follows/subscribes to the creator
	// (used for unlocked streams where payment is not required).
	CheckStreamAccess(ctx context.Context, req StreamAccessRequest) (*StreamAccessResponse, error)
	// SyncStream pushes stream metadata to the Laravel backend for persistence.
	SyncStream(ctx context.Context, req SyncStreamRequest) (*SyncStreamResponse, error)
}

type HTTPClient struct {
	baseURL        string
	internalSecret string
	httpClient     *http.Client
}

func New(baseURL, internalSecret string) *HTTPClient {
	return &HTTPClient{
		baseURL:        baseURL,
		internalSecret: internalSecret,
		httpClient:     &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *HTTPClient) VerifyToken(ctx context.Context, token string) (*UserInfo, error) {
	var out UserInfo
	if err := c.post(ctx, "/internal/auth/verify", map[string]string{"token": token}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *HTTPClient) ChargePayment(ctx context.Context, req ChargeRequest) (*ChargeResponse, error) {
	var out ChargeResponse
	if err := c.post(ctx, "/internal/payments/charge", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *HTTPClient) NotifyUser(ctx context.Context, userID, eventType string, payload map[string]string) error {
	body := map[string]interface{}{
		"user_id":    userID,
		"event_type": eventType,
		"payload":    payload,
	}
	return c.post(ctx, "/internal/notifications/send", body, nil)
}

func (c *HTTPClient) CheckStreamAccess(ctx context.Context, req StreamAccessRequest) (*StreamAccessResponse, error) {
	var out StreamAccessResponse
	if err := c.post(ctx, "/internal/streams/access-check", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *HTTPClient) SyncStream(ctx context.Context, req SyncStreamRequest) (*SyncStreamResponse, error) {
	var out SyncStreamResponse
	if err := c.post(ctx, "/internal/streams/sync", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *HTTPClient) post(ctx context.Context, path string, body interface{}, out interface{}) error {
	b, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(b))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Secret", c.internalSecret)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("laravel API %s returned status %d", path, resp.StatusCode)
	}

	if out == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}
