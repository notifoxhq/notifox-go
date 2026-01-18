package notifox

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

const (
	// DefaultBaseURL is the default base URL for the Notifox API.
	DefaultBaseURL = "https://api.notifox.com"
	// DefaultTimeout is the default timeout for API requests.
	DefaultTimeout = 30 * time.Second
	// DefaultMaxRetries is the default number of retries for failed requests.
	DefaultMaxRetries = 3
	// EnvAPIKey is the environment variable name for the API key.
	EnvAPIKey = "NOTIFOX_API_KEY"
)

// Client is the Notifox API client.
type Client struct {
	apiKey     string
	baseURL    string
	timeout    time.Duration
	maxRetries int
	httpClient *http.Client
}

// ClientOption is a function that configures a Client.
type ClientOption func(*Client)

// WithBaseURL sets the base URL for the client.
func WithBaseURL(url string) ClientOption {
	return func(c *Client) {
		c.baseURL = url
	}
}

// WithTimeout sets the timeout for API requests.
func WithTimeout(timeout time.Duration) ClientOption {
	return func(c *Client) {
		c.timeout = timeout
		c.httpClient.Timeout = timeout
	}
}

// WithMaxRetries sets the maximum number of retries for failed requests.
func WithMaxRetries(maxRetries int) ClientOption {
	return func(c *Client) {
		c.maxRetries = maxRetries
	}
}

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(httpClient *http.Client) ClientOption {
	return func(c *Client) {
		c.httpClient = httpClient
	}
}

// NewClient creates a new Notifox client with the provided API key.
// If apiKey is empty, it will attempt to read from the NOTIFOX_API_KEY environment variable.
func NewClient(apiKey string, opts ...ClientOption) (*Client, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("api key is required (provide it directly or set %s environment variable)", EnvAPIKey)
	}

	client := &Client{
		apiKey:     apiKey,
		baseURL:    DefaultBaseURL,
		timeout:    DefaultTimeout,
		maxRetries: DefaultMaxRetries,
		httpClient: &http.Client{
			Timeout: DefaultTimeout,
		},
	}

	for _, opt := range opts {
		opt(client)
	}

	return client, nil
}

// NewClientFromEnv creates a new Notifox client using the API key from the NOTIFOX_API_KEY environment variable.
func NewClientFromEnv(opts ...ClientOption) (*Client, error) {
	apiKey := os.Getenv(EnvAPIKey)
	if apiKey == "" {
		return nil, fmt.Errorf("api key is required (set %s environment variable)", EnvAPIKey)
	}

	return NewClient(apiKey, opts...)
}

// SendAlert sends an alert to a verified audience.
func (c *Client) SendAlert(ctx context.Context, audience, alert string) (*AlertResponse, error) {
	return c.SendAlertWithOptions(ctx, AlertRequest{
		Audience: audience,
		Alert:    alert,
	})
}

// SendAlertWithOptions sends an alert with additional options.
// The channel can be set to "sms" or "email".
func (c *Client) SendAlertWithOptions(ctx context.Context, req AlertRequest) (*AlertResponse, error) {
	if req.Audience == "" {
		return nil, fmt.Errorf("audience cannot be empty")
	}
	if req.Alert == "" {
		return nil, fmt.Errorf("alert message cannot be empty")
	}

	// Validate channel is either empty, "sms", or "email"
	if req.Channel != "" && req.Channel != "sms" && req.Channel != "email" {
		return nil, fmt.Errorf("channel must be either 'sms' or 'email'")
	}

	url := fmt.Sprintf("%s/alert", c.baseURL)

	var err error
	var result interface{}

	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		result, err = c.doRequest(ctx, http.MethodPost, url, req, &AlertResponse{})
		if err == nil {
			return result.(*AlertResponse), nil
		}

		// Don't retry on authentication errors or rate limit errors
		if _, isAuthErr := err.(*NotifoxAuthenticationError); isAuthErr {
			return nil, err
		}
		if _, isRateLimitErr := err.(*NotifoxRateLimitError); isRateLimitErr {
			return nil, err
		}
		// Don't retry on other 4xx client errors (bad requests, etc.)
		if apiErr, isAPIErr := err.(*NotifoxAPIError); isAPIErr && apiErr.StatusCode >= 400 && apiErr.StatusCode < 500 {
			return nil, err
		}

		// Don't retry on the last attempt
		if attempt < c.maxRetries {
			// Simple exponential backoff
			backoff := time.Duration(attempt+1) * 100 * time.Millisecond
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
				// Continue to next attempt
			}
		}
	}

	return nil, err
}

// CalculateParts calculates the number of SMS parts, cost, encoding, and character count
// for a message without actually sending it.
func (c *Client) CalculateParts(ctx context.Context, alert string) (*PartsResponse, error) {
	if alert == "" {
		return nil, fmt.Errorf("alert message cannot be empty")
	}

	req := PartsRequest{Alert: alert}
	url := fmt.Sprintf("%s/alert/parts", c.baseURL)

	var resp PartsResponse
	_, err := c.doRequest(ctx, http.MethodPost, url, req, &resp)
	if err != nil {
		return nil, err
	}

	return &resp, nil
}

// doRequest performs an HTTP request and handles the response.
func (c *Client) doRequest(ctx context.Context, method, url string, body interface{}, result interface{}) (interface{}, error) {
	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, &NotifoxConnectionError{
				NotifoxError: NotifoxError{Message: "failed to marshal request"},
				Err:          err,
			}
		}
		reqBody = bytes.NewBuffer(jsonData)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, &NotifoxConnectionError{
			NotifoxError: NotifoxError{Message: "failed to create request"},
			Err:          err,
		}
	}

	req.Header.Set("Content-Type", "application/json")

	// Only add Authorization header for endpoints that require it
	if url != fmt.Sprintf("%s/alert/parts", c.baseURL) {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, &NotifoxConnectionError{
			NotifoxError: NotifoxError{Message: "request failed"},
			Err:          err,
		}
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &NotifoxConnectionError{
			NotifoxError: NotifoxError{Message: "failed to read response"},
			Err:          err,
		}
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		if result != nil {
			if err := json.Unmarshal(respBody, result); err != nil {
				return nil, &NotifoxAPIError{
					NotifoxError: NotifoxError{Message: "failed to unmarshal response"},
					StatusCode:   resp.StatusCode,
					ResponseText: string(respBody),
				}
			}
		}
		return result, nil
	}

	// Try to parse error response
	var errorResp ErrorResponse
	if err := json.Unmarshal(respBody, &errorResp); err == nil && errorResp.Error != "" {
		return nil, parseError(resp.StatusCode, errorResp.Error)
	}

	return nil, parseError(resp.StatusCode, string(respBody))
}
