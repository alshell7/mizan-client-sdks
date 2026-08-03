# Mizan Python SDK

Typed Python 3.10+ client with no runtime dependencies. It validates configuration, limits responses to
2 MiB, adds request correlation/timestamps, and performs bounded mutation retries without changing the
encoded body or idempotency key.

## Install

After the first PyPI release:

```bash
python -m pip install mizan-billing
```

From a checkout:

```bash
python -m pip install ./mizan-python
```

The distribution is `mizan-billing`; import it as `mizan`.

## Configure and activate

```python
import os
from mizan import ActivationRequest, MizanClient

client = MizanClient(
    os.environ["MIZAN_BASE_URL"],
    os.environ["MIZAN_API_TOKEN"],
    timeout=10.0,
    max_attempts=3,
)

request: ActivationRequest = {
    "catalog_version": "azeer-2026-08-03-v2",
    "plan_id": "start",
    "term": "monthly",
    "seats": 1,
    "payment_status": "confirmed",
    "payment_event_id": "checkout-session-001",
    "currency": "SAR",
    "paid_total_minor": "25300",
}
result = client.activate_subscription(
    "business-123",
    request,
    idempotency_key="activate:business-123:checkout-session-001",
)
print(result["data"]["invoice"]["total_minor"])
```

`paid_total_minor` must come from the authoritative checkout/payment flow and exactly match Mizan's
catalog invoice. A mismatch fails closed.

## Check and consume

Eligibility is a read-only preview. Consumption is the authoritative atomic mutation.

```python
from datetime import datetime, timezone
from mizan import ConsumptionRequest

preview = client.check_eligibility(
    "business-123",
    "outbound_delivered_message",
    {"quantity": "1"},
)

usage: ConsumptionRequest = {
    "source_event_id": "message-delivered-001",
    "occurred_at": datetime.now(timezone.utc).isoformat(),
    "feature_code": "outbound_delivered_message",
    "quantity": "1",
    "metadata": {"channel": "whatsapp", "provider_event_id": "meta-001"},
}
decision = client.consume(
    "business-123",
    usage,
    idempotency_key="consume:message-delivered-001",
)
print(decision["data"]["balances"]["azeer_unit_millis"])
```

For combined usage, send `components`; Mizan accepts or rejects all components together.

## Exact values

| Field suffix | Unit | Python type | Example |
|---|---|---|---|
| `_minor` | halala | `str` / `ExactAmount` | `"75"` = SAR 0.75 |
| `_millis` | Azeer milliunit | `str` / `ExactAmount` | `"500"` = 0.5 unit |
| `_bps` | basis points | `int` | `1500` = 15% |

Use `decimal.Decimal` only for UI conversion; never convert exact values to `float`.

## Error and retry contract

```python
from mizan import MizanAPIError, MizanProtocolError, MizanTransportError

try:
    client.consume("business-123", usage, idempotency_key="consume:message-delivered-001")
except MizanAPIError as error:
    # Stable domain/API decision. Retry only when error.retryable is true.
    print(error.status, error.code, error.details, error.request_id)
except (MizanTransportError, MizanProtocolError) as error:
    # Outcome can be unknown. Persist this key and retry the IDENTICAL body.
    print(error.idempotency_key, error.request_id)
```

If no mutation key is supplied, the SDK creates one and exposes it on transport/protocol exceptions.
Supplying and persisting your own domain key is recommended.

## Methods

| Method | Effect |
|---|---|
| `get_catalog` | Read immutable plan/add-on/usage pricing templates |
| `activate_subscription` | Create the first paid subscription and unit grant |
| `change_subscription` | Schedule one next-renewal catalog change |
| `cancel_subscription` | Schedule period-end cancellation |
| `apply_renewal_event` | Apply a confirmed/failed provider renewal event |
| `top_up_azeer_units` | Add a 12-month purchased unit lot |
| `top_up_provider_balance` | Fund prepaid provider costs |
| `refund_provider_balance` | Apply a confirmed refund |
| `set_feature_budget` | Configure monthly alert/pause behavior |
| `check_eligibility` | Preview a charge without mutation |
| `get_entitlement` | Read plan capability/fair-use state |
| `consume` | Atomically decide, debit, ledger, and deduplicate usage |
| `get_billing_summary` | Read subscription, balances, lots, budgets, and replication |
| `get_ledger` | Page immutable ledger entries by business sequence |

All public request and response types are exported from `mizan`; the package includes `py.typed`.

## Test and build

```bash
python -m pip install --editable ".[dev]"
python -m unittest discover -s tests -v
python -m mypy src
python -m build
python -m twine check dist/*
```
