# Go end-to-end examples

These programs are compile-checked by `go test ./...`. Run them only from a trusted backend environment; both API and webhook bearer tokens are secrets.

## Customer billing lifecycle

[`end_to_end`](end_to_end/main.go) executes the complete happy path:

1. loads the current catalog version;
2. activates a Start/monthly subscription from a confirmed payment;
3. optionally funds the provider balance from a second confirmed payment;
4. checks a capability entitlement and an advisory usage eligibility decision;
5. records an authoritative delivered-message consumption;
6. replays the identical mutation and verifies that no second ledger identity is created;
7. reads the billing summary and exports every ledger page.

Required environment variables:

| Variable | Meaning |
|---|---|
| `MIZAN_BASE_URL` | Mizan runtime Worker URL |
| `MIZAN_API_TOKEN` | Runtime bearer credential scoped to `MIZAN_BUSINESS_ID` in production |
| `MIZAN_BUSINESS_ID` | A fresh business, or the business used by the same example run |
| `MIZAN_ACTIVATION_PAID_TOTAL_MINOR` | Trusted VAT-inclusive activation payment in integer halala |

Optional variables:

| Variable | Default | Meaning |
|---|---|---|
| `MIZAN_EXAMPLE_RUN_ID` | `checkout-001` | Stable workflow identity used in payment, source-event, and idempotency keys |
| `MIZAN_BUSINESS_TIMEZONE` | `Asia/Riyadh` | IANA timezone frozen into the subscription |
| `MIZAN_PROVIDER_TOP_UP_MINOR` | unset | Exact provider-balance principal in halala |
| `MIZAN_PROVIDER_TOP_UP_PAID_TOTAL_MINOR` | unset | Trusted VAT-inclusive provider top-up payment; set together with the principal |

```bash
go run ./examples/end_to_end
```

The example uses the current UTC instant because it creates and consumes the event immediately. A real service must persist the actual timezone-aware event timestamp and report it only while it belongs to the active subscription's current open month. Never rebuild a mutation body with a new timestamp during an uncertain retry: replay the stored body and idempotency key exactly.

Use a new run ID for a genuinely new checkout. Reusing a key with changed payment facts or a changed catalog version correctly produces `IDEMPOTENCY_KEY_REUSED`.

## Webhook receiver

[`webhook_receiver`](webhook_receiver/main.go) mounts the SDK receiver at `POST /mizan/webhooks`. It authenticates and validates the payload, then fsyncs an idempotent file-inbox record before returning success. Ledger acknowledgement is therefore emitted only after durable acceptance.

```bash
export MIZAN_WEBHOOK_TOKEN='replace-from-secret-manager'
export MIZAN_WEBHOOK_INBOX='/var/lib/my-service/mizan-inbox'
export MIZAN_WEBHOOK_ADDR=':8080'
go run ./examples/webhook_receiver
```

The file inbox is intentionally a single-process reference implementation. For multiple replicas, implement the same callback boundary with a shared transactional database and unique constraints on both outbox ID and ledger event ID. Process/project the durable inbox asynchronously, or commit the business projection and inbox receipt in the same database transaction.

`consume_features` remains the compact, compile-checked reference for every individual metering feature.
