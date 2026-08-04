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
from .models import (AIAssistActionOverAllowanceConsumptionRequest,
                     AIReplyHandlingConsumptionRequest, BudgetRequest,
                     ConfirmedTopUp, Conversation24HConsumptionRequest,
                     InboundVoiceMinuteConsumptionRequest,
                     OtherProviderChargeConsumptionRequest,
                     OutboundDeliveredMessageConsumptionRequest,
                     ProviderRefundRequest, ProviderUsageMetadata,
                     TelephonyVoiceMinuteConsumptionRequest, UsageMetadata,
                     VoiceAIStartedMinuteConsumptionRequest,
                     WhatsAppMetaMarketingMessageConsumptionRequest)

_MAX_INT64 = 9_223_372_036_854_775_807
_INTEGER = re.compile(r"^\d+$")
_DECIMAL = re.compile(r"^(\d+)(?:\.(\d{1,3}))?$")


def confirmed_top_up(*, amount_minor: str, payment_event_id: str, paid_total_minor: str) -> ConfirmedTopUp:
    return {"amount_minor": amount_minor, "payment_event_id": payment_event_id,
            "payment_status": PaymentStatus.CONFIRMED, "currency": Currency.SAR,
            "paid_total_minor": paid_total_minor}


def confirmed_refund(*, amount_minor: str, payment_event_id: str, reason: str) -> ProviderRefundRequest:
    return {"amount_minor": amount_minor, "payment_event_id": payment_event_id, "reason": reason,
            "refund_status": RefundStatus.CONFIRMED, "currency": Currency.SAR,
            "refunded_total_minor": amount_minor}


def feature_budget(*, metric: BudgetMetric, limit: str, action: BudgetAction,
                   warning_bps: int = 8000, sensitive: bool = False) -> BudgetRequest:
    return {"metric": metric, "period": BudgetPeriod.SUBSCRIPTION_MONTH.value,
            "limit": limit, "warning_bps": warning_bps, "action": action, "sensitive": sensitive}


def _event(source_event_id: str, occurred_at: str) -> None:
    if not isinstance(source_event_id, str) or not 1 <= len(source_event_id) <= 128:
        raise ValueError("source_event_id must contain 1 to 128 characters")
    if not isinstance(occurred_at, str):
        raise ValueError("occurred_at must be an ISO-8601 timestamp with a timezone")
    try:
        value = datetime.fromisoformat(occurred_at.replace("Z", "+00:00"))
    except ValueError as exc:
        raise ValueError("occurred_at must be an ISO-8601 timestamp with a timezone") from exc
    if value.tzinfo is None:
        raise ValueError("occurred_at must include a timezone")


def _quantity(value: str) -> None:
    match = _DECIMAL.fullmatch(value) if isinstance(value, str) else None
    if not match:
        raise ValueError("quantity must be a positive decimal string with at most 3 decimal places")
    fraction = (match.group(2) or "").ljust(3, "0")
    scaled = int(match.group(1)) * 1_000 + int(fraction or "0")
    if not 0 < scaled <= _MAX_INT64:
        raise ValueError("quantity must be positive and fit the supported exact range")


def _integer(value: str, field: str, *, allow_zero: bool = False) -> None:
    if not isinstance(value, str) or not _INTEGER.fullmatch(value):
        raise ValueError(f"{field} must be a non-negative integer string")
    parsed = int(value)
    if parsed > _MAX_INT64 or (parsed == 0 and not allow_zero):
        qualifier = "non-negative" if allow_zero else "positive"
        raise ValueError(f"{field} must be {qualifier} and fit the supported exact range")


def _validate_metadata(metadata: UsageMetadata | ProviderUsageMetadata | None) -> None:
    if metadata is None:
        return
    channel = metadata.get("channel")
    if channel is not None and str(channel) not in {item.value for item in Channel}:
        raise ValueError("metadata.channel is not a supported channel")
    attributes = metadata.get("attributes", {})
    if not isinstance(attributes, dict) or len(attributes) > 32:
        raise ValueError("metadata.attributes must be an object with at most 32 entries")
    for key, value in attributes.items():
        if not isinstance(key, str) or len(key) > 64 or value is not None and not isinstance(value, (str, int, float, bool)):
            raise ValueError("metadata.attributes contains an invalid key or scalar value")
        if value is not None and len(str(value)) > 512:
            raise ValueError("metadata attribute values must be at most 512 characters")
        if isinstance(value, float) and not math.isfinite(value):
            raise ValueError("metadata attribute numbers must be finite")


def _count_usage(feature_code: FeatureCode, *, source_event_id: str, occurred_at: str,
                 quantity: str, metadata: UsageMetadata | ProviderUsageMetadata | None) -> dict[str, Any]:
    _event(source_event_id, occurred_at)
    _quantity(quantity)
    _validate_metadata(metadata)
    request: dict[str, Any] = {"source_event_id": source_event_id, "occurred_at": occurred_at,
                               "feature_code": feature_code, "quantity": quantity}
    if metadata is not None:
        request["metadata"] = dict(metadata)
    return request


def _provider_metadata(*, provider: str, provider_event_id: str,
                       metadata: UsageMetadata | None = None) -> ProviderUsageMetadata:
    _validate_metadata(metadata)
    if not isinstance(provider, str) or not provider.strip():
        raise ValueError("provider is required for provider-priced usage")
    if not isinstance(provider_event_id, str) or not provider_event_id.strip():
        raise ValueError("provider_event_id is required for provider-priced usage")
    result = dict(metadata or {})
    result["provider"] = provider.strip()
    result["provider_event_id"] = provider_event_id.strip()
    return cast(ProviderUsageMetadata, result)


def conversation_24h(*, source_event_id: str, occurred_at: str, quantity: str = "1",
                     metadata: UsageMetadata | None = None) -> Conversation24HConsumptionRequest:
    """Build ``conversation_24h``. One quantity is one fixed 24-hour conversation window."""
    return cast(Conversation24HConsumptionRequest, _count_usage(
        FeatureCode.CONVERSATION_24H, source_event_id=source_event_id,
        occurred_at=occurred_at, quantity=quantity, metadata=metadata))


def outbound_delivered_message(*, source_event_id: str, occurred_at: str, quantity: str = "1",
                               metadata: UsageMetadata | None = None) -> OutboundDeliveredMessageConsumptionRequest:
    """Build ``outbound_delivered_message``; provider/Meta fees are separate charges."""
    return cast(OutboundDeliveredMessageConsumptionRequest, _count_usage(
        FeatureCode.OUTBOUND_DELIVERED_MESSAGE, source_event_id=source_event_id,
        occurred_at=occurred_at, quantity=quantity, metadata=metadata))


def ai_assist_action_over_allowance(*, source_event_id: str, occurred_at: str, quantity: str = "1",
                                    metadata: UsageMetadata | None = None) -> AIAssistActionOverAllowanceConsumptionRequest:
    """Build ``ai_assist_action_over_allowance`` after the included allowance is exhausted."""
    return cast(AIAssistActionOverAllowanceConsumptionRequest, _count_usage(
        FeatureCode.AI_ASSIST_ACTION_OVER_ALLOWANCE, source_event_id=source_event_id,
        occurred_at=occurred_at, quantity=quantity, metadata=metadata))


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
        occurred_at=occurred_at, quantity=billable_minutes, metadata=provider_metadata))


def inbound_voice_minute(*, source_event_id: str, occurred_at: str, provider: str,
                         provider_event_id: str, billable_minutes: str = "1",
                         metadata: UsageMetadata | None = None) -> InboundVoiceMinuteConsumptionRequest:
    """Build provider-normalized inbound minutes; the default catalog treatment is zero-rated."""
    provider_metadata = _provider_metadata(provider=provider, provider_event_id=provider_event_id, metadata=metadata)
    return cast(InboundVoiceMinuteConsumptionRequest, _count_usage(
        FeatureCode.INBOUND_VOICE_MINUTE, source_event_id=source_event_id,
        occurred_at=occurred_at, quantity=billable_minutes, metadata=provider_metadata))


def other_provider_charge(*, source_event_id: str, occurred_at: str, provider: str,
                          provider_event_id: str, provider_amount_minor: str,
                          metadata: UsageMetadata | None = None) -> OtherProviderChargeConsumptionRequest:
    """Build an exact pass-through provider settlement amount in halala; zero is valid."""
    _event(source_event_id, occurred_at)
    _integer(provider_amount_minor, "provider_amount_minor", allow_zero=True)
    provider_metadata = _provider_metadata(provider=provider, provider_event_id=provider_event_id, metadata=metadata)
    return {"source_event_id": source_event_id, "occurred_at": occurred_at,
            "feature_code": "other_provider_charge", "provider_amount_minor": provider_amount_minor,
            "metadata": provider_metadata}
