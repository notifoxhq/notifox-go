package notifox

import (
	"fmt"
	"net/http"
)

// NotifoxError is the base error type for all Notifox errors.
type NotifoxError struct {
	Message string
}

func (e *NotifoxError) Error() string {
	return e.Message
}

// NotifoxAuthenticationError represents authentication failures (401/403).
type NotifoxAuthenticationError struct {
	NotifoxError
	StatusCode   int
	ResponseText string
}

func (e *NotifoxAuthenticationError) Error() string {
	if e.ResponseText != "" {
		return fmt.Sprintf("authentication failed (%d): %s", e.StatusCode, e.ResponseText)
	}
	return fmt.Sprintf("authentication failed (%d)", e.StatusCode)
}

// NotifoxRateLimitError represents rate limit exceeded errors (429).
type NotifoxRateLimitError struct {
	NotifoxError
	ResponseText string
}

func (e *NotifoxRateLimitError) Error() string {
	if e.ResponseText != "" {
		return fmt.Sprintf("rate limit exceeded: %s", e.ResponseText)
	}
	return "rate limit exceeded"
}

// NotifoxInsufficientBalanceError represents insufficient balance errors (402).
type NotifoxInsufficientBalanceError struct {
	NotifoxError
	ResponseText string
}

func (e *NotifoxInsufficientBalanceError) Error() string {
	if e.ResponseText != "" {
		return fmt.Sprintf("insufficient balance: %s", e.ResponseText)
	}
	return "insufficient balance"
}

// NotifoxAPIError represents general API errors.
type NotifoxAPIError struct {
	NotifoxError
	StatusCode   int
	ResponseText string
}

func (e *NotifoxAPIError) Error() string {
	if e.ResponseText != "" {
		return fmt.Sprintf("API error (%d): %s", e.StatusCode, e.ResponseText)
	}
	return fmt.Sprintf("API error (%d)", e.StatusCode)
}

// NotifoxConnectionError represents network/connection errors.
type NotifoxConnectionError struct {
	NotifoxError
	Err error
}

func (e *NotifoxConnectionError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("connection failed: %v", e.Err)
	}
	return "connection failed"
}

func (e *NotifoxConnectionError) Unwrap() error {
	return e.Err
}

// parseError creates the appropriate error type based on the HTTP status code.
func parseError(statusCode int, responseText string) error {
	switch statusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		return &NotifoxAuthenticationError{
			NotifoxError: NotifoxError{Message: "authentication failed"},
			StatusCode:   statusCode,
			ResponseText: responseText,
		}
	case http.StatusPaymentRequired: // 402
		return &NotifoxInsufficientBalanceError{
			NotifoxError: NotifoxError{Message: "insufficient balance"},
			ResponseText: responseText,
		}
	case http.StatusTooManyRequests: // 429
		return &NotifoxRateLimitError{
			NotifoxError: NotifoxError{Message: "rate limit exceeded"},
			ResponseText: responseText,
		}
	default:
		return &NotifoxAPIError{
			NotifoxError: NotifoxError{Message: "API error"},
			StatusCode:   statusCode,
			ResponseText: responseText,
		}
	}
}
