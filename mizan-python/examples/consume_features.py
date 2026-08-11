"""Compile-checked examples for every Mizan consumption feature code.

These functions are examples only; call them from a trusted backend with a real
client, durable source event IDs, and idempotency keys stored with those events.
"""
from mizan import (
    Channel,
    Conversation24HConsumptionRequest,
    MizanClient,
    conversation_24h,
)


def queueable_conversation(source_event_id: str, occurred_at: str,
                           conversation_id: str) -> Conversation24HConsumptionRequest:
    """Build JSON-safe usage for an outbox without sending it immediately."""
    return conversation_24h(
        source_event_id=source_event_id,
        occurred_at=occurred_at,
        conversation_id=conversation_id,
        channel=Channel.WHATSAPP,
        # Report activity; Mizan decides whether a new 24-hour window opens.
    )


def consume_every_feature(client: MizanClient, business_id: str, occurred_at: str) -> None:
    """Show the distinct canonical method signature for every feature code."""
    client.consume_conversation_24h(
        business_id, source_event_id="conversation-1", occurred_at=occurred_at,
        conversation_id="conversation-1", channel=Channel.WHATSAPP,
        idempotency_key="consume:conversation-1",
    )
    client.consume_outbound_delivered_message(
        business_id, source_event_id="message-1", occurred_at=occurred_at,
        idempotency_key="consume:message-1",
    )
    # Report every action; Mizan returns included-versus-billable allowance facts.
    client.consume_ai_assist_action(
        business_id, source_event_id="assist-action-1", occurred_at=occurred_at,
        idempotency_key="consume:assist-action-1",
    )
    client.consume_voice_ai_started_minute(
        business_id, source_event_id="voice-ai-1", occurred_at=occurred_at,
        duration_seconds="61",  # raw duration; Mizan charges two started minutes
        idempotency_key="consume:voice-ai-1",
    )
    client.consume_ai_reply_handling(
        business_id, source_event_id="reply-1", occurred_at=occurred_at,
        idempotency_key="consume:reply-1",
    )
    client.consume_whatsapp_meta_marketing_message(
        business_id, source_event_id="meta-1", occurred_at=occurred_at,
        provider_event_id="wamid.meta-1",  # provider is fixed to Meta
        idempotency_key="consume:meta-1",
    )
    client.consume_telephony_voice_minute(
        business_id, source_event_id="outbound-call-1", occurred_at=occurred_at,
        provider="Twilio", provider_event_id="CA1", billable_minutes="1.5",
        metadata={"raw_quantity": "83", "billable_quantity": "1.5"},
        idempotency_key="consume:outbound-call-1",
    )
    client.consume_inbound_voice_minute(
        business_id, source_event_id="inbound-call-1", occurred_at=occurred_at,
        provider="Carrier", provider_event_id="IN1",
        idempotency_key="consume:inbound-call-1",
    )
    client.consume_other_provider_charge(
        business_id, source_event_id="provider-fee-1", occurred_at=occurred_at,
        provider="Carrier", provider_event_id="INV1", provider_amount_minor="337",
        provider_invoice_id="INV-2026-08", original_amount_minor="337",
        original_currency="SAR", tariff_version="carrier-v4",
        idempotency_key="consume:provider-fee-1",
    )
