package payment

type TipRequest struct {
	AmountUSD float64 `json:"amount_usd" example:"5"`
	Message   string  `json:"message" example:"great stream"`
}
