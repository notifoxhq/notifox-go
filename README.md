# Notifox Go Client

Go SDK for Notifox.

## Installation

```bash
go get github.com/notifoxhq/notifox-go
```

## Usage

### Basic Usage

The recommended way is to use the environment variable:

```go
package main

import (
    "context"
    "fmt"
    "github.com/notifoxhq/notifox-go"
)

func main() {
    // Reads from NOTIFOX_API_KEY environment variable
    client, err := notifox.NewClientFromEnv()
    if err != nil {
        panic(err)
    }

    ctx := context.Background()
    resp, err := client.SendAlert(ctx, "mike", "Database server is down!")
    if err != nil {
        panic(err)
    }

    fmt.Printf("Alert sent! Message ID: %s\n", resp.MessageID)
}
```

**Note:** You can also use `notifox.NewClient("")` which is equivalent to `notifox.NewClientFromEnv()`.

### Providing API Key Directly

Alternatively, you can provide the API key directly:

```go
package main

import (
    "context"
    "fmt"
    "github.com/notifoxhq/notifox-go"
)

func main() {
    client, err := notifox.NewClient("your_api_key_here")
    if err != nil {
        panic(err)
    }

    ctx := context.Background()
    resp, err := client.SendAlert(ctx, "mike", "High CPU usage!")
    if err != nil {
        panic(err)
    }

    fmt.Printf("Alert sent! Cost: $%.3f\n", resp.Cost)
}
```

### Configuration

```go
client, err := notifox.NewClient(
    "your_api_key",
    notifox.WithBaseURL("https://api.notifox.com"),
    notifox.WithTimeout(30*time.Second),
    notifox.WithMaxRetries(3),
)
```

### Error Handling

```go
package main

import (
    "context"
    "fmt"
    "github.com/notifoxhq/notifox-go"
)

func main() {
    client, err := notifox.NewClient("your_api_key")
    if err != nil {
        panic(err)
    }

    ctx := context.Background()
    resp, err := client.SendAlert(ctx, "admin", "System is running low on memory")
    if err != nil {
        switch e := err.(type) {
        case *notifox.NotifoxAuthenticationError:
            fmt.Printf("Authentication failed. Check your API key. Status: %d\n", e.StatusCode)
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

    fmt.Printf("Alert sent successfully! Message ID: %s\n", resp.MessageID)
}
```

### Available Error Types

- `NotifoxError` - Base error type
- `NotifoxAuthenticationError` - Authentication failed (401/403)
- `NotifoxRateLimitError` - Rate limit exceeded (429)
- `NotifoxAPIError` - General API errors
- `NotifoxConnectionError` - Network errors

### Additional Methods

#### Calculate Parts

Calculate message parts without sending:

```go
resp, err := client.CalculateParts(ctx, "Your message here")
if err != nil {
    panic(err)
}

fmt.Printf("Parts: %d, Cost: $%.3f, Encoding: %s\n", 
    resp.Parts, resp.Cost, resp.Encoding)
```

#### Send Alert with Options

Send alert with additional options:

```go
resp, err := client.SendAlertWithOptions(ctx, notifox.AlertRequest{
    Audience: "admin",
    Alert:    "Critical system failure!",
    Channel:  "sms",
})
```
