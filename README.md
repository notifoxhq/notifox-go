# Notifox Go Client

[![Go Reference](https://pkg.go.dev/badge/github.com/notifoxhq/notifox-go.svg)](https://pkg.go.dev/github.com/notifoxhq/notifox-go)
[![CI](https://github.com/notifoxhq/notifox-go/actions/workflows/ci.yaml/badge.svg)](https://github.com/notifoxhq/notifox-go/actions/workflows/ci.yaml)

Go SDK for the Notifox API. Create a client with `NewClient()` (key from env, no options) or `NewClientWithOptions` (key via `WithAPIKey` or env, with options). Send alerts with `SendAlert` and an `AlertRequest`. The API key is never a direct argument—use the `WithAPIKey` option or the `NOTIFOX_API_KEY` environment variable.

## Installation

```bash
go get github.com/notifoxhq/notifox-go
```

## Usage

### Basic usage

Create a client from the `NOTIFOX_API_KEY` environment variable with `NewClient()`, then call `SendAlert` with an `AlertRequest`:

```go
// go get github.com/notifoxhq/notifox-go
import "github.com/notifoxhq/notifox-go"

client, _ := notifox.NewClient()

client.SendAlert(ctx, notifox.AlertRequest{
    Audience: "oncall-team",
    Channel:  notifox.SMS,
    Alert:    "🚨 Production DB down!",
})
```

Full example with error handling:

```go
package main

import (
    "context"
    "fmt"
    "log"
    "github.com/notifoxhq/notifox-go"
)

func main() {
    client, err := notifox.NewClient()
    if err != nil {
        log.Fatal(err)
    }

    ctx := context.Background()
    resp, err := client.SendAlert(ctx, notifox.AlertRequest{
        Audience: "oncall-team",
        Channel:  notifox.SMS,
        Alert:    "🚨 Production DB down!",
    })
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Alert sent! Message ID: %s\n", resp.MessageID)
}
```

**AlertRequest fields:**

- **Audience** – Verified audience identifier (e.g. team or user slug).
- **Channel** – `notifox.SMS`, `notifox.Email`, or leave empty.
- **Alert** – The alert message body.

### Creating a client

**`NewClient()`**  
No arguments. Creates a client using the API key from the `NOTIFOX_API_KEY` environment variable. No configuration options—use `NewClientWithOptions` if you need to set base URL, timeout, etc.

```go
client, err := notifox.NewClient()
if err != nil {
    log.Fatal(err)
}
```

**`NewClientWithOptions(opts ...ClientOption)`**  
Creates a client from options. API key: pass `WithAPIKey(key)` or omit to use `NOTIFOX_API_KEY`. Use this when you need to configure the client (base URL, timeout, retries, etc.).

```go
// Key from env
client, err := notifox.NewClientWithOptions()

// Key from option
client, err := notifox.NewClientWithOptions(notifox.WithAPIKey("your_api_key_here"))

// Key + options
client, err := notifox.NewClientWithOptions(
    notifox.WithAPIKey("your_api_key_here"),
    notifox.WithBaseURL("https://api.notifox.com"),
)
if err != nil {
    log.Fatal(err)
}
```

### Configuration options

`NewClientWithOptions` accepts optional `ClientOption` functions. `NewClient()` takes no options.

| Option | Description |
|--------|-------------|
| `WithAPIKey(string)` | Set the API key (omit to use `NOTIFOX_API_KEY`). |
| `WithBaseURL(string)` | Set the API base URL (default: `https://api.notifox.com`). |
| `WithTimeout(time.Duration)` | Set the HTTP client timeout (default: 30s). |
| `WithMaxRetries(int)` | Set the number of retries for failed requests (default: 3). |
| `WithHTTPClient(*http.Client)` | Use a custom HTTP client. |
| `WithUserAgent(string)` | Set the User-Agent header (empty string uses default). |

Example:

```go
client, err := notifox.NewClientWithOptions(
    notifox.WithAPIKey("your_api_key"),
    notifox.WithBaseURL("https://api.notifox.com"),
    notifox.WithTimeout(30*time.Second),
    notifox.WithMaxRetries(3),
)
```

### Sending alerts

**`SendAlert(ctx context.Context, req AlertRequest) (*AlertResponse, error)`**  
Sends an alert. Always pass an `AlertRequest` (Audience, Channel, Alert).

```go
resp, err := client.SendAlert(ctx, notifox.AlertRequest{
    Audience: "oncall-team",
    Channel:  notifox.SMS,
    Alert:    "🚨 Production DB down!",
})
// resp.MessageID, resp.Parts, resp.Cost, resp.Currency, resp.Encoding, resp.Characters
```

### Calculate parts

**`CalculateParts(ctx context.Context, alert string) (*PartsResponse, error)`**  
Returns SMS parts, cost, encoding, and character count without sending.

```go
resp, err := client.CalculateParts(ctx, "Your message here")
if err != nil {
    log.Fatal(err)
}
fmt.Printf("Parts: %d, Cost: $%.3f, Encoding: %s\n", resp.Parts, resp.Cost, resp.Encoding)
```

### Error handling

Use type assertions or `errors.As` to handle specific error types:

```go
resp, err := client.SendAlert(ctx, notifox.AlertRequest{
    Audience: "admin",
    Alert:    "System is running low on memory",
})
if err != nil {
    switch e := err.(type) {
    case *notifox.NotifoxAuthenticationError:
        fmt.Printf("Authentication failed. Check your API key. Status: %d\n", e.StatusCode)
    case *notifox.NotifoxInsufficientBalanceError:
        fmt.Printf("Insufficient balance: %s\n", e.ResponseText)
    case *notifox.NotifoxRateLimitError:
        fmt.Println("Rate limit exceeded. Please wait before sending more alerts.")
    case *notifox.NotifoxAPIError:
        fmt.Printf("API error (%d): %s\n", e.StatusCode, e.ResponseText)
    case *notifox.NotifoxConnectionError:
        fmt.Printf("Connection failed: %v\n", e.Err)
    default:
        fmt.Printf("Error: %v\n", err)
    }
    return
}
fmt.Printf("Alert sent! Message ID: %s\n", resp.MessageID)
```

**Error types:**

- `NotifoxError` – Base error type
- `NotifoxAuthenticationError` – Authentication failed (401/403)
- `NotifoxInsufficientBalanceError` – Insufficient balance (402)
- `NotifoxRateLimitError` – Rate limit exceeded (429)
- `NotifoxAPIError` – General API errors (4xx/5xx)
- `NotifoxConnectionError` – Network/connection errors

### Constants

- **`notifox.EnvAPIKey`** – Environment variable name for the API key: `NOTIFOX_API_KEY`
- **`notifox.SMS`**, **`notifox.Email`** – Channel values for `AlertRequest.Channel`
