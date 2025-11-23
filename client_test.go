package notifox

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	tests := []struct {
		name      string
		apiKey    string
		wantError bool
	}{
		{
			name:      "valid API key",
			apiKey:    "test-api-key",
			wantError: false,
		},
		{
			name:      "empty API key",
			apiKey:    "",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient(tt.apiKey)
			if tt.wantError {
				if err == nil {
					t.Errorf("NewClient() expected error, got nil")
				}
				if client != nil {
					t.Errorf("NewClient() expected nil client on error, got %v", client)
				}
			} else {
				if err != nil {
					t.Errorf("NewClient() unexpected error: %v", err)
				}
				if client == nil {
					t.Fatal("NewClient() returned nil client")
				}
				if client.apiKey != tt.apiKey {
					t.Errorf("NewClient() apiKey = %v, want %v", client.apiKey, tt.apiKey)
				}
				if client.baseURL != DefaultBaseURL {
					t.Errorf("NewClient() baseURL = %v, want %v", client.baseURL, DefaultBaseURL)
				}
			}
		})
	}
}

func TestNewClientFromEnv(t *testing.T) {
	t.Run("with environment variable", func(t *testing.T) {
		os.Setenv(EnvAPIKey, "env-api-key")
		defer os.Unsetenv(EnvAPIKey)

		client, err := NewClientFromEnv()
		if err != nil {
			t.Errorf("NewClientFromEnv() unexpected error: %v", err)
		}
		if client == nil {
			t.Fatal("NewClientFromEnv() returned nil client")
		}
		if client.apiKey != "env-api-key" {
			t.Errorf("NewClientFromEnv() apiKey = %v, want env-api-key", client.apiKey)
		}
	})

	t.Run("without environment variable", func(t *testing.T) {
		os.Unsetenv(EnvAPIKey)

		client, err := NewClientFromEnv()
		if err == nil {
			t.Error("NewClientFromEnv() expected error, got nil")
		}
		if client != nil {
			t.Errorf("NewClientFromEnv() expected nil client on error, got %v", client)
		}
	})
}

func TestNewClientWithOptions(t *testing.T) {
	customURL := "https://custom.example.com"
	customTimeout := 60 * time.Second
	customRetries := 5

	client, err := NewClient("test-key",
		WithBaseURL(customURL),
		WithTimeout(customTimeout),
		WithMaxRetries(customRetries),
	)
	if err != nil {
		t.Fatalf("NewClient() unexpected error: %v", err)
	}

	if client.baseURL != customURL {
		t.Errorf("NewClient() baseURL = %v, want %v", client.baseURL, customURL)
	}
	if client.timeout != customTimeout {
		t.Errorf("NewClient() timeout = %v, want %v", client.timeout, customTimeout)
	}
	if client.maxRetries != customRetries {
		t.Errorf("NewClient() maxRetries = %v, want %v", client.maxRetries, customRetries)
	}
}

func TestSendAlert(t *testing.T) {
	tests := []struct {
		name          string
		statusCode    int
		responseBody  interface{}
		wantError     bool
		wantErrorType string
		audience      string
		alert         string
	}{
		{
			name:       "successful send",
			statusCode: http.StatusOK,
			responseBody: AlertResponse{
				MessageID:  "123e4567-e89b-12d3-a456-426614174000",
				Parts:      1,
				Cost:       0.025,
				Currency:   "USD",
				Encoding:   "GSM-7",
				Characters: 24,
			},
			wantError: false,
			audience:  "test-user",
			alert:     "Test alert",
		},
		{
			name:       "authentication error",
			statusCode: http.StatusUnauthorized,
			responseBody: ErrorResponse{
				Error: "Authentication failed",
			},
			wantError:     true,
			wantErrorType: "NotifoxAuthenticationError",
			audience:      "test-user",
			alert:         "Test alert",
		},
		{
			name:       "rate limit error",
			statusCode: http.StatusTooManyRequests,
			responseBody: ErrorResponse{
				Error: "rate limit exceeded",
			},
			wantError:     true,
			wantErrorType: "NotifoxRateLimitError",
			audience:      "test-user",
			alert:         "Test alert",
		},
		{
			name:       "bad request",
			statusCode: http.StatusBadRequest,
			responseBody: ErrorResponse{
				Error: "audience cannot be empty",
			},
			wantError:     true,
			wantErrorType: "NotifoxAPIError",
			audience:      "test-user",
			alert:         "Test alert",
		},
		{
			name:       "empty audience",
			statusCode: http.StatusOK,
			wantError:  true,
			audience:   "",
			alert:      "Test alert",
		},
		{
			name:       "empty alert",
			statusCode: http.StatusOK,
			wantError:  true,
			audience:   "test-user",
			alert:      "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost {
					t.Errorf("expected POST, got %s", r.Method)
				}
				if r.URL.Path != "/alert" {
					t.Errorf("expected /alert, got %s", r.URL.Path)
				}

				authHeader := r.Header.Get("Authorization")
				if authHeader != "Bearer test-api-key" {
					t.Errorf("expected Authorization header, got %s", authHeader)
				}

				var req AlertRequest
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					t.Errorf("failed to decode request: %v", err)
				}

				w.WriteHeader(tt.statusCode)
				json.NewEncoder(w).Encode(tt.responseBody)
			}))
			defer server.Close()

			client, err := NewClient("test-api-key", WithBaseURL(server.URL))
			if err != nil {
				t.Fatalf("NewClient() unexpected error: %v", err)
			}

			ctx := context.Background()
			resp, err := client.SendAlert(ctx, tt.audience, tt.alert)

			if tt.wantError {
				if err == nil {
					t.Error("SendAlert() expected error, got nil")
				} else if tt.wantErrorType != "" {
					switch tt.wantErrorType {
					case "NotifoxAuthenticationError":
						if _, ok := err.(*NotifoxAuthenticationError); !ok {
							t.Errorf("SendAlert() expected NotifoxAuthenticationError, got %T", err)
						}
					case "NotifoxRateLimitError":
						if _, ok := err.(*NotifoxRateLimitError); !ok {
							t.Errorf("SendAlert() expected NotifoxRateLimitError, got %T", err)
						}
					case "NotifoxAPIError":
						if _, ok := err.(*NotifoxAPIError); !ok {
							t.Errorf("SendAlert() expected NotifoxAPIError, got %T", err)
						}
					}
				}
				if resp != nil {
					t.Errorf("SendAlert() expected nil response on error, got %v", resp)
				}
			} else {
				if err != nil {
					t.Errorf("SendAlert() unexpected error: %v", err)
				}
				if resp == nil {
					t.Fatal("SendAlert() returned nil response")
				}
				expectedResp := tt.responseBody.(AlertResponse)
				if resp.MessageID != expectedResp.MessageID {
					t.Errorf("SendAlert() MessageID = %v, want %v", resp.MessageID, expectedResp.MessageID)
				}
			}
		})
	}
}

func TestSendAlertWithOptions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req AlertRequest
		json.NewDecoder(r.Body).Decode(&req)

		if req.Audience != "test-user" {
			t.Errorf("expected audience test-user, got %s", req.Audience)
		}
		if req.Alert != "Test alert" {
			t.Errorf("expected alert 'Test alert', got %s", req.Alert)
		}
		if req.Channel != "sms" {
			t.Errorf("expected channel sms, got %s", req.Channel)
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(AlertResponse{
			MessageID:  "123e4567-e89b-12d3-a456-426614174000",
			Parts:      1,
			Cost:       0.025,
			Currency:   "USD",
			Encoding:   "GSM-7",
			Characters: 24,
		})
	}))
	defer server.Close()

	client, err := NewClient("test-api-key", WithBaseURL(server.URL))
	if err != nil {
		t.Fatalf("NewClient() unexpected error: %v", err)
	}

	ctx := context.Background()
	resp, err := client.SendAlertWithOptions(ctx, AlertRequest{
		Audience: "test-user",
		Alert:    "Test alert",
		Channel:  "sms",
	})

	if err != nil {
		t.Errorf("SendAlertWithOptions() unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatal("SendAlertWithOptions() returned nil response")
	}
}

func TestCalculateParts(t *testing.T) {
	tests := []struct {
		name         string
		alert        string
		statusCode   int
		responseBody PartsResponse
		wantError    bool
	}{
		{
			name:       "successful calculation",
			alert:      "Test message",
			statusCode: http.StatusOK,
			responseBody: PartsResponse{
				Parts:      1,
				Cost:       0.025,
				Currency:   "USD",
				Encoding:   "GSM-7",
				Characters: 24,
				Message:    "Notifox: Test message",
			},
			wantError: false,
		},
		{
			name:       "empty alert",
			alert:      "",
			statusCode: http.StatusOK,
			wantError:  true,
		},
		{
			name:       "API error",
			alert:      "Test message",
			statusCode: http.StatusBadRequest,
			responseBody: PartsResponse{
				Parts: 0,
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost {
					t.Errorf("expected POST, got %s", r.Method)
				}
				if r.URL.Path != "/alert/parts" {
					t.Errorf("expected /alert/parts, got %s", r.URL.Path)
				}

				// Parts endpoint should not require auth
				authHeader := r.Header.Get("Authorization")
				if authHeader != "" {
					t.Errorf("expected no Authorization header for /alert/parts, got %s", authHeader)
				}

				w.WriteHeader(tt.statusCode)
				if tt.statusCode == http.StatusOK {
					json.NewEncoder(w).Encode(tt.responseBody)
				} else {
					json.NewEncoder(w).Encode(ErrorResponse{Error: "bad request"})
				}
			}))
			defer server.Close()

			client, err := NewClient("test-api-key", WithBaseURL(server.URL))
			if err != nil {
				t.Fatalf("NewClient() unexpected error: %v", err)
			}

			ctx := context.Background()
			resp, err := client.CalculateParts(ctx, tt.alert)

			if tt.wantError {
				if err == nil {
					t.Error("CalculateParts() expected error, got nil")
				}
				if resp != nil {
					t.Errorf("CalculateParts() expected nil response on error, got %v", resp)
				}
			} else {
				if err != nil {
					t.Errorf("CalculateParts() unexpected error: %v", err)
				}
				if resp == nil {
					t.Fatal("CalculateParts() returned nil response")
				}
				if resp.Parts != tt.responseBody.Parts {
					t.Errorf("CalculateParts() Parts = %v, want %v", resp.Parts, tt.responseBody.Parts)
				}
				if resp.Message != tt.responseBody.Message {
					t.Errorf("CalculateParts() Message = %v, want %v", resp.Message, tt.responseBody.Message)
				}
			}
		})
	}
}

func TestRetryLogic(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(ErrorResponse{Error: "Internal Server Error"})
			return
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(AlertResponse{
			MessageID:  "123e4567-e89b-12d3-a456-426614174000",
			Parts:      1,
			Cost:       0.025,
			Currency:   "USD",
			Encoding:   "GSM-7",
			Characters: 24,
		})
	}))
	defer server.Close()

	client, err := NewClient("test-api-key",
		WithBaseURL(server.URL),
		WithMaxRetries(3),
	)
	if err != nil {
		t.Fatalf("NewClient() unexpected error: %v", err)
	}

	ctx := context.Background()
	resp, err := client.SendAlert(ctx, "test-user", "Test alert")

	if err != nil {
		t.Errorf("SendAlert() unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatal("SendAlert() returned nil response")
	}
	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestNoRetryOnAuthError(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "Authentication failed"})
	}))
	defer server.Close()

	client, err := NewClient("test-api-key",
		WithBaseURL(server.URL),
		WithMaxRetries(3),
	)
	if err != nil {
		t.Fatalf("NewClient() unexpected error: %v", err)
	}

	ctx := context.Background()
	_, err = client.SendAlert(ctx, "test-user", "Test alert")

	if err == nil {
		t.Error("SendAlert() expected error, got nil")
	}
	if _, ok := err.(*NotifoxAuthenticationError); !ok {
		t.Errorf("SendAlert() expected NotifoxAuthenticationError, got %T", err)
	}
	if attempts != 1 {
		t.Errorf("expected 1 attempt (no retry on auth error), got %d", attempts)
	}
}

func TestErrorTypes(t *testing.T) {
	tests := []struct {
		name          string
		statusCode    int
		errorMessage  string
		wantErrorType string
	}{
		{
			name:          "authentication error 401",
			statusCode:    http.StatusUnauthorized,
			errorMessage:  "Authentication failed",
			wantErrorType: "NotifoxAuthenticationError",
		},
		{
			name:          "authentication error 403",
			statusCode:    http.StatusForbidden,
			errorMessage:  "Forbidden",
			wantErrorType: "NotifoxAuthenticationError",
		},
		{
			name:          "rate limit error",
			statusCode:    http.StatusTooManyRequests,
			errorMessage:  "rate limit exceeded",
			wantErrorType: "NotifoxRateLimitError",
		},
		{
			name:          "API error",
			statusCode:    http.StatusBadRequest,
			errorMessage:  "bad request",
			wantErrorType: "NotifoxAPIError",
		},
		{
			name:          "server error",
			statusCode:    http.StatusInternalServerError,
			errorMessage:  "internal error",
			wantErrorType: "NotifoxAPIError",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				json.NewEncoder(w).Encode(ErrorResponse{Error: tt.errorMessage})
			}))
			defer server.Close()

			// For client errors (4xx), we don't retry, so use maxRetries=0 to avoid waiting
			// For server errors (5xx), we also use maxRetries=0 for this test to keep it simple
			maxRetries := 0
			if tt.statusCode >= 400 && tt.statusCode < 500 {
				maxRetries = 0 // Don't retry on client errors
			}

			client, err := NewClient("test-api-key",
				WithBaseURL(server.URL),
				WithMaxRetries(maxRetries),
			)
			if err != nil {
				t.Fatalf("NewClient() unexpected error: %v", err)
			}

			ctx := context.Background()
			_, err = client.SendAlert(ctx, "test-user", "Test alert")

			if err == nil {
				t.Fatal("SendAlert() expected error, got nil")
			}

			switch tt.wantErrorType {
			case "NotifoxAuthenticationError":
				if _, ok := err.(*NotifoxAuthenticationError); !ok {
					t.Errorf("expected NotifoxAuthenticationError, got %T: %v", err, err)
				}
			case "NotifoxRateLimitError":
				if _, ok := err.(*NotifoxRateLimitError); !ok {
					t.Errorf("expected NotifoxRateLimitError, got %T: %v", err, err)
				}
			case "NotifoxAPIError":
				if _, ok := err.(*NotifoxAPIError); !ok {
					t.Errorf("expected NotifoxAPIError, got %T: %v", err, err)
				}
			}
		})
	}
}

func TestConnectionError(t *testing.T) {
	// Create a client with an invalid URL to trigger connection error
	client, err := NewClient("test-api-key", WithBaseURL("http://invalid-url-that-does-not-exist:12345"))
	if err != nil {
		t.Fatalf("NewClient() unexpected error: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	_, err = client.SendAlert(ctx, "test-user", "Test alert")

	if err == nil {
		t.Error("SendAlert() expected error, got nil")
	}
	// Connection errors can be either NotifoxConnectionError or context errors
	if _, ok := err.(*NotifoxConnectionError); !ok {
		// Check if it's a context error (which is also acceptable for connection failures)
		if err != context.DeadlineExceeded && err != context.Canceled {
			t.Errorf("expected NotifoxConnectionError or context error, got %T: %v", err, err)
		}
	}
}

func TestDefaultChannel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req AlertRequest
		json.NewDecoder(r.Body).Decode(&req)

		// Channel should default to "sms" if not provided
		if req.Channel != "sms" {
			t.Errorf("expected default channel 'sms', got %s", req.Channel)
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(AlertResponse{
			MessageID:  "123e4567-e89b-12d3-a456-426614174000",
			Parts:      1,
			Cost:       0.025,
			Currency:   "USD",
			Encoding:   "GSM-7",
			Characters: 24,
		})
	}))
	defer server.Close()

	client, err := NewClient("test-api-key", WithBaseURL(server.URL))
	if err != nil {
		t.Fatalf("NewClient() unexpected error: %v", err)
	}

	ctx := context.Background()
	_, err = client.SendAlertWithOptions(ctx, AlertRequest{
		Audience: "test-user",
		Alert:    "Test alert",
		// Channel not set, should default to "sms"
	})

	if err != nil {
		t.Errorf("SendAlertWithOptions() unexpected error: %v", err)
	}
}
