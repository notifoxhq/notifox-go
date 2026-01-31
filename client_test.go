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
	t.Run("with environment variable", func(t *testing.T) {
		os.Setenv(EnvAPIKey, "env-api-key")
		defer os.Unsetenv(EnvAPIKey)

		client, err := NewClient()
		if err != nil {
			t.Fatalf("NewClient() unexpected error: %v", err)
		}
		if client == nil {
			t.Fatal("NewClient() returned nil client")
		}
		if client.apiKey != "env-api-key" {
			t.Errorf("apiKey = %q, want %q", client.apiKey, "env-api-key")
		}
		if client.baseURL != DefaultBaseURL {
			t.Errorf("baseURL = %v, want %v", client.baseURL, DefaultBaseURL)
		}
	})

	t.Run("without environment variable", func(t *testing.T) {
		os.Unsetenv(EnvAPIKey)

		client, err := NewClient()
		if err == nil {
			t.Error("NewClient() expected error, got nil")
		}
		if client != nil {
			t.Errorf("NewClient() expected nil client on error, got %v", client)
		}
	})
}


func TestNewClientAppliesOptions(t *testing.T) {
	customURL := "https://custom.example.com"
	customTimeout := 60 * time.Second
	customRetries := 5

	client, err := NewClientWithOptions(
		WithAPIKey("test-key"),
		WithBaseURL(customURL),
		WithTimeout(customTimeout),
		WithMaxRetries(customRetries),
	)
	if err != nil {
		t.Fatalf("NewClientWithOptions() unexpected error: %v", err)
	}

	if client.baseURL != customURL {
		t.Errorf("baseURL = %v, want %v", client.baseURL, customURL)
	}
	if client.timeout != customTimeout {
		t.Errorf("timeout = %v, want %v", client.timeout, customTimeout)
	}
	if client.maxRetries != customRetries {
		t.Errorf("maxRetries = %v, want %v", client.maxRetries, customRetries)
	}
}

func TestNewClientWithOptions(t *testing.T) {
	t.Run("key from option", func(t *testing.T) {
		client, err := NewClientWithOptions(WithAPIKey("option-key"))
		if err != nil {
			t.Fatalf("NewClientWithOptions() unexpected error: %v", err)
		}
		if client.apiKey != "option-key" {
			t.Errorf("apiKey = %q, want %q", client.apiKey, "option-key")
		}
	})

	t.Run("key from env when option omitted", func(t *testing.T) {
		os.Setenv(EnvAPIKey, "env-key")
		defer os.Unsetenv(EnvAPIKey)

		client, err := NewClientWithOptions()
		if err != nil {
			t.Fatalf("NewClientWithOptions() unexpected error: %v", err)
		}
		if client.apiKey != "env-key" {
			t.Errorf("apiKey = %q, want %q", client.apiKey, "env-key")
		}
	})

	t.Run("error when no key", func(t *testing.T) {
		os.Unsetenv(EnvAPIKey)

		_, err := NewClientWithOptions()
		if err == nil {
			t.Error("NewClientWithOptions() expected error, got nil")
		}
	})

	t.Run("key from option with other options", func(t *testing.T) {
		customURL := "https://custom.example.com"
		client, err := NewClientWithOptions(WithAPIKey("my-key"), WithBaseURL(customURL))
		if err != nil {
			t.Fatalf("NewClientWithOptions() unexpected error: %v", err)
		}
		if client.apiKey != "my-key" {
			t.Errorf("apiKey = %q, want %q", client.apiKey, "my-key")
		}
		if client.baseURL != customURL {
			t.Errorf("baseURL = %v, want %v", client.baseURL, customURL)
		}
	})
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
				// 401 returns plain text "Unauthorized", not JSON
				if tt.statusCode == http.StatusUnauthorized {
					w.Write([]byte("Unauthorized"))
				} else {
					json.NewEncoder(w).Encode(tt.responseBody)
				}
			}))
			defer server.Close()

			client, err := NewClientWithOptions(WithAPIKey("test-api-key"), WithBaseURL(server.URL))
			if err != nil {
				t.Fatalf("NewClient() unexpected error: %v", err)
			}

			ctx := context.Background()
			resp, err := client.SendAlert(ctx, AlertRequest{Audience: tt.audience, Alert: tt.alert})

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

func TestSendAlertWithChannel(t *testing.T) {
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

	client, err := NewClientWithOptions(WithAPIKey("test-api-key"), WithBaseURL(server.URL))
	if err != nil {
		t.Fatalf("NewClient() unexpected error: %v", err)
	}

	ctx := context.Background()
	resp, err := client.SendAlert(ctx, AlertRequest{
		Audience: "test-user",
		Alert:    "Test alert",
		Channel:  SMS,
	})

	if err != nil {
		t.Errorf("SendAlert() unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatal("SendAlert() returned nil response")
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

			client, err := NewClientWithOptions(WithAPIKey("test-api-key"), WithBaseURL(server.URL))
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

	client, err := NewClientWithOptions(
		WithAPIKey("test-api-key"),
		WithBaseURL(server.URL),
		WithMaxRetries(3),
	)
	if err != nil {
		t.Fatalf("NewClient() unexpected error: %v", err)
	}

	ctx := context.Background()
	resp, err := client.SendAlert(ctx, AlertRequest{Audience: "test-user", Alert: "Test alert"})

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
		// 401 returns plain text, not JSON
		w.Write([]byte("Unauthorized"))
	}))
	defer server.Close()

	client, err := NewClientWithOptions(
		WithAPIKey("test-api-key"),
		WithBaseURL(server.URL),
		WithMaxRetries(3),
	)
	if err != nil {
		t.Fatalf("NewClient() unexpected error: %v", err)
	}

	ctx := context.Background()
	_, err = client.SendAlert(ctx, AlertRequest{Audience: "test-user", Alert: "Test alert"})

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
			errorMessage:  "Unauthorized",
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
				// 401 returns plain text, not JSON
				if tt.statusCode == http.StatusUnauthorized {
					w.Write([]byte(tt.errorMessage))
				} else {
					json.NewEncoder(w).Encode(ErrorResponse{Error: tt.errorMessage})
				}
			}))
			defer server.Close()

			// For client errors (4xx), we don't retry, so use maxRetries=0 to avoid waiting
			// For server errors (5xx), we also use maxRetries=0 for this test to keep it simple
			maxRetries := 0
			if tt.statusCode >= 400 && tt.statusCode < 500 {
				maxRetries = 0 // Don't retry on client errors
			}

			client, err := NewClientWithOptions(
				WithAPIKey("test-api-key"),
				WithBaseURL(server.URL),
				WithMaxRetries(maxRetries),
			)
			if err != nil {
				t.Fatalf("NewClient() unexpected error: %v", err)
			}

			ctx := context.Background()
			_, err = client.SendAlert(ctx, AlertRequest{Audience: "test-user", Alert: "Test alert"})

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
	client, err := NewClientWithOptions(WithAPIKey("test-api-key"), WithBaseURL("http://invalid-url-that-does-not-exist:12345"))
	if err != nil {
		t.Fatalf("NewClient() unexpected error: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	_, err = client.SendAlert(ctx, AlertRequest{Audience: "test-user", Alert: "Test alert"})

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

		// Channel should be empty if not provided
		if req.Channel != "" {
			t.Errorf("expected empty channel when not specified, got %s", req.Channel)
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

	client, err := NewClientWithOptions(WithAPIKey("test-api-key"), WithBaseURL(server.URL))
	if err != nil {
		t.Fatalf("NewClient() unexpected error: %v", err)
	}

	ctx := context.Background()
	_, err = client.SendAlert(ctx, AlertRequest{
		Audience: "test-user",
		Alert:    "Test alert",
		// Channel not set, should be empty
	})

	if err != nil {
		t.Errorf("SendAlert() unexpected error: %v", err)
	}
}
