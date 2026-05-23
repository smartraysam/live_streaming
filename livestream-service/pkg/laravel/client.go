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

type Client interface {
	VerifyToken(ctx context.Context, token string) (*UserInfo, error)
	ChargePayment(ctx context.Context, req ChargeRequest) (*ChargeResponse, error)
	NotifyUser(ctx context.Context, userID, eventType string, payload map[string]string) error
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
