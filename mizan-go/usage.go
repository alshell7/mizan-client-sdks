package mizan

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/big"
	"regexp"
	"strings"
	"time"
)

// Conversation24HUsage is the contract for conversation_24h. Quantity is the
// number of fixed 24-hour conversation windows and defaults to one when empty.
type Conversation24HUsage struct {
	SourceEventID string
	OccurredAt    time.Time
	Quantity      ExactAmount
	Metadata      *UsageMetadata
}

// OutboundDeliveredMessageUsage is the contract for
// outbound_delivered_message. It records product delivery only; any Meta or
// carrier fee is a separate provider feature or an atomic second component.
type OutboundDeliveredMessageUsage struct {
	SourceEventID string
	OccurredAt    time.Time
	Quantity      ExactAmount
	Metadata      *UsageMetadata
}

// AIAssistActionOverAllowanceUsage is the contract for
// ai_assist_action_over_allowance. Call it only after the product has established
// that the plan's included allowance is exhausted. Quantity defaults to one.
type AIAssistActionOverAllowanceUsage struct {
	SourceEventID string
	OccurredAt    time.Time
	Quantity      ExactAmount
	Metadata      *UsageMetadata
}

// AIReplyHandlingUsage is the contract for ai_reply_handling. The default
// catalog records it as included/zero-charge usage. Quantity defaults to one.
type AIReplyHandlingUsage struct {
	SourceEventID string
	OccurredAt    time.Time
	Quantity      ExactAmount
	Metadata      *UsageMetadata
}

// VoiceAIStartedMinuteUsage is the contract for voice_ai_started_minute.
// DurationSeconds is raw positive seconds; Mizan calculates ceil(seconds/60).
type VoiceAIStartedMinuteUsage struct {
	SourceEventID   string
	OccurredAt      time.Time
	DurationSeconds ExactAmount
	Metadata        *UsageMetadata
}

// WhatsAppMetaMarketingMessageUsage is the contract for
// whatsapp_meta_marketing_msg. Provider is fixed to Meta, ProviderEventID is
// mandatory, and Quantity defaults to one.
type WhatsAppMetaMarketingMessageUsage struct {
	SourceEventID   string
	OccurredAt      time.Time
	ProviderEventID string
	Quantity        ExactAmount
	Metadata        *UsageMetadata
}

// TelephonyVoiceMinuteUsage is the contract for telephony_voice_minute.
// BillableMinutes must be the provider-normalized tariff quantity, not raw call
// seconds. Provider and ProviderEventID are mandatory. It defaults to one minute.
type TelephonyVoiceMinuteUsage struct {
	SourceEventID   string
	OccurredAt      time.Time
	Provider        string
	ProviderEventID string
	BillableMinutes ExactAmount
	Metadata        *UsageMetadata
}

// InboundVoiceMinuteUsage is the contract for inbound_voice_minute. It carries
// provider-normalized minutes and mandatory provider attribution. The default
// catalog treatment is zero-rated, but the event remains auditable.
type InboundVoiceMinuteUsage struct {
	SourceEventID   string
	OccurredAt      time.Time
	Provider        string
	ProviderEventID string
	BillableMinutes ExactAmount
	Metadata        *UsageMetadata
}

// OtherProviderChargeUsage is the contract for other_provider_charge.
// ProviderAmountMinor is an exact provider-confirmed settlement amount in halala;
// zero is allowed. Never derive this value with float64.
type OtherProviderChargeUsage struct {
	SourceEventID       string
	OccurredAt          time.Time
	Provider            string
	ProviderEventID     string
	ProviderAmountMinor ExactAmount
	Metadata            *UsageMetadata
}

var (
	// quantityPattern accepts canonical positive decimals with at most milli precision.
	quantityPattern = regexp.MustCompile(`^(\d+)(?:\.(\d{1,3}))?$`)
	// integerPattern is used for raw seconds and provider halala amounts without coercion.
	integerPattern = regexp.MustCompile(`^\d+$`)
	// maxInt64Exact mirrors the Worker's persisted exact-value boundary.
	maxInt64Exact = big.NewInt(9_223_372_036_854_775_807)
)

func quantityOrOne(value ExactAmount) ExactAmount {
	// Only quantity-based contracts define an omitted value as exactly one.
	if value == "" {
		return "1"
	}
	return value
}

func validateUsageEvent(sourceEventID string, occurredAt time.Time) error {
	if len(sourceEventID) < 1 || len(sourceEventID) > 128 {
		return errors.New("mizan: source event ID must contain 1 to 128 characters")
	}
	if occurredAt.IsZero() {
		return errors.New("mizan: occurred at is required")
	}
	return nil
}

func validateQuantity(value ExactAmount) error {
	match := quantityPattern.FindStringSubmatch(string(value))
	if match == nil {
		return errors.New("mizan: quantity must be a positive decimal string with at most 3 decimal places")
	}
	whole, _ := new(big.Int).SetString(match[1], 10)
	// Scale through big.Int so validation never passes through float64.
	scaled := new(big.Int).Mul(whole, big.NewInt(1000))
	if match[2] != "" {
		fraction, _ := new(big.Int).SetString(match[2]+strings.Repeat("0", 3-len(match[2])), 10)
		scaled.Add(scaled, fraction)
	}
	if scaled.Sign() <= 0 || scaled.Cmp(maxInt64Exact) > 0 {
		return errors.New("mizan: quantity must be positive and fit the supported exact range")
	}
	return nil
}

func validateExactInteger(value ExactAmount, field string, allowZero bool) error {
	if !integerPattern.MatchString(string(value)) {
		return fmt.Errorf("mizan: %s must be a non-negative integer string", field)
	}
	parsed, _ := new(big.Int).SetString(string(value), 10)
	// Zero is valid only for contracts such as an exact pass-through provider amount.
	if parsed.Cmp(maxInt64Exact) > 0 || (!allowZero && parsed.Sign() == 0) {
		qualifier := "positive"
		if allowZero {
			qualifier = "non-negative"
		}
		return fmt.Errorf("mizan: %s must be %s and fit the supported exact range", field, qualifier)
	}
	return nil
}

func validateMetadata(metadata *UsageMetadata) error {
	if metadata == nil {
		return nil
	}
	if metadata.Channel != "" {
		switch metadata.Channel {
		case ChannelWhatsApp, ChannelInstagram, ChannelFacebook, ChannelTikTok, ChannelTelephony, ChannelWebchat:
		default:
			return errors.New("mizan: metadata channel is not supported")
		}
	}
	if len(metadata.Attributes) > 32 {
		return errors.New("mizan: metadata attributes support at most 32 entries")
	}
	for key, value := range metadata.Attributes {
		// Custom attributes are bounded reconciliation facts, not arbitrary provider payloads.
		if len(key) > 64 {
			return errors.New("mizan: metadata attribute keys must be at most 64 characters")
		}
		switch typed := value.(type) {
		case nil, string, bool, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		case float32:
			if math.IsNaN(float64(typed)) || math.IsInf(float64(typed), 0) {
				return errors.New("mizan: metadata attribute numbers must be finite")
			}
		case float64:
			if math.IsNaN(typed) || math.IsInf(typed, 0) {
				return errors.New("mizan: metadata attribute numbers must be finite")
			}
		default:
			return errors.New("mizan: metadata attribute values must be scalar")
		}
		if value != nil && len(fmt.Sprint(value)) > 512 {
			return errors.New("mizan: metadata attribute values must be at most 512 characters")
		}
	}
	return nil
}

func providerMetadata(provider, eventID string, source *UsageMetadata) (*UsageMetadata, error) {
	if strings.TrimSpace(provider) == "" || strings.TrimSpace(eventID) == "" {
		return nil, errors.New("mizan: provider and provider event ID are required for provider-priced usage")
	}
	if err := validateMetadata(source); err != nil {
		return nil, err
	}
	metadata := UsageMetadata{}
	if source != nil {
		// Copy the value so canonical provider fields do not mutate caller-owned metadata.
		metadata = *source
	}
	// Required method arguments take precedence over conflicting optional metadata.
	metadata.Provider = strings.TrimSpace(provider)
	metadata.ProviderEventID = strings.TrimSpace(eventID)
	return &metadata, nil
}

func countRequest(feature FeatureCode, sourceEventID string, occurredAt time.Time, quantity ExactAmount,
	metadata *UsageMetadata) (ConsumptionRequest, error) {
	if err := validateUsageEvent(sourceEventID, occurredAt); err != nil {
		return ConsumptionRequest{}, err
	}
	quantity = quantityOrOne(quantity)
	if err := validateQuantity(quantity); err != nil {
		return ConsumptionRequest{}, err
	}
	if err := validateMetadata(metadata); err != nil {
		return ConsumptionRequest{}, err
	}
	// Construct wire input only after every local fact has passed validation.
	return ConsumptionRequest{SourceEventID: sourceEventID, OccurredAt: occurredAt,
		FeatureCode: feature, Quantity: string(quantity), Metadata: metadata}, nil
}

// ConsumeConversation24H records one or more fixed 24-hour conversation windows.
func (c *Client) ConsumeConversation24H(ctx context.Context, businessID string, in Conversation24HUsage, idempotencyKey string) (Response, error) {
	request, err := countRequest(FeatureConversation24H, in.SourceEventID, in.OccurredAt, in.Quantity, in.Metadata)
	if err != nil {
		return nil, err
	}
	return c.Consume(ctx, businessID, request, idempotencyKey)
}

// ConsumeOutboundDeliveredMessage records delivered product messages; provider fees are separate.
func (c *Client) ConsumeOutboundDeliveredMessage(ctx context.Context, businessID string, in OutboundDeliveredMessageUsage, idempotencyKey string) (Response, error) {
	request, err := countRequest(FeatureOutboundDeliveredMessage, in.SourceEventID, in.OccurredAt, in.Quantity, in.Metadata)
	if err != nil {
		return nil, err
	}
	return c.Consume(ctx, businessID, request, idempotencyKey)
}

// ConsumeAIAssistActionOverAllowance records actions after the caller establishes allowance exhaustion.
func (c *Client) ConsumeAIAssistActionOverAllowance(ctx context.Context, businessID string, in AIAssistActionOverAllowanceUsage, idempotencyKey string) (Response, error) {
	request, err := countRequest(FeatureAIAssistOverAllowance, in.SourceEventID, in.OccurredAt, in.Quantity, in.Metadata)
	if err != nil {
		return nil, err
	}
	return c.Consume(ctx, businessID, request, idempotencyKey)
}

// ConsumeAIReplyHandling records included handling for audit and fair-use visibility.
func (c *Client) ConsumeAIReplyHandling(ctx context.Context, businessID string, in AIReplyHandlingUsage, idempotencyKey string) (Response, error) {
	request, err := countRequest(FeatureAIReplyHandling, in.SourceEventID, in.OccurredAt, in.Quantity, in.Metadata)
	if err != nil {
		return nil, err
	}
	return c.Consume(ctx, businessID, request, idempotencyKey)
}

// ConsumeVoiceAIStartedMinute sends raw seconds; Mizan rounds up to started minutes.
func (c *Client) ConsumeVoiceAIStartedMinute(ctx context.Context, businessID string, in VoiceAIStartedMinuteUsage, idempotencyKey string) (Response, error) {
	if err := validateUsageEvent(in.SourceEventID, in.OccurredAt); err != nil {
		return nil, err
	}
	if err := validateExactInteger(in.DurationSeconds, "duration seconds", false); err != nil {
		return nil, err
	}
	if err := validateMetadata(in.Metadata); err != nil {
		return nil, err
	}
	return c.Consume(ctx, businessID, ConsumptionRequest{SourceEventID: in.SourceEventID, OccurredAt: in.OccurredAt,
		FeatureCode: FeatureVoiceAIStartedMinute, DurationSeconds: in.DurationSeconds, Metadata: in.Metadata}, idempotencyKey)
}

// ConsumeWhatsAppMetaMarketingMessage charges Meta's tariff with provider-event deduplication.
func (c *Client) ConsumeWhatsAppMetaMarketingMessage(ctx context.Context, businessID string, in WhatsAppMetaMarketingMessageUsage, idempotencyKey string) (Response, error) {
	metadata, err := providerMetadata("Meta", in.ProviderEventID, in.Metadata)
	if err != nil {
		return nil, err
	}
	request, err := countRequest(FeatureWhatsAppMetaMarketingMessage, in.SourceEventID, in.OccurredAt, in.Quantity, metadata)
	if err != nil {
		return nil, err
	}
	return c.Consume(ctx, businessID, request, idempotencyKey)
}

// ConsumeTelephonyVoiceMinute sends provider-normalized billable minutes, not raw seconds.
func (c *Client) ConsumeTelephonyVoiceMinute(ctx context.Context, businessID string, in TelephonyVoiceMinuteUsage, idempotencyKey string) (Response, error) {
	metadata, err := providerMetadata(in.Provider, in.ProviderEventID, in.Metadata)
	if err != nil {
		return nil, err
	}
	request, err := countRequest(FeatureTelephonyVoiceMinute, in.SourceEventID, in.OccurredAt, in.BillableMinutes, metadata)
	if err != nil {
		return nil, err
	}
	return c.Consume(ctx, businessID, request, idempotencyKey)
}

// ConsumeInboundVoiceMinute records attributed inbound minutes even when the tariff is zero.
func (c *Client) ConsumeInboundVoiceMinute(ctx context.Context, businessID string, in InboundVoiceMinuteUsage, idempotencyKey string) (Response, error) {
	metadata, err := providerMetadata(in.Provider, in.ProviderEventID, in.Metadata)
	if err != nil {
		return nil, err
	}
	request, err := countRequest(FeatureInboundVoiceMinute, in.SourceEventID, in.OccurredAt, in.BillableMinutes, metadata)
	if err != nil {
		return nil, err
	}
	return c.Consume(ctx, businessID, request, idempotencyKey)
}

// ConsumeOtherProviderCharge debits an exact provider-confirmed pass-through amount in halala.
func (c *Client) ConsumeOtherProviderCharge(ctx context.Context, businessID string, in OtherProviderChargeUsage, idempotencyKey string) (Response, error) {
	if err := validateUsageEvent(in.SourceEventID, in.OccurredAt); err != nil {
		return nil, err
	}
	if err := validateExactInteger(in.ProviderAmountMinor, "provider amount minor", true); err != nil {
		return nil, err
	}
	metadata, err := providerMetadata(in.Provider, in.ProviderEventID, in.Metadata)
	if err != nil {
		return nil, err
	}
	return c.Consume(ctx, businessID, ConsumptionRequest{SourceEventID: in.SourceEventID, OccurredAt: in.OccurredAt,
		FeatureCode: FeatureOtherProviderCharge, ProviderAmountMinor: in.ProviderAmountMinor, Metadata: metadata}, idempotencyKey)
}

// Compatibility aliases use the same feature-specific contracts.
// ConsumeAIAssistOverAllowance is a compatibility alias for ConsumeAIAssistActionOverAllowance.
func (c *Client) ConsumeAIAssistOverAllowance(ctx context.Context, businessID string, in AIAssistActionOverAllowanceUsage, idempotencyKey string) (Response, error) {
	return c.ConsumeAIAssistActionOverAllowance(ctx, businessID, in, idempotencyKey)
}

// ConsumeVoiceAI is a compatibility alias for ConsumeVoiceAIStartedMinute.
func (c *Client) ConsumeVoiceAI(ctx context.Context, businessID string, in VoiceAIStartedMinuteUsage, idempotencyKey string) (Response, error) {
	return c.ConsumeVoiceAIStartedMinute(ctx, businessID, in, idempotencyKey)
}

// ConsumeTelephonyVoice is a compatibility alias for ConsumeTelephonyVoiceMinute.
func (c *Client) ConsumeTelephonyVoice(ctx context.Context, businessID string, in TelephonyVoiceMinuteUsage, idempotencyKey string) (Response, error) {
	return c.ConsumeTelephonyVoiceMinute(ctx, businessID, in, idempotencyKey)
}

// ConsumeInboundVoice is a compatibility alias for ConsumeInboundVoiceMinute.
func (c *Client) ConsumeInboundVoice(ctx context.Context, businessID string, in InboundVoiceMinuteUsage, idempotencyKey string) (Response, error) {
	return c.ConsumeInboundVoiceMinute(ctx, businessID, in, idempotencyKey)
}
