package session

type CreateSessionRequest struct {
	Title           string  `json:"title" example:"1:1 coaching"`
	Description     string  `json:"description" example:"Private mentoring"`
	InvitedViewerID string  `json:"invited_viewer_id" example:"user_456"`
	PriceUSD        float64 `json:"price_usd" example:"20"`
}
