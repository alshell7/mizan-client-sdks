# Mizan Go SDK

Go 1.22+ server-side client using only the standard library. It preserves exact amounts as
`mizan.ExactAmount`, supports contexts, bounds response bodies, and returns structured `APIError`,
`TransportError`, and `ProtocolError` values.

## Install

```bash
go get github.com/alshell7/mizan-client-sdks/mizan-go@v1.2.0
```

Because the module lives in a repository subdirectory, repository release tags use
`mizan-go/v1.2.0`; consumers import `github.com/alshell7/mizan-client-sdks/mizan-go`.

## Configure and activate

```go
package main

import (
    "context"
    "log"
    "os"

    mizan "github.com/alshell7/mizan-client-sdks/mizan-go"
)

func main() {
    client, err := mizan.NewClient(os.Getenv("MIZAN_BASE_URL"), os.Getenv("MIZAN_API_TOKEN"))
    if err != nil { log.Fatal(err) }

    response, err := client.ActivateSubscription(context.Background(), "business-123", mizan.ActivationRequest{
        CatalogVersion: "azeer-2026-08-03-v2",
        PlanID:         mizan.PlanStart,
        Term:           mizan.TermMonthly,
        Seats:          1,
        PaymentStatus:  "confirmed",
        PaymentEventID: "checkout-session-001",
        Currency:       "SAR",
        PaidTotalMinor: "25300",
    }, "activate:business-123:checkout-session-001")
    if err != nil { log.Fatal(err) }

    activation, err := mizan.DecodeData[mizan.ActivationResult](response)
    if err != nil { log.Fatal(err) }
    log.Print(activation.Invoice.TotalMinor)
}
```

## Consume atomically

```go
response, err := client.Consume(ctx, "business-123", mizan.ConsumptionRequest{
    SourceEventID: "message-delivered-001",
    OccurredAt:    time.Now().UTC(),
    FeatureCode:   "outbound_delivered_message",
    Quantity:      "1",
    Metadata: &mizan.UsageMetadata{
        Channel:         "whatsapp",
        ProviderEventID: "meta-001",
    },
}, "consume:message-delivered-001")
if err != nil { /* handle below */ }

decision, err := mizan.DecodeData[mizan.ConsumptionResult](response)
if err != nil { log.Fatal(err) }
fmt.Println(decision.Balances.AzeerUnitMillis)
```

Use `Components` for mixed-rail usage. The Durable Object accepts or rejects the complete event in one
transaction.

## Exact values and errors

Never parse `ExactAmount` through `float64`. Money is integer halala; units are integer milliunits.

```go
var apiErr *mizan.APIError
var transportErr *mizan.TransportError
var protocolErr *mizan.ProtocolError

switch {
case errors.As(err, &apiErr):
    // Stable server decision. apiErr.Retryable controls safe SDK retries.
case errors.As(err, &transportErr):
    // Unknown mutation outcome: retry identical input with transportErr.IdempotencyKey.
case errors.As(err, &protocolErr):
    // Invalid response: preserve protocolErr.IdempotencyKey while investigating.
}
```

`Response` remains a forward-compatible map. `DecodeData[T]` converts its `data` member into documented
result structs without altering exact strings.

## Test

```bash
go test ./...
go vet ./...
```

Linux CI also runs `go test -race ./...` on Go 1.22 and 1.23.
