// Command consume_features contains compile-checked examples for every feature
// contract. Integrate these calls into a trusted backend; never ship the Mizan
// token to a browser or mobile application.
package main

import (
	"context"
	"time"

	mizan "github.com/alshell7/mizan-client-sdks/mizan-go"
)

func consumeEveryFeature(ctx context.Context, client *mizan.Client, businessID string, occurredAt time.Time) error {
	if _, err := client.ConsumeConversation24H(ctx, businessID, mizan.Conversation24HUsage{
		SourceEventID: "conversation-1", OccurredAt: occurredAt, // Quantity defaults to one.
	}, "consume:conversation-1"); err != nil {
		return err
	}
	if _, err := client.ConsumeOutboundDeliveredMessage(ctx, businessID, mizan.OutboundDeliveredMessageUsage{
		SourceEventID: "message-1", OccurredAt: occurredAt,
	}, "consume:message-1"); err != nil {
		return err
	}
	if _, err := client.ConsumeAIAssistActionOverAllowance(ctx, businessID, mizan.AIAssistActionOverAllowanceUsage{
		SourceEventID: "assist-overage-1", OccurredAt: occurredAt,
	}, "consume:assist-overage-1"); err != nil {
		return err
	}
	if _, err := client.ConsumeVoiceAIStartedMinute(ctx, businessID, mizan.VoiceAIStartedMinuteUsage{
		SourceEventID: "voice-ai-1", OccurredAt: occurredAt, DurationSeconds: "61", // Mizan rounds to two started minutes.
	}, "consume:voice-ai-1"); err != nil {
		return err
	}
	if _, err := client.ConsumeAIReplyHandling(ctx, businessID, mizan.AIReplyHandlingUsage{
		SourceEventID: "reply-1", OccurredAt: occurredAt,
	}, "consume:reply-1"); err != nil {
		return err
	}
	if _, err := client.ConsumeWhatsAppMetaMarketingMessage(ctx, businessID, mizan.WhatsAppMetaMarketingMessageUsage{
		SourceEventID: "meta-1", OccurredAt: occurredAt, ProviderEventID: "wamid.meta-1", // Provider is fixed to Meta.
	}, "consume:meta-1"); err != nil {
		return err
	}
	if _, err := client.ConsumeTelephonyVoiceMinute(ctx, businessID, mizan.TelephonyVoiceMinuteUsage{
		SourceEventID: "outbound-call-1", OccurredAt: occurredAt, Provider: "Twilio", ProviderEventID: "CA1",
		BillableMinutes: "1.5", Metadata: &mizan.UsageMetadata{RawQuantity: "83", BillableQuantity: "1.5"},
	}, "consume:outbound-call-1"); err != nil {
		return err
	}
	if _, err := client.ConsumeInboundVoiceMinute(ctx, businessID, mizan.InboundVoiceMinuteUsage{
		SourceEventID: "inbound-call-1", OccurredAt: occurredAt, Provider: "Carrier", ProviderEventID: "IN1",
	}, "consume:inbound-call-1"); err != nil {
		return err
	}
	if _, err := client.ConsumeOtherProviderCharge(ctx, businessID, mizan.OtherProviderChargeUsage{
		SourceEventID: "provider-fee-1", OccurredAt: occurredAt, Provider: "Carrier", ProviderEventID: "INV1",
		ProviderAmountMinor: "337", Metadata: &mizan.UsageMetadata{ProviderInvoiceID: "INV-2026-08", TariffVersion: "carrier-v4"},
	}, "consume:provider-fee-1"); err != nil {
		return err
	}
	return nil
}

func main() {}
