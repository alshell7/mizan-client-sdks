"""Validated feature-specific request builders for the Mizan API.

Each public usage builder maps to exactly one feature code and accepts only that
feature's billable input. Quantity-based features default to one. Provider-priced
features require financial attribution before any HTTP request is attempted.
"""
from __future__ import annotations

import re
import math
from datetime import datetime
from typing import Any, cast

from .enums import (BudgetAction, BudgetMetric, BudgetPeriod, Channel, Currency,
                    FeatureCode, PaymentStatus, RefundStatus)
from .models import (AIAssistActionConsumptionRequest,
                     AIAssistActionOverAllowanceConsumptionRequest,
                     AIReplyHandlingConsumptionRequest, BudgetRequest,
                     ConfirmedTopUp, Conversation24HConsumptionRequest,
                     ConversationUsageMetadata,
                     InboundVoiceMinuteConsumptionRequest,
                     OtherProviderChargeConsumptionRequest,
                     OutboundDeliveredMessageConsumptionRequest,
                     PassThroughProviderUsageMetadata, ProviderRefundRequest,
                     ProviderUsageMetadata,
                     TelephonyVoiceMinuteConsumptionRequest, UsageMetadata,
                     VoiceAIStartedMinuteConsumptionRequest,
                     WhatsAppMetaMarketingMessageConsumptionRequest)

# The Worker persists exact quantities in the positive signed-int64 domain.
_MAX_INT64 = 9_223_372_036_854_775_807
# Integer contracts reject signs, whitespace, exponent notation, and decimal points.
_INTEGER = re.compile(r"^\d+$")
# Count/minute contracts accept up to milli precision and never round caller facts.
_DECIMAL = re.compile(r"^(\d+)(?:\.(\d{1,3}))?$")


def confirmed_top_up(*, amount_minor: str, payment_event_id: str, paid_total_minor: str) -> ConfirmedTopUp:
    """Build confirmed SAR funding; ``paid_total_minor`` must include VAT."""
    _integer(amount_minor, "amount_minor")
    _integer(paid_total_minor, "paid_total_minor")
    _payment_event(payment_event_id)
    return {"amount_minor": amount_minor, "payment_event_id": payment_event_id,
            "payment_status": PaymentStatus.CONFIRMED, "currency": Currency.SAR,
            "paid_total_minor": paid_total_minor}


def confirmed_refund(*, amount_minor: str, refunded_total_minor: str,
                     payment_event_id: str, reason: str) -> ProviderRefundRequest:
    """Build a confirmed refund; total must include the principal's VAT reversal."""
    _integer(amount_minor, "amount_minor")
    _integer(refunded_total_minor, "refunded_total_minor")
    _payment_event(payment_event_id)
    if not isinstance(reason, str) or not reason.strip() or len(reason) > 1_000:
        raise ValueError("reason must contain 1 to 1000 characters")
    return {"amount_minor": amount_minor, "payment_event_id": payment_event_id, "reason": reason,
            "refund_status": RefundStatus.CONFIRMED, "currency": Currency.SAR,
            "refunded_total_minor": refunded_total_minor}


def feature_budget(*, metric: BudgetMetric, limit: str, action: BudgetAction,
                   warning_bps: int = 8000, sensitive: bool = False) -> BudgetRequest:
    """Build a subscription-month feature budget with an exact non-negative limit."""
    return {"metric": metric, "period": BudgetPeriod.SUBSCRIPTION_MONTH.value,
            "limit": limit, "warning_bps": warning_bps, "action": action, "sensitive": sensitive}


def _event(source_event_id: str, occurred_at: str) -> None:
    """Validate the application dedupe key and timezone-aware event time."""
    if not isinstance(source_event_id, str) or not 1 <= len(source_event_id) <= 128:
        raise ValueError("source_event_id must contain 1 to 128 characters")
    if not isinstance(occurred_at, str):
        raise ValueError("occurred_at must be an ISO-8601 timestamp with a timezone")
    try:
        # Python 3.10 does not accept a trailing Z, so normalize it to UTC offset syntax.
        value = datetime.fromisoformat(occurred_at.replace("Z", "+00:00"))
    except ValueError as exc:
        raise ValueError("occurred_at must be an ISO-8601 timestamp with a timezone") from exc
    if value.tzinfo is None:
        raise ValueError("occurred_at must include a timezone")


def _payment_event(payment_event_id: str) -> None:
    """Validate the payment-provider deduplication identifier."""
    if not isinstance(payment_event_id, str) or not 1 <= len(payment_event_id) <= 128:
        raise ValueError("payment_event_id must contain 1 to 128 characters")


def _quantity(value: str) -> None:
    """Validate an exact positive decimal and its scaled int64 representation."""
    match = _DECIMAL.fullmatch(value) if isinstance(value, str) else None
    if not match:
        raise ValueError("quantity must be a positive decimal string with at most 3 decimal places")
    fraction = (match.group(2) or "").ljust(3, "0")
    # Scale with integers to mirror the Worker's parseDecimal implementation exactly.
    scaled = int(match.group(1)) * 1_000 + int(fraction or "0")
    if not 0 < scaled <= _MAX_INT64:
        raise ValueError("quantity must be positive and fit the supported exact range")


def _whole_count(value: str) -> None:
    """Validate an indivisible event count whose milli representation fits int64."""
    if not isinstance(value, str) or not _INTEGER.fullmatch(value):
        raise ValueError("quantity must be a positive whole-count string")
    parsed = int(value)
    if not 0 < parsed <= _MAX_INT64 // 1_000:
        raise ValueError("quantity must be positive and fit the supported exact range after milli scaling")


def _integer(value: str, field: str, *, allow_zero: bool = False) -> None:
    """Validate a canonical unsigned integer fact without numeric coercion."""
    if not isinstance(value, str) or not _INTEGER.fullmatch(value):
        raise ValueError(f"{field} must be a non-negative integer string")
    parsed = int(value)
    if parsed > _MAX_INT64 or (parsed == 0 and not allow_zero):
        qualifier = "non-negative" if allow_zero else "positive"
        raise ValueError(f"{field} must be {qualifier} and fit the supported exact range")


def _validate_metadata(metadata: UsageMetadata | ProviderUsageMetadata | ConversationUsageMetadata
                       | PassThroughProviderUsageMetadata | None) -> None:
    """Bound optional attribution to the Worker's small scalar metadata contract."""
    if metadata is None:
        return
    reserved = {"actor", "channel", "channel_account_id", "provider", "provider_event_id",
                "conversation_id", "campaign_id", "raw_quantity", "billable_quantity",
                "provider_invoice_id", "original_amount_minor", "original_currency", "fx_rule",
                "tariff_version", "attributes"}
    if any(key not in reserved for key in metadata):
        raise ValueError("custom metadata top-level keys are not supported; use metadata.attributes")
    channel = metadata.get("channel")
    if channel is not None and str(channel) not in {item.value for item in Channel}:
        raise ValueError("metadata.channel is not a supported channel")
    actor = metadata.get("actor")
    if actor is not None and (not isinstance(actor, dict) or set(actor) - {"type", "id"}
                              or actor.get("type") not in {"user", "system", "campaign"}
                              or not isinstance(actor.get("id"), str) or not 1 <= len(actor["id"]) <= 128):
        raise ValueError("metadata.actor requires a supported type and bounded id")
    for field in ("channel_account_id", "provider", "provider_event_id", "conversation_id", "campaign_id",
                  "raw_quantity", "billable_quantity", "provider_invoice_id", "original_currency", "fx_rule",
                  "tariff_version"):
        value = metadata.get(field)
        if value is not None and (not isinstance(value, str) or not 1 <= len(value) <= 512):
            raise ValueError(f"metadata.{field} must contain 1 to 512 characters")
    if metadata.get("original_amount_minor") is not None:
        _integer(str(metadata["original_amount_minor"]), "metadata.original_amount_minor", allow_zero=True)
    attributes = metadata.get("attributes", {})
    if not isinstance(attributes, dict) or len(attributes) > 32:
        raise ValueError("metadata.attributes must be an object with at most 32 entries")
    for key, value in attributes.items():
        # Metadata is reconciliation context, not an unrestricted provider-payload store.
        if not isinstance(key, str) or len(key) > 64 or value is not None and not isinstance(value, (str, int, float, bool)):
            raise ValueError("metadata.attributes contains an invalid key or scalar value")
        if value is not None and len(str(value)) > 512:
            raise ValueError("metadata attribute values must be at most 512 characters")
        if isinstance(value, float) and not math.isfinite(value):
            raise ValueError("metadata attribute numbers must be finite")


def _count_usage(feature_code: FeatureCode, *, source_event_id: str, occurred_at: str,
                 quantity: str, metadata: UsageMetadata | ProviderUsageMetadata
                 | ConversationUsageMetadata | None,
                 whole_count: bool = True) -> dict[str, Any]:
    """Build the shared wire shape only after every local fact has passed validation."""
    _event(source_event_id, occurred_at)
    (_whole_count if whole_count else _quantity)(quantity)
    _validate_metadata(metadata)
    request: dict[str, Any] = {"source_event_id": source_event_id, "occurred_at": occurred_at,
                               "feature_code": feature_code, "quantity": quantity}
    if metadata is not None:
        request["metadata"] = dict(metadata)
    return request


def _provider_metadata(*, provider: str, provider_event_id: str,
                       metadata: UsageMetadata | None = None) -> ProviderUsageMetadata:
    """Merge required provider attribution over optional caller metadata."""
    _validate_metadata(metadata)
    if not isinstance(provider, str) or not provider.strip():
        raise ValueError("provider is required for provider-priced usage")
    if not isinstance(provider_event_id, str) or not provider_event_id.strip():
        raise ValueError("provider_event_id is required for provider-priced usage")
    result = dict(metadata or {})
    # Required arguments win over conflicting optional metadata to keep attribution canonical.
    result["provider"] = provider.strip()
    result["provider_event_id"] = provider_event_id.strip()
    return cast(ProviderUsageMetadata, result)


def _required_metadata_string(value: str, field: str) -> str:
    if not isinstance(value, str) or not value.strip() or len(value.strip()) > 512:
        raise ValueError(f"{field} must contain 1 to 512 characters")
    return value.strip()


def conversation_24h(*, source_event_id: str, occurred_at: str, conversation_id: str,
                     channel: Channel,
                     metadata: UsageMetadata | None = None) -> Conversation24HConsumptionRequest:
    """Report conversation activity; Mizan owns opening and deduplicating the 24-hour window."""
    _validate_metadata(metadata)
    resolved_channel = str(channel)
    if resolved_channel not in {item.value for item in Channel}:
        raise ValueError("channel is not a supported channel")
    resolved: dict[str, Any] = dict(metadata or {})
    # Explicit identity wins so optional caller metadata cannot redirect window accounting.
    resolved["conversation_id"] = _required_metadata_string(conversation_id, "conversation_id")
    resolved["channel"] = resolved_channel
    return cast(Conversation24HConsumptionRequest, _count_usage(
        FeatureCode.CONVERSATION_24H, source_event_id=source_event_id,
        occurred_at=occurred_at, quantity="1",
        metadata=cast(ConversationUsageMetadata, resolved)))


def outbound_delivered_message(*, source_event_id: str, occurred_at: str, quantity: str = "1",
                               metadata: UsageMetadata | None = None) -> OutboundDeliveredMessageConsumptionRequest:
    """Build ``outbound_delivered_message``; provider/Meta fees are separate charges."""
    return cast(OutboundDeliveredMessageConsumptionRequest, _count_usage(
        FeatureCode.OUTBOUND_DELIVERED_MESSAGE, source_event_id=source_event_id,
        occurred_at=occurred_at, quantity=quantity, metadata=metadata))


def ai_assist_action_over_allowance(*, source_event_id: str, occurred_at: str, quantity: str = "1",
                                    metadata: UsageMetadata | None = None) -> AIAssistActionOverAllowanceConsumptionRequest:
    """Report every AI-assist action; Mizan decides included versus over-allowance quantity."""
    return cast(AIAssistActionOverAllowanceConsumptionRequest, _count_usage(
        FeatureCode.AI_ASSIST_ACTION_OVER_ALLOWANCE, source_event_id=source_event_id,
        occurred_at=occurred_at, quantity=quantity, metadata=metadata))


def ai_assist_action(*, source_event_id: str, occurred_at: str, quantity: str = "1",
                     metadata: UsageMetadata | None = None) -> AIAssistActionConsumptionRequest:
    """Preferred semantic alias for reporting every AI-assist action."""
    return ai_assist_action_over_allowance(
        source_event_id=source_event_id, occurred_at=occurred_at,
        quantity=quantity, metadata=metadata)


def ai_reply_handling(*, source_event_id: str, occurred_at: str, quantity: str = "1",
                      metadata: UsageMetadata | None = None) -> AIReplyHandlingConsumptionRequest:
    """Build zero-charge ``ai_reply_handling`` usage for audit and fair-use visibility."""
    return cast(AIReplyHandlingConsumptionRequest, _count_usage(
        FeatureCode.AI_REPLY_HANDLING, source_event_id=source_event_id,
        occurred_at=occurred_at, quantity=quantity, metadata=metadata))


def voice_ai_started_minute(*, source_event_id: str, occurred_at: str, duration_seconds: str,
                            metadata: UsageMetadata | None = None) -> VoiceAIStartedMinuteConsumptionRequest:
    """Build ``voice_ai_started_minute`` from raw seconds; Mizan performs started-minute rounding."""
    _event(source_event_id, occurred_at)
    _integer(duration_seconds, "duration_seconds")
    _validate_metadata(metadata)
    request: dict[str, Any] = {"source_event_id": source_event_id, "occurred_at": occurred_at,
                               "feature_code": FeatureCode.VOICE_AI_STARTED_MINUTE,
                               "duration_seconds": duration_seconds}
    if metadata is not None:
        request["metadata"] = dict(metadata)
    return cast(VoiceAIStartedMinuteConsumptionRequest, request)


def whatsapp_meta_marketing_message(*, source_event_id: str, occurred_at: str, provider_event_id: str,
                                    quantity: str = "1", metadata: UsageMetadata | None = None
                                    ) -> WhatsAppMetaMarketingMessageConsumptionRequest:
    """Build Meta marketing-message usage. Provider is fixed to ``Meta`` and quantity defaults to one."""
    provider_metadata = _provider_metadata(provider="Meta", provider_event_id=provider_event_id, metadata=metadata)
    return cast(WhatsAppMetaMarketingMessageConsumptionRequest, _count_usage(
        FeatureCode.WHATSAPP_META_MARKETING_MSG, source_event_id=source_event_id,
        occurred_at=occurred_at, quantity=quantity, metadata=provider_metadata))


def telephony_voice_minute(*, source_event_id: str, occurred_at: str, provider: str,
                           provider_event_id: str, billable_minutes: str = "1",
                           metadata: UsageMetadata | None = None) -> TelephonyVoiceMinuteConsumptionRequest:
    """Build provider-normalized outbound minutes; never pass raw call seconds here."""
    provider_metadata = _provider_metadata(provider=provider, provider_event_id=provider_event_id, metadata=metadata)
    return cast(TelephonyVoiceMinuteConsumptionRequest, _count_usage(
        FeatureCode.TELEPHONY_VOICE_MINUTE, source_event_id=source_event_id,
        occurred_at=occurred_at, quantity=billable_minutes, metadata=provider_metadata, whole_count=False))


def inbound_voice_minute(*, source_event_id: str, occurred_at: str, provider: str,
                         provider_event_id: str, billable_minutes: str = "1",
                         metadata: UsageMetadata | None = None) -> InboundVoiceMinuteConsumptionRequest:
    """Build provider-normalized inbound minutes; the default catalog treatment is zero-rated."""
    provider_metadata = _provider_metadata(provider=provider, provider_event_id=provider_event_id, metadata=metadata)
    return cast(InboundVoiceMinuteConsumptionRequest, _count_usage(
        FeatureCode.INBOUND_VOICE_MINUTE, source_event_id=source_event_id,
        occurred_at=occurred_at, quantity=billable_minutes, metadata=provider_metadata, whole_count=False))


def other_provider_charge(*, source_event_id: str, occurred_at: str, provider: str,
                          provider_event_id: str, provider_amount_minor: str,
                          provider_invoice_id: str, original_amount_minor: str,
                          original_currency: str, tariff_version: str,
                          fx_rule: str | None = None,
                          metadata: UsageMetadata | None = None) -> OtherProviderChargeConsumptionRequest:
    """Build pass-through SAR settlement with complete provider invoice evidence."""
    _event(source_event_id, occurred_at)
    _integer(provider_amount_minor, "provider_amount_minor", allow_zero=True)
    _integer(original_amount_minor, "original_amount_minor", allow_zero=True)
    currency = _required_metadata_string(original_currency, "original_currency").upper()
    if not re.fullmatch(r"[A-Z]{3}", currency):
        raise ValueError("original_currency must be a three-letter ISO currency")
    if currency == "SAR" and original_amount_minor != provider_amount_minor:
        raise ValueError("SAR provider_amount_minor must equal original_amount_minor")
    if currency != "SAR" and (not isinstance(fx_rule, str) or not fx_rule.strip()):
        raise ValueError("fx_rule is required when original_currency is not SAR")
    provider_metadata = _provider_metadata(provider=provider, provider_event_id=provider_event_id, metadata=metadata)
    evidence: dict[str, Any] = dict(provider_metadata)
    evidence.update({
        "provider_invoice_id": _required_metadata_string(provider_invoice_id, "provider_invoice_id"),
        "original_amount_minor": original_amount_minor,
        "original_currency": currency,
        "tariff_version": _required_metadata_string(tariff_version, "tariff_version"),
    })
    if fx_rule is not None:
        evidence["fx_rule"] = _required_metadata_string(fx_rule, "fx_rule")
    return {"source_event_id": source_event_id, "occurred_at": occurred_at,
            "feature_code": "other_provider_charge", "provider_amount_minor": provider_amount_minor,
            "metadata": cast(PassThroughProviderUsageMetadata, evidence)}
