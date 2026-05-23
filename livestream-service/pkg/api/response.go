package api

// Response is the standard success envelope.
type Response struct {
	Data  interface{} `json:"data"`
	Error *string     `json:"error"`
}

// ErrorResponse is the standard error envelope.
type ErrorResponse struct {
	Data  *string `json:"data"`
	Error string  `json:"error"`
}
