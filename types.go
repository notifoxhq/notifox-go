package notifox

// AlertRequest represents a request to send an alert.
type AlertRequest struct {
	Audience string `json:"audience"`
	Alert    string `json:"alert"`
	Channel  string `json:"channel,omitempty"`
}

// AlertResponse represents the response from sending an alert.
type AlertResponse struct {
	MessageID  string  `json:"message_id"`
	Parts      int     `json:"parts"`
	Cost       float64 `json:"cost"`
	Currency   string  `json:"currency"`
	Encoding   string  `json:"encoding"`
	Characters int     `json:"characters"`
}

// PartsRequest represents a request to calculate message parts.
type PartsRequest struct {
	Alert string `json:"alert"`
}

// PartsResponse represents the response from calculating message parts.
type PartsResponse struct {
	Parts      int     `json:"parts"`
	Cost       float64 `json:"cost"`
	Currency   string  `json:"currency"`
	Encoding   string  `json:"encoding"`
	Characters int     `json:"characters"`
	Message    string  `json:"message"`
}

// ErrorResponse represents an error response from the API.
type ErrorResponse struct {
	Error string `json:"error"`
}
