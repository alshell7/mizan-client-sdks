# Python end-to-end examples

The examples are checked by the package's strict MyPy configuration. Run them only from a trusted backend environment; API and webhook bearer tokens are secrets.

## Customer billing lifecycle

[`end_to_end.py`](end_to_end.py) executes the complete happy path:

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
| `MIZAN_API_TOKEN` | Runtime bearer credential scoped to `MIZAN_BUSINESS_ID` |
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
python examples/end_to_end.py
```

The example binds its business-scoped token to `MIZAN_BUSINESS_ID` and uses the activation response's `current_period_start` for its synthetic event time. A real service should persist the actual timezone-aware event timestamp with its domain event; consumption is accepted only within the currently open subscription month. Never rebuild a mutation body with a new timestamp during an uncertain retry: replay the stored body and idempotency key exactly.

Use a new run ID for a genuinely new checkout. Reusing a key with changed payment facts or a changed catalog version correctly produces `IDEMPOTENCY_KEY_REUSED`.

## FastAPI webhook receiver

[`webhook_receiver.py`](webhook_receiver.py) mounts the SDK receiver at `POST /mizan/webhooks`. It authenticates and validates the payload, then commits an idempotent SQLite inbox row before returning success. Ledger acknowledgement is therefore emitted only after durable acceptance.

```bash
export MIZAN_WEBHOOK_TOKEN='replace-from-secret-manager'
export MIZAN_WEBHOOK_INBOX='./data/mizan-webhooks.sqlite3'
uvicorn webhook_receiver:app --app-dir examples --host 0.0.0.0 --port 8080
```

Install the optional integration first with `pip install 'mizan-billing[fastapi]'`. SQLite is a single-node reference implementation; use a shared transactional database for multiple replicas. Keep unique constraints on both outbox ID and ledger event ID, then process/project the durable inbox asynchronously or in the same transaction as the receipt.

`consume_features.py` remains the compact, checked reference for every individual metering feature.
