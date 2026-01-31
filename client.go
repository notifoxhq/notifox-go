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
	// Version is the SDK version, used in the User-Agent header.
	Version = "v0.1.6"
	// DefaultUserAgent is the default User-Agent string for API requests.
	DefaultUserAgent = "notifox-go/" + Version
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
	UserAgent  string
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

// WithAPIKey sets the API key for the client. Optional when using NewClientWithOptions;
// if not set, the key is read from the NOTIFOX_API_KEY environment variable.
func WithAPIKey(apiKey string) ClientOption {
	return func(c *Client) {
		c.apiKey = apiKey
	}
}

// WithUserAgent sets the User-Agent header for requests.
func WithUserAgent(userAgent string) ClientOption {
	return func(c *Client) {
		if userAgent == "" {
			userAgent = DefaultUserAgent
		}

		c.UserAgent = userAgent
	}
}

// NewClient creates a new Notifox client using the API key from the NOTIFOX_API_KEY
// environment variable. For configuration (base URL, timeout, etc.) use
// NewClientWithOptions and the option functions (e.g. WithBaseURL, WithAPIKey).
func NewClient() (*Client, error) {
	apiKey := os.Getenv(EnvAPIKey)
	if apiKey == "" {
		return nil, fmt.Errorf("api key is required (set %s environment variable or use NewClientWithOptions with WithAPIKey)", EnvAPIKey)
	}

	return &Client{
		apiKey:     apiKey,
		baseURL:    DefaultBaseURL,
		timeout:    DefaultTimeout,
		maxRetries: DefaultMaxRetries,
		httpClient: &http.Client{
			Timeout: DefaultTimeout,
		},
		UserAgent: DefaultUserAgent,
	}, nil
}

// NewClientWithOptions creates a new Notifox client from options. The API key is optional:
// pass WithAPIKey(key) to set it, or omit it to use the NOTIFOX_API_KEY environment variable.
func NewClientWithOptions(opts ...ClientOption) (*Client, error) {
	client := &Client{
		baseURL:    DefaultBaseURL,
		timeout:    DefaultTimeout,
		maxRetries: DefaultMaxRetries,
		httpClient: &http.Client{
			Timeout: DefaultTimeout,
		},
		UserAgent: DefaultUserAgent,
	}

	for _, opt := range opts {
		opt(client)
	}

	if client.apiKey == "" {
		client.apiKey = os.Getenv(EnvAPIKey)
	}
	if client.apiKey == "" {
		return nil, fmt.Errorf("api key is required (set %s environment variable or use notifox.WithAPIKey)", EnvAPIKey)
	}

	return client, nil
}

// SendAlert sends an alert to a verified audience.
// Always use AlertRequest to specify audience, channel (SMS or Email), and alert message.
func (c *Client) SendAlert(ctx context.Context, req AlertRequest) (*AlertResponse, error) {
	if req.Audience == "" {
		return nil, fmt.Errorf("audience cannot be empty")
	}
	if req.Alert == "" {
		return nil, fmt.Errorf("alert message cannot be empty")
	}

	// Validate channel is either empty, SMS, or Email
	if req.Channel != "" && req.Channel != SMS && req.Channel != Email {
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
	req.Header.Set("User-Agent", c.UserAgent)

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

	// Handle 401 Unauthorized - returns plain text, not JSON
	if resp.StatusCode == http.StatusUnauthorized {
		return nil, parseError(resp.StatusCode, string(respBody))
	}

	// Try to parse error response as JSON
	var errorResp ErrorResponse
	if err := json.Unmarshal(respBody, &errorResp); err == nil && errorResp.Error != "" {
		return nil, parseError(resp.StatusCode, errorResp.Error)
	}

	// Fallback to raw response body if JSON parsing fails
	return nil, parseError(resp.StatusCode, string(respBody))
}
