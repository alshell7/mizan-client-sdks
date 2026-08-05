# Consuming Mizan webhooks

The SDK receiver gives an application one entry point for both outbound streams. It authenticates the request, validates the JSON contract, identifies whether it is a ledger event or an operational notification, invokes the matching callback, and generates the correct HTTP response.

Delivery is **at least once**. The callback must commit its business effect durably before returning. If it raises or returns an error, the SDK returns HTTP 500 and Mizan retries the same delivery.

## The four identifiers

| Value | Scope | Stable across a retry? | What the consumer should do |
|---|---|---:|---|
| `X-Mizan-Outbox-Id` | One outbound delivery item | Yes | Use as the primary inbox/deduplication key. Persist it with the application effect. |
| `event_id` | One immutable ledger entry | Yes | Use as the ledger event identity. It is equal to `entry.id`; it is not a retry-attempt ID. |
| `entry.source_event_id` | Upstream payment, usage, job, or admin correlation | Yes | Use for cross-system audit and reconciliation. Do not assume it is globally unique outside its source/domain. |
| `business_sequence` | Ordered ledger position within one business | Yes | Persist as the per-business replication cursor and return it in `X-Mizan-Ack-Sequence`. The SDK emits this header after the callback succeeds. |

Notifications have an outbox ID but no event ID or business sequence. They are not an ordered financial replication stream.

## Correct transaction and retry pattern

For every callback:

1. Begin an application database transaction.
2. Look up `WebhookContext.outbox_id` in a durable inbox table with a unique constraint.
3. If it already exists, make no repeated application change and commit successfully. The SDK will return the same successful response again.
4. For a new ledger event, verify the stored cursor for `business_id`. The expected sequence is normally the previous cursor plus one. A brand-new consumer may establish its initial cursor from an agreed backfill boundary.
5. Apply the application change, insert the outbox ID and event ID, and update the business cursor in the same transaction.
6. Commit, then let the callback return.

Do not store the outbox ID after committing the application effect in a separate transaction. A crash between those operations could repeat the effect. Do not return success while work is merely queued unless enqueueing into that durable queue is itself the complete, deduplicated application effect.

Mizan already serializes ledger delivery per business. A missing or failed ledger sequence blocks later sequences. The consumer-side cursor is still valuable: it detects accidental endpoint sharing, database restoration, deleted inbox state, and an incorrect backfill boundary.

When a callback fails, the SDK intentionally sends no ledger acknowledgement. Mizan retries with the same body and `X-Mizan-Outbox-Id`. Retry delay grows exponentially with jitter, is capped at one hour, and the item becomes a dead letter after the tenth failed receiver attempt. Timeouts are uncertain outcomes, so duplicate delivery must be expected even if the first callback committed successfully.

## Receiver contract

Mizan sends an HTTPS `POST` with:

- `Content-Type: application/json`
- `X-Mizan-Outbox-Id: <stable delivery ID>`
- `Authorization: Bearer <receiver secret>` when bearer auth is configured

The default SDK maximum body size is 1 MiB. The receiver rejects malformed payloads, non-canonical exact amounts, unsupported event types, ledger events whose `event_id` differs from `entry.id`, and postings that do not balance to zero per rail and unit.

On success:

- ledger: HTTP 204 and `X-Mizan-Ack-Sequence` equal to `business_sequence`;
- notification: HTTP 204, with no sequence acknowledgement.

The SDK supports synchronous or asynchronous Python callbacks. Go callbacks use the request context. The SDK removes `Authorization` from callback headers and never returns callback exception text to the sender; applications must also keep configured secrets out of logs.

## Ledger JSON model

```json
{
  "event_id": "8ceae0fe-64b0-4c36-a239-c46d2a3ab777",
  "business_id": "business_123",
  "business_sequence": 42,
  "entry": {
    "id": "8ceae0fe-64b0-4c36-a239-c46d2a3ab777",
    "entry_type": "usage_consumed",
    "source_command": "consume",
    "source_event_id": "usage_01J...",
    "subscription_id": "sub_01J...",
    "feature_code": "outbound_delivered_message",
    "effective_at": "2026-08-05T12:00:00.000Z",
    "catalog_version": "azeer-2026-08-03-v2",
    "policy_version": "policy-2026-08-03-v2",
    "metadata": {"components": ["outbound_delivered_message"]}
  },
  "postings": [
    {"rail": "azeer_units", "account_code": "azeer_units", "amount": "-1000", "unit": "milliunit", "lot_id": "lot_01J..."},
    {"rail": "azeer_units", "account_code": "usage:outbound_delivered_message", "amount": "1000", "unit": "milliunit"}
  ]
}
```

`effective_at` is the business-effective time, not the delivery time. `metadata` is entry-type-specific audit data. Posting amounts are signed base-10 integer strings: Azeer Units use `milliunit`, and SAR rails use `halala`. Entries that record state or audit decisions without moving value can have an empty `postings` array.

## Ledger entry types

| `entry_type` | What it means | Common source/audit data |
|---|---|---|
| `subscription_activated` | The first confirmed subscription was created and its initial included-unit grant and paid invoice were committed. | Payment event, subscription, invoice, immutable plan snapshot. |
| `subscription_change_scheduled` | A plan, term, seats, or add-on change was accepted for a future renewal boundary. No current balance changes. | Change request and effective time. |
| `subscription_cancellation_scheduled` | Access is scheduled to end at the current paid period boundary. | Cancellation event and reason. |
| `subscription_cancelled` | The subscription ended, either by its scheduled job or an audited immediate admin action. | Whether scheduled/immediate, admin attribution when applicable. |
| `subscription_cancellation_revoked` | An audited admin action removed a scheduled cancellation before it took effect. | Actor, role, reason, correlation ID. |
| `renewal_failed` | A unique payment-provider renewal failure moved the subscription/account into its past-due path. | Payment-provider event ID. |
| `subscription_renewed` | A confirmed renewal opened the next subscription period, applied any pending change, invoiced it, and granted included units. | Payment event, prior subscription, optional change ID, invoice. |
| `azeer_units_topped_up` | A confirmed catalog unit-package purchase created a purchased credit lot. | Payment event, expiry, principal, VAT. |
| `provider_balance_topped_up` | Confirmed funding increased the provider-cost wallet. | Payment event, before/after balance, VAT. |
| `provider_balance_refunded` | A confirmed refund reduced provider principal and posted it to refund clearing. | Payment event, reason, before/after balance. |
| `feature_budget_updated` | A monthly feature budget, threshold, action, or reserve setting changed. | Feature, metric, exact limit, warning basis points, action. |
| `billing_account_depleted` | A consumption attempt found insufficient Azeer Units or provider balance and marked the account depleted. | Rejection reason and available/required details. |
| `feature_paused_budget` | Consumption was rejected because a pause budget was reached or remained active. | Feature, budget period and limit details. |
| `feature_paused_manual` | An audited admin action manually paused a feature. | Actor, role, reason, correlation ID. |
| `feature_resumed_manual` | An audited admin action removed a manual feature pause. | Actor, role, reason, correlation ID. |
| `usage_rejected` | A billable source event was rejected for a reason not represented by the dedicated depletion or budget-pause entry. No charge postings. | Source event, feature(s), rejection code/details. |
| `usage_consumed` | A billable source event was accepted and its unit/provider postings and lot allocations were committed. | Source event, components, application metadata, allocations. |
| `included_units_expired` | Remaining included units expired at their lifecycle boundary or were closed during cancellation/renewal handling. | Lot IDs and exact expired milliunits. |
| `purchased_units_expired` | A purchased unit lot reached its expiry with a remaining balance. | Lot ID and exact expired milliunits. |
| `promotional_units_expired` | A promotional unit lot reached its expiry with a remaining balance. | Lot ID and exact expired milliunits. |
| `promotional_units_granted` | An audited admin action created a promotional credit lot. | Actor, reason, expiry, lot. |
| `monthly_included_units_granted` | A scheduled monthly grant expired any prior monthly included remainder and created the next included lot. | Job ID, expired/granted milliunits, lot ID. |
| `outbox_retry_requested` | An operator reviewed and manually reset a delivery item for retry. This is an audit entry, not proof that delivery succeeded. | Actor, reason, retried outbox ID. |

## Notification JSON models

The five notification types are:

- `budget_warning`: first accepted consumption reaches the warning threshold; includes `period`, `projected`, and `limit`.
- `budget_breached`: first accepted consumption exceeds an alert-only limit; includes `period`, `projected`, and `limit`.
- `budget_paused`: consumption is rejected while a pause budget is active; includes `period`.
- `feature_paused_manual`: an admin manually pauses a feature.
- `feature_resumed_manual`: an admin removes a manual feature pause.

Every notification includes `type`, `business_id`, and `feature_code`.

## Go Fiber

```go
import (
    "context"

    "github.com/alshell7/mizan-client-sdks/mizan-go"
    mizanfiber "github.com/alshell7/mizan-client-sdks/mizan-go/fiber"
    "github.com/gofiber/fiber/v2"
)

receiver := mizan.WebhookReceiver{
    BearerToken: mustSecret("MIZAN_WEBHOOK_SECRET"),
    Handler: mizan.WebhookHandlerFuncs{
        Ledger: func(ctx context.Context, event mizan.LedgerWebhook, delivery mizan.WebhookContext) error {
            return inbox.ApplyLedgerOnce(ctx, delivery.OutboxID, event)
        },
        Notification: func(ctx context.Context, event mizan.NotificationWebhook, delivery mizan.WebhookContext) error {
            return inbox.ApplyNotificationOnce(ctx, delivery.OutboxID, event)
        },
    },
}

app := fiber.New()
app.Post("/integrations/mizan", mizanfiber.Middleware(receiver))
```

The same receiver also implements `http.Handler`. For another Go framework, call `receiver.Receive(ctx, headers, rawJSON)` and map the returned status, headers, and body to that framework's response.

## Python FastAPI

Install the optional integration:

```bash
pip install "mizan-billing[fastapi]"
```

```python
from fastapi import FastAPI
from mizan import LedgerWebhook, NotificationWebhook, WebhookContext, WebhookReceiver
from mizan.fastapi import mount_webhooks

async def ledger(event: LedgerWebhook, delivery: WebhookContext) -> None:
    await inbox.apply_ledger_once(delivery.outbox_id, event)

async def notification(event: NotificationWebhook, delivery: WebhookContext) -> None:
    await inbox.apply_notification_once(delivery.outbox_id, event)

receiver = WebhookReceiver(
    bearer_token=settings.mizan_webhook_secret,
    on_ledger=ledger,
    on_notification=notification,
)

app = FastAPI()
mount_webhooks(app, receiver, "/integrations/mizan")
```

For Flask, Django, Starlette, or a custom server, call `await receiver.receive(headers, raw_json)`. The payload argument may be bytes, a string, or an already decoded mapping. Return its `status_code`, `headers`, and `body` unchanged.

## Operational handling

- Monitor non-2xx receiver responses, callback latency, duplicate count, and the per-business sequence cursor.
- Alert when the next received ledger sequence is not the expected one; do not silently skip a gap.
- Preserve the original event and identifiers for reconciliation, while keeping the bearer secret out of logs.
- A manual delivery command makes eligible pending/retrying items due immediately but does not change the deduplication contract.
- A reviewed dead-letter retry keeps the original outbox item identity. The audit operation also creates a separate `outbox_retry_requested` ledger event.
