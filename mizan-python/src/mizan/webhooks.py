"""Framework-neutral receivers for Mizan ledger and notification webhooks."""

from __future__ import annotations

import hmac
import inspect
import json
from dataclasses import dataclass
from typing import Any, Awaitable, Callable, Literal, Mapping, TypeAlias, TypedDict, cast

from .models import ExactAmount

# Stable delivery identity; persist this value to suppress at-least-once duplicates.
OUTBOX_ID_HEADER = "x-mizan-outbox-id"
# Ledger receivers emit this header only after durable application success.
ACK_SEQUENCE_HEADER = "x-mizan-ack-sequence"
# Default protects framework adapters from unbounded request bodies.
DEFAULT_MAX_BODY_BYTES = 1 << 20

LedgerEntryType: TypeAlias = Literal[
    "subscription_activated",
    "subscription_change_scheduled",
    "subscription_cancellation_scheduled",
    "subscription_cancelled",
    "subscription_cancellation_revoked",
    "renewal_failed",
    "subscription_renewed",
    "azeer_units_topped_up",
    "provider_balance_topped_up",
    "provider_balance_refunded",
    "feature_budget_updated",
    "billing_account_depleted",
    "feature_paused_budget",
    "feature_paused_manual",
    "feature_resumed_manual",
    "usage_rejected",
    "usage_consumed",
    "included_units_expired",
    "purchased_units_expired",
    "promotional_units_expired",
    "promotional_units_granted",
    "monthly_included_units_granted",
    "outbox_retry_requested",
]

NotificationType: TypeAlias = Literal[
    "budget_warning",
    "budget_breached",
    "budget_paused",
    "feature_paused_manual",
    "feature_resumed_manual",
]


class LedgerEntryWebhook(TypedDict):
    """Immutable business-ledger entry with versioned effective-time metadata."""
    id: str
    entry_type: LedgerEntryType
    source_command: str
    source_event_id: str
    subscription_id: str | None
    feature_code: str | None
    effective_at: str
    catalog_version: str
    policy_version: str
    metadata: dict[str, Any]


class LedgerPostingWebhook(TypedDict, total=False):
    """One exact debit or credit; postings balance independently per rail and unit."""
    rail: Literal["azeer_units", "provider_balance", "invoice"]
    account_code: str
    amount: ExactAmount
    unit: Literal["milliunit", "halala"]
    lot_id: str | None
    metadata: dict[str, Any]


class LedgerWebhook(TypedDict):
    """Strictly ordered ledger delivery; deduplicate by outbox ID and event ID."""
    event_id: str
    business_id: str
    business_sequence: int
    entry: LedgerEntryWebhook
    postings: list[LedgerPostingWebhook]


class NotificationWebhook(TypedDict, total=False):
    """At-least-once operational notification without ledger ordering semantics."""
    type: NotificationType
    business_id: str
    feature_code: str
    period: str
    projected: ExactAmount
    limit: ExactAmount


@dataclass(frozen=True)
class WebhookContext:
    """Transport identity passed to application callbacks.

    ``outbox_id`` is stable across retries and should be the application's
    durable duplicate-suppression key. ``raw_body`` is retained for applications
    that archive the exact received contract.
    """

    outbox_id: str
    headers: Mapping[str, str]
    raw_body: bytes


@dataclass(frozen=True)
class WebhookResponse:
    """Framework-neutral HTTP response returned by :class:`WebhookReceiver`."""
    status_code: int
    headers: Mapping[str, str]
    body: bytes = b""


LedgerCallback: TypeAlias = Callable[[LedgerWebhook, WebhookContext], None | Awaitable[None]]
NotificationCallback: TypeAlias = Callable[[NotificationWebhook, WebhookContext], None | Awaitable[None]]
# Framework-neutral input supports raw bytes/strings and already-decoded test integrations.
WebhookPayload: TypeAlias = bytes | bytearray | str | Mapping[str, Any]


class WebhookReceiver:
    """Authenticate, validate, dispatch, and acknowledge both webhook streams.

    A callback must return only after its application effect has been durably
    committed. Raising an exception produces HTTP 500 without a ledger sequence
    acknowledgement, causing Mizan to retry the same outbox item.
    """

    def __init__(
        self,
        *,
        on_ledger: LedgerCallback,
        on_notification: NotificationCallback,
        bearer_token: str | None = None,
        max_body_bytes: int = DEFAULT_MAX_BODY_BYTES,
    ) -> None:
        if not callable(on_ledger) or not callable(on_notification):
            raise TypeError("on_ledger and on_notification must be callable")
        if bearer_token == "":
            raise ValueError("bearer_token must not be empty")
        if max_body_bytes < 1:
            raise ValueError("max_body_bytes must be positive")
        self._on_ledger = on_ledger
        self._on_notification = on_notification
        self._bearer_token = bearer_token
        self._max_body_bytes = max_body_bytes

    async def receive(self, headers: Mapping[str, str], payload: WebhookPayload) -> WebhookResponse:
        """Receive headers and raw JSON (or an already decoded JSON object).

        This is the integration point for any framework. Fiber and FastAPI
        adapters call the same method and therefore share identical behavior.
        """

        normalized_headers = {str(name).lower(): str(value) for name, value in headers.items()}
        # Authenticate before parsing so unauthorized callers cannot exercise the JSON contract.
        if not self._authorized(normalized_headers.get("authorization")):
            return _error_response(401, "webhook authorization failed")
        outbox_id = normalized_headers.get(OUTBOX_ID_HEADER, "").strip()
        if not outbox_id:
            return _error_response(400, "X-Mizan-Outbox-Id is required")

        try:
            raw_body, decoded = _decode_payload(payload, self._max_body_bytes)
        except _PayloadError as error:
            return _error_response(error.status_code, str(error))
        # Never forward the bearer credential to application callbacks or logging code.
        callback_headers = {name: value for name, value in normalized_headers.items() if name != "authorization"}
        context = WebhookContext(outbox_id=outbox_id, headers=callback_headers, raw_body=raw_body)

        if "business_sequence" in decoded:
            # Ledger shape takes precedence and requires ordered acknowledgement after callback success.
            try:
                event = _validate_ledger(decoded)
            except ValueError as error:
                return _error_response(422, str(error))
            try:
                result = self._on_ledger(event, context)
                if inspect.isawaitable(result):
                    await result
            except Exception:
                return _error_response(500, "ledger webhook processing failed")
            return WebhookResponse(204, {ACK_SEQUENCE_HEADER: str(event["business_sequence"])})

        if "type" in decoded:
            # Notifications are deduplicated by outbox ID but do not acknowledge a ledger sequence.
            try:
                notification = _validate_notification(decoded)
            except ValueError as error:
                return _error_response(422, str(error))
            try:
                result = self._on_notification(notification, context)
                if inspect.isawaitable(result):
                    await result
            except Exception:
                return _error_response(500, "notification webhook processing failed")
            return WebhookResponse(204, {})

        return _error_response(422, "unknown Mizan webhook payload")

    def _authorized(self, authorization: str | None) -> bool:
        if self._bearer_token is None:
            return True
        # Constant-time comparison avoids leaking a configured bearer token by timing.
        return authorization is not None and hmac.compare_digest(
            authorization.encode("utf-8"), f"Bearer {self._bearer_token}".encode("utf-8")
        )


class _PayloadError(ValueError):
    def __init__(self, status_code: int, message: str) -> None:
        super().__init__(message)
        self.status_code = status_code


def _decode_payload(payload: WebhookPayload, limit: int) -> tuple[bytes, dict[str, Any]]:
    """Normalize supported payload forms while preserving exact raw bytes for callbacks."""
    if isinstance(payload, Mapping):
        raw = json.dumps(payload, separators=(",", ":"), ensure_ascii=False).encode("utf-8")
        decoded: Any = dict(payload)
    else:
        raw = payload.encode("utf-8") if isinstance(payload, str) else bytes(payload)
        if not raw:
            raise _PayloadError(400, "request body is required")
        try:
            decoded = json.loads(raw)
        except (UnicodeDecodeError, json.JSONDecodeError) as error:
            raise _PayloadError(400, "request body must be a JSON object") from error
    if len(raw) > limit:
        raise _PayloadError(413, "request body exceeds the configured limit")
    if not isinstance(decoded, dict):
        raise _PayloadError(400, "request body must be a JSON object")
    return raw, cast(dict[str, Any], decoded)


# Closed entry vocabulary prevents silently accepting a ledger reason the SDK cannot interpret.
_LEDGER_TYPES = {
    "subscription_activated", "subscription_change_scheduled", "subscription_cancellation_scheduled",
    "subscription_cancelled", "subscription_cancellation_revoked", "renewal_failed",
    "subscription_renewed", "azeer_units_topped_up", "provider_balance_topped_up",
    "provider_balance_refunded", "feature_budget_updated", "billing_account_depleted",
    "feature_paused_budget", "feature_paused_manual", "feature_resumed_manual", "usage_rejected",
    "usage_consumed", "included_units_expired", "purchased_units_expired",
    "promotional_units_expired", "promotional_units_granted", "monthly_included_units_granted",
    "outbox_retry_requested",
}


def _validate_ledger(payload: dict[str, Any]) -> LedgerWebhook:
    """Validate ledger identity, exact postings, and independent rail/unit balance."""
    event_id = payload.get("event_id")
    business_id = payload.get("business_id")
    sequence = payload.get("business_sequence")
    entry = payload.get("entry")
    postings = payload.get("postings")
    if not isinstance(event_id, str) or not event_id or not isinstance(business_id, str) or not business_id:
        raise ValueError("ledger event_id and business_id are required")
    if isinstance(sequence, bool) or not isinstance(sequence, int) or sequence < 1:
        raise ValueError("business_sequence must be a positive integer")
    if not isinstance(entry, dict) or entry.get("id") != event_id:
        raise ValueError("event_id must be equal to entry.id")
    required_entry_strings = (
        "id", "entry_type", "source_command", "source_event_id", "effective_at", "catalog_version", "policy_version"
    )
    if any(not isinstance(entry.get(field), str) or not entry[field] for field in required_entry_strings):
        raise ValueError("ledger entry fields do not match the contract")
    if entry["entry_type"] not in _LEDGER_TYPES or not isinstance(entry.get("metadata"), dict):
        raise ValueError("ledger entry fields do not match the contract")
    if not isinstance(postings, list):
        raise ValueError("ledger postings must be an array")
    balances: dict[tuple[str, str], int] = {}
    for posting in postings:
        if not isinstance(posting, dict):
            raise ValueError("ledger posting fields do not match the contract")
        rail, unit, amount = posting.get("rail"), posting.get("unit"), posting.get("amount")
        if rail not in {"azeer_units", "provider_balance", "invoice"} or unit not in {"milliunit", "halala"}:
            raise ValueError("ledger posting fields do not match the contract")
        if not isinstance(posting.get("account_code"), str) or not posting["account_code"] or not _canonical_integer(amount):
            raise ValueError("ledger posting amounts must be exact integer strings")
        key = (rail, unit)
        # Money and unit rails must each balance independently to zero.
        balances[key] = balances.get(key, 0) + int(cast(str, amount))
    if any(balance != 0 for balance in balances.values()):
        raise ValueError("ledger postings must balance to zero per rail and unit")
    return cast(LedgerWebhook, payload)


def _validate_notification(payload: dict[str, Any]) -> NotificationWebhook:
    """Validate type-dependent notification fields and exact threshold amounts."""
    event_type = payload.get("type")
    if not isinstance(payload.get("business_id"), str) or not payload["business_id"]:
        raise ValueError("notification business_id and feature_code are required")
    if not isinstance(payload.get("feature_code"), str) or not payload["feature_code"]:
        raise ValueError("notification business_id and feature_code are required")
    if event_type in {"budget_warning", "budget_breached"}:
        if not isinstance(payload.get("period"), str) or not payload["period"]:
            raise ValueError("budget threshold notifications require period, projected, and limit")
        if not _unsigned_canonical_integer(payload.get("projected")) or not _unsigned_canonical_integer(payload.get("limit")):
            raise ValueError("budget threshold notifications require period, projected, and limit")
    elif event_type == "budget_paused":
        if not isinstance(payload.get("period"), str) or not payload["period"]:
            raise ValueError("budget_paused notifications require period")
    elif event_type not in {"feature_paused_manual", "feature_resumed_manual"}:
        raise ValueError("notification type is not supported")
    return cast(NotificationWebhook, payload)


def _canonical_integer(value: Any) -> bool:
    """Accept only canonical signed-int64 strings without leading zeroes."""
    if not isinstance(value, str) or not value:
        return False
    canonical = _unsigned_canonical_digits(value[1:]) if value.startswith("-") else _unsigned_canonical_digits(value)
    return canonical and -(2**63) <= int(value) <= 2**63 - 1


def _unsigned_canonical_integer(value: Any) -> bool:
    """Accept only canonical non-negative int64 strings."""
    return isinstance(value, str) and _unsigned_canonical_digits(value) and int(value) <= 2**63 - 1


def _unsigned_canonical_digits(value: str) -> bool:
    return value == "0" or (value[0:1] in "123456789" and value.isdigit())


def _error_response(status_code: int, message: str) -> WebhookResponse:
    body = json.dumps({"error": {"message": message}}, separators=(",", ":")).encode("utf-8")
    return WebhookResponse(status_code, {"content-type": "application/json"}, body)
