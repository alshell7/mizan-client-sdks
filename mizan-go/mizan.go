package mizan

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Version is the SDK version sent in the HTTP User-Agent header.
const Version = "1.7.0"

// ExactAmount is an exact base-10 integer string. Money uses halala and Azeer
// Units use milliunits; never construct these values through float64 arithmetic.
type ExactAmount string

// Response is the forward-compatible Mizan API envelope. Use DecodeData to
// convert its data member to a documented result type.
type Response map[string]any

// PlanID identifies an immutable public catalog plan template.
type PlanID string

const (
	// PlanStart is the entry public catalog template.
	PlanStart PlanID = "start"
	// PlanGrowth extends Start with growth-tier capabilities and limits.
	PlanGrowth PlanID = "growth"
	// PlanCommand is the highest public catalog template.
	PlanCommand PlanID = "command"
)

// BillingTerm controls the paid subscription period and catalog discount.
type BillingTerm string

const (
	// TermMonthly is one anchored subscription month with no term discount.
	TermMonthly BillingTerm = "monthly"
	// TermQuarterly is three paid months with monthly included-unit grants.
	TermQuarterly BillingTerm = "quarterly"
	// TermSemiAnnual is six paid months with monthly included-unit grants.
	TermSemiAnnual BillingTerm = "semi_annual"
	// TermAnnual is twelve paid months with all included units granted up front.
	TermAnnual BillingTerm = "annual"
)

// FeatureCode identifies a versioned metering and pricing contract.
type FeatureCode string

const (
	// FeatureConversation24H charges fixed conversation windows in Azeer Units.
	FeatureConversation24H FeatureCode = "conversation_24h"
	// FeatureOutboundDeliveredMessage records product delivery; provider fees are separate.
	FeatureOutboundDeliveredMessage FeatureCode = "outbound_delivered_message"
	// FeatureAIAssistOverAllowance is used only after included allowance is exhausted.
	FeatureAIAssistOverAllowance FeatureCode = "ai_assist_action_over_allowance"
	// FeatureVoiceAIStartedMinute accepts raw seconds and rounds up inside Mizan.
	FeatureVoiceAIStartedMinute FeatureCode = "voice_ai_started_minute"
	// FeatureAIReplyHandling is included/zero-charge in the default catalog.
	FeatureAIReplyHandling FeatureCode = "ai_reply_handling"
	// FeatureWhatsAppMetaMarketingMessage requires Meta provider-event attribution.
	FeatureWhatsAppMetaMarketingMessage FeatureCode = "whatsapp_meta_marketing_msg"
	// FeatureTelephonyVoiceMinute accepts provider-normalized outbound billable minutes.
	FeatureTelephonyVoiceMinute FeatureCode = "telephony_voice_minute"
	// FeatureInboundVoiceMinute is attributed and zero-rated by the default catalog.
	FeatureInboundVoiceMinute FeatureCode = "inbound_voice_minute"
	// FeatureOtherProviderCharge passes through an exact provider amount in halala.
	FeatureOtherProviderCharge FeatureCode = "other_provider_charge"
)

// Currency is an ISO currency accepted by the Mizan contract.
type Currency string

// CurrencySAR is Saudi riyal; all money amounts are integer halala strings.
const CurrencySAR Currency = "SAR"

// PaymentStatus is the trusted outcome of a uniquely identified payment event.
type PaymentStatus string

const (
	// PaymentConfirmed requires exact currency and total reconciliation.
	PaymentConfirmed PaymentStatus = "confirmed"
	// PaymentFailed is valid for renewal events and moves the account past due.
	PaymentFailed PaymentStatus = "failed"
)

// RefundStatus is the trusted outcome of a uniquely identified refund event.
type RefundStatus string

// RefundConfirmed records a trusted, completed provider refund.
const RefundConfirmed RefundStatus = "confirmed"

// BudgetMetric selects the exact value accumulated for a feature budget.
type BudgetMetric string

const (
	// BudgetAzeerUnitMillis accumulates platform-credit spend in milliunits.
	BudgetAzeerUnitMillis BudgetMetric = "azeer_unit_millis"
	// BudgetMoneyMinor accumulates provider-wallet spend in halala.
	BudgetMoneyMinor BudgetMetric = "money_minor"
	// BudgetQuantity accumulates normalized quantities in thousandths.
	BudgetQuantity BudgetMetric = "quantity"
)

// BudgetPeriod selects the lifecycle window in which a budget counter resets.
type BudgetPeriod string

// BudgetSubscriptionMonth is anchored to subscription activation, not calendar month start.
const BudgetSubscriptionMonth BudgetPeriod = "subscription_month"

// BudgetAction controls whether crossing a limit alerts or rejects usage.
type BudgetAction string

const (
	// BudgetAlert commits crossing usage and emits warning or breach notifications.
	BudgetAlert BudgetAction = "alert"
	// BudgetPause rejects the crossing request and keeps the feature paused.
	BudgetPause BudgetAction = "pause"
)

// Channel identifies application attribution stored with a usage decision.
type Channel string

const (
	ChannelWhatsApp  Channel = "whatsapp"
	ChannelInstagram Channel = "instagram"
	ChannelFacebook  Channel = "facebook"
	ChannelTikTok    Channel = "tiktok"
	ChannelTelephony Channel = "telephony"
	ChannelWebchat   Channel = "webchat"
)

// RecurringAddonCode identifies a catalog-backed subscription add-on.
type RecurringAddonCode string

const (
	// Fixed-price catalog add-ons.
	AddonWhatsApp011Landline RecurringAddonCode = "whatsapp_011_landline"
	AddonWhatsApp05Mobile    RecurringAddonCode = "whatsapp_05_mobile"
	AddonConcurrentCalls5    RecurringAddonCode = "concurrent_calls_5"
	AddonConcurrentCalls10   RecurringAddonCode = "concurrent_calls_10"
	AddonConcurrentCalls20   RecurringAddonCode = "concurrent_calls_20"
	AddonAutoDialer          RecurringAddonCode = "auto_dialer"
	AddonCSATStart           RecurringAddonCode = "csat_start"
	// AddonInstagramAdditionalAccounts uses per-unit tier pricing.
	AddonInstagramAdditionalAccounts RecurringAddonCode = "instagram_additional_accounts"
	// Quote-priced add-ons require approved quote ID and monthly price evidence.
	AddonWhatsApp9200               RecurringAddonCode = "whatsapp_9200"
	AddonTollFree800                RecurringAddonCode = "toll_free_800"
	AddonInternationalNumber        RecurringAddonCode = "international_number"
	AddonOutboundMinutes500         RecurringAddonCode = "outbound_minute_bundle_500"
	AddonOutboundMinutes1000        RecurringAddonCode = "outbound_minute_bundle_1000"
	AddonVoiceBroadcast             RecurringAddonCode = "voice_broadcast"
	AddonExtendedRecordingRetention RecurringAddonCode = "recording_retention_extended"
)

// ErrorCode is a stable machine-readable API decision code.
type ErrorCode string

const (
	// Client/request validation and authentication errors.
	ErrCodeInvalidRequest ErrorCode = "INVALID_REQUEST"
	ErrCodeUnauthorized   ErrorCode = "UNAUTHORIZED"
	ErrCodeForbidden      ErrorCode = "FORBIDDEN"
	ErrCodeNotFound       ErrorCode = "NOT_FOUND"
	// Account, subscription, feature, and funding decisions.
	ErrCodeAccountInactive             ErrorCode = "ACCOUNT_INACTIVE"
	ErrCodeSubscriptionInactive        ErrorCode = "SUBSCRIPTION_INACTIVE"
	ErrCodeFeatureDisabled             ErrorCode = "FEATURE_DISABLED"
	ErrCodeFeaturePausedBudget         ErrorCode = "FEATURE_PAUSED_BUDGET"
	ErrCodeFeaturePausedManual         ErrorCode = "FEATURE_PAUSED_MANUAL"
	ErrCodeInsufficientAzeerUnits      ErrorCode = "INSUFFICIENT_AZEER_UNITS"
	ErrCodeInsufficientProviderBalance ErrorCode = "INSUFFICIENT_PROVIDER_BALANCE"
	// Payment and replay conflicts require reconciliation rather than blind retries.
	ErrCodePaymentAmountMismatch ErrorCode = "PAYMENT_AMOUNT_MISMATCH"
	ErrCodeIdempotencyKeyReused  ErrorCode = "IDEMPOTENCY_KEY_REUSED"
	// Retryable infrastructure failures preserve the original body and key.
	ErrCodeInternalRetryable                ErrorCode = "INTERNAL_RETRYABLE"
	ErrCodeDependencyTemporarilyUnavailable ErrorCode = "DEPENDENCY_TEMPORARILY_UNAVAILABLE"
	ErrCodeDuplicatePaymentEvent            ErrorCode = "DUPLICATE_PAYMENT_EVENT"
	ErrCodeDuplicateProviderEvent           ErrorCode = "DUPLICATE_PROVIDER_EVENT"
	ErrCodeDuplicateSourceEvent             ErrorCode = "DUPLICATE_SOURCE_EVENT"
	// Remaining commercial, timing, configuration, and invariant decisions.
	ErrCodeEarlyRenewalEvent            ErrorCode = "EARLY_RENEWAL_EVENT"
	ErrCodeInvalidQuantity              ErrorCode = "INVALID_QUANTITY"
	ErrCodeInvariantViolation           ErrorCode = "INVARIANT_VIOLATION"
	ErrCodeMisconfigured                ErrorCode = "MISCONFIGURED"
	ErrCodeQuoteRequired                ErrorCode = "QUOTE_REQUIRED"
	ErrCodeQuoteVerificationUnavailable ErrorCode = "QUOTE_VERIFICATION_UNAVAILABLE"
	ErrCodeRequestTimestampOutOfRange   ErrorCode = "REQUEST_TIMESTAMP_OUT_OF_RANGE"
	ErrCodeSensitiveReserveReached      ErrorCode = "SENSITIVE_RESERVE_REACHED"
	ErrCodeStalePlanVersion             ErrorCode = "STALE_PLAN_VERSION"
	ErrCodeSubscriptionChangePending    ErrorCode = "SUBSCRIPTION_CHANGE_PENDING"
)

// DomainError is a sentinel matched by errors.Is against an APIError.
type DomainError ErrorCode

func (e DomainError) Error() string { return "mizan: " + string(e) }

// Known domain-error sentinels support errors.Is without discarding APIError details.
var (
	ErrInvalidRequest              = DomainError(ErrCodeInvalidRequest)
	ErrUnauthorized                = DomainError(ErrCodeUnauthorized)
	ErrForbidden                   = DomainError(ErrCodeForbidden)
	ErrNotFound                    = DomainError(ErrCodeNotFound)
	ErrAccountInactive             = DomainError(ErrCodeAccountInactive)
	ErrSubscriptionInactive        = DomainError(ErrCodeSubscriptionInactive)
	ErrFeatureDisabled             = DomainError(ErrCodeFeatureDisabled)
	ErrFeaturePausedBudget         = DomainError(ErrCodeFeaturePausedBudget)
	ErrFeaturePausedManual         = DomainError(ErrCodeFeaturePausedManual)
	ErrInsufficientAzeerUnits      = DomainError(ErrCodeInsufficientAzeerUnits)
	ErrInsufficientProviderBalance = DomainError(ErrCodeInsufficientProviderBalance)
	ErrPaymentAmountMismatch       = DomainError(ErrCodePaymentAmountMismatch)
	ErrIdempotencyKeyReused        = DomainError(ErrCodeIdempotencyKeyReused)
	ErrInternalRetryable           = DomainError(ErrCodeInternalRetryable)
)

// Balance contains post-transaction exact balances for both billing rails.
type Balance struct {
	// AzeerUnitMillis is the remaining platform-credit balance in thousandths of a unit.
	AzeerUnitMillis ExactAmount `json:"azeer_unit_millis"`
	// ProviderBalanceMinor is the remaining prepaid provider wallet in halala.
	ProviderBalanceMinor ExactAmount `json:"provider_balance_minor"`
}

// InvoiceLine is one immutable, independently taxed invoice line.
type InvoiceLine struct {
	Code               string      `json:"code"`
	Quantity           ExactAmount `json:"quantity"`
	NetMinor           ExactAmount `json:"net_minor"`
	DiscountMinor      ExactAmount `json:"discount_minor"`
	AfterDiscountMinor ExactAmount `json:"after_discount_minor"`
	VATMinor           ExactAmount `json:"vat_minor"`
	TotalMinor         ExactAmount `json:"total_minor"`
}

// Invoice contains exact halala totals calculated by the authoritative Worker.
type Invoice struct {
	ID                 string        `json:"id"`
	Currency           Currency      `json:"currency"`
	EligibleGrossMinor ExactAmount   `json:"eligible_gross_minor"`
	DiscountMinor      ExactAmount   `json:"discount_minor"`
	SubtotalMinor      ExactAmount   `json:"subtotal_minor"`
	VATMinor           ExactAmount   `json:"vat_minor"`
	TotalMinor         ExactAmount   `json:"total_minor"`
	Lines              []InvoiceLine `json:"lines"`
}

// ActivationResult describes the committed first subscription period and funding grant.
type ActivationResult struct {
	SubscriptionID            string      `json:"subscription_id"`
	CycleID                   string      `json:"cycle_id"`
	Status                    string      `json:"status"`
	CurrentPeriodStart        time.Time   `json:"current_period_start"`
	CurrentPeriodEnd          time.Time   `json:"current_period_end"`
	IncludedUnitMillisGranted ExactAmount `json:"included_unit_millis_granted"`
	Invoice                   Invoice     `json:"invoice"`
	LedgerEntryID             string      `json:"ledger_entry_id"`
	Balances                  Balance     `json:"balances"`
}

// ConsumptionResult is the authoritative all-or-nothing decision for one source event.
type ConsumptionResult struct {
	// Accepted is true only when every component and related record committed atomically.
	Accepted bool `json:"accepted"`
	// Code is the authoritative stable decision; do not parse an error message instead.
	Code          string `json:"code"`
	SourceEventID string `json:"source_event_id"`
	// LedgerEntryID links the decision to immutable financial history.
	LedgerEntryID string `json:"ledger_entry_id"`
	// BusinessSequence orders downstream ledger replication for this business.
	BusinessSequence int64 `json:"business_sequence"`
	// Charges contains normalized exact per-component rail amounts.
	Charges []map[string]any `json:"charges"`
	// AllocationsByFeature identifies Azeer credit lots consumed by each component.
	AllocationsByFeature []map[string]any       `json:"allocations_by_feature"`
	Totals               map[string]ExactAmount `json:"totals"`
	// Balances are post-transaction values, not a preflight projection.
	Balances Balance        `json:"balances"`
	Details  map[string]any `json:"details"`
}

// EntitlementResult reports whether an active subscription snapshot includes a capability.
type EntitlementResult struct {
	Capability Capability     `json:"capability"`
	Enabled    bool           `json:"enabled"`
	PlanID     string         `json:"plan_id"`
	FairUse    map[string]any `json:"fair_use"`
}

// LedgerResult is a business-sequence page of immutable financial history.
type LedgerResult struct {
	Entries           []map[string]any `json:"entries"`
	NextAfterSequence *int64           `json:"next_after_sequence"`
}

// BillingSummaryResult is the current account view for billing and support interfaces.
type BillingSummaryResult struct {
	BusinessID   string           `json:"business_id"`
	Account      map[string]any   `json:"account"`
	Subscription map[string]any   `json:"subscription"`
	Balances     Balance          `json:"balances"`
	CreditLots   []map[string]any `json:"credit_lots"`
	Budgets      []map[string]any `json:"budgets"`
	Pauses       []map[string]any `json:"pauses"`
	Replication  map[string]any   `json:"replication"`
}

// DecodeData converts an API response's data field into a documented result type
// without converting exact string amounts to floating point.
func DecodeData[T any](response Response) (T, error) {
	var result T
	data, ok := response["data"]
	if !ok {
		return result, errors.New("mizan: response has no data field")
	}
	raw, err := json.Marshal(data)
	if err != nil {
		return result, fmt.Errorf("mizan: encode response data: %w", err)
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return result, fmt.Errorf("mizan: decode response data: %w", err)
	}
	return result, nil
}

// RecurringAddon selects an add-on and, for quote pricing, its approved exact price.
type RecurringAddon struct {
	Code                 RecurringAddonCode `json:"code"`
	Quantity             ExactAmount        `json:"quantity,omitempty"`
	ApprovedQuoteID      string             `json:"approved_quote_id,omitempty"`
	ApprovedMonthlyMinor ExactAmount        `json:"approved_monthly_minor,omitempty"`
}

// ActivationRequest creates and pays the first subscription period. Set exactly
// one of PlanID and PlanConfigurationID and use the current catalog version.
type ActivationRequest struct {
	// CatalogVersion prevents checkout created under stale prices from activating.
	CatalogVersion string `json:"catalog_version"`
	// Set exactly one of PlanID and PlanConfigurationID.
	PlanID              PlanID        `json:"plan_id,omitempty"`
	PlanConfigurationID string        `json:"plan_configuration_id,omitempty"`
	Term                BillingTerm   `json:"term"`
	Seats               int           `json:"seats"`
	Timezone            string        `json:"timezone,omitempty"`
	PaymentStatus       PaymentStatus `json:"payment_status"`
	PaymentEventID      string        `json:"payment_event_id"`
	Currency            Currency      `json:"currency"`
	// PaidTotalMinor is the trusted payment total including VAT in halala.
	PaidTotalMinor ExactAmount      `json:"paid_total_minor"`
	Addons         []RecurringAddon `json:"addons,omitempty"`
}

// SubscriptionChangeRequest schedules a catalog-backed change at renewal; v1 does not prorate.
type SubscriptionChangeRequest struct {
	CatalogVersion      string           `json:"catalog_version"`
	PlanID              PlanID           `json:"plan_id,omitempty"`
	PlanConfigurationID string           `json:"plan_configuration_id,omitempty"`
	Term                BillingTerm      `json:"term,omitempty"`
	Seats               int              `json:"seats,omitempty"`
	Addons              []RecurringAddon `json:"addons,omitempty"`
	RequestedBy         string           `json:"requested_by,omitempty"`
	Reason              string           `json:"reason,omitempty"`
}

// CancellationRequest schedules cancellation at the end of the paid period.
type CancellationRequest struct {
	EventID string `json:"event_id,omitempty"`
	Reason  string `json:"reason,omitempty"`
}

// RenewalEventRequest applies one unique confirmed or failed payment-provider event.
type RenewalEventRequest struct {
	PaymentEventID string        `json:"payment_event_id"`
	PaymentStatus  PaymentStatus `json:"payment_status"`
	Currency       Currency      `json:"currency,omitempty"`
	PaidTotalMinor ExactAmount   `json:"paid_total_minor,omitempty"`
}

// ConfirmedTopUp records exact confirmed funding; PaidTotalMinor includes VAT.
type ConfirmedTopUp struct {
	AmountMinor    ExactAmount   `json:"amount_minor"`
	PaymentEventID string        `json:"payment_event_id"`
	PaymentStatus  PaymentStatus `json:"payment_status"`
	Currency       Currency      `json:"currency"`
	PaidTotalMinor ExactAmount   `json:"paid_total_minor"`
}

// ProviderRefundRequest records a confirmed refund as compensating immutable history.
type ProviderRefundRequest struct {
	AmountMinor        ExactAmount  `json:"amount_minor"`
	PaymentEventID     string       `json:"payment_event_id"`
	RefundStatus       RefundStatus `json:"refund_status"`
	Currency           Currency     `json:"currency"`
	RefundedTotalMinor ExactAmount  `json:"refunded_total_minor"`
	Reason             string       `json:"reason"`
}

// BudgetRequest configures one feature's subscription-month limit and reserve policy.
type BudgetRequest struct {
	Metric          BudgetMetric `json:"metric"`
	Period          BudgetPeriod `json:"period"`
	Limit           ExactAmount  `json:"limit"`
	WarningBPS      int          `json:"warning_bps,omitempty"`
	Action          BudgetAction `json:"action"`
	Sensitive       bool         `json:"sensitive,omitempty"`
	AbsoluteReserve ExactAmount  `json:"absolute_reserve,omitempty"`
	ReserveBPS      int          `json:"reserve_bps,omitempty"`
}

// UsageMetadata contains bounded reconciliation and application attribution. Do
// not include credentials, signatures, or unrestricted provider payloads.
type UsageMetadata struct {
	Actor            map[string]string `json:"actor,omitempty"`
	Channel          Channel           `json:"channel,omitempty"`
	ChannelAccountID string            `json:"channel_account_id,omitempty"`
	// Provider identifies the financial/tariff source. Feature-specific methods
	// populate it from their required provider field (or Meta for Meta messages).
	Provider string `json:"provider,omitempty"`
	// ProviderEventID is the provider-side deduplication key for this feature.
	ProviderEventID string `json:"provider_event_id,omitempty"`
	ConversationID  string `json:"conversation_id,omitempty"`
	CampaignID      string `json:"campaign_id,omitempty"`
	// RawQuantity and BillableQuantity preserve provider normalization evidence.
	RawQuantity      string `json:"raw_quantity,omitempty"`
	BillableQuantity string `json:"billable_quantity,omitempty"`
	// ProviderInvoiceID links the decision to a provider invoice/statement.
	ProviderInvoiceID string `json:"provider_invoice_id,omitempty"`
	// OriginalAmountMinor and OriginalCurrency record the pre-conversion amount.
	OriginalAmountMinor ExactAmount `json:"original_amount_minor,omitempty"`
	OriginalCurrency    string      `json:"original_currency,omitempty"`
	// FXRule and TariffVersion identify the versioned financial rules applied.
	FXRule        string         `json:"fx_rule,omitempty"`
	TariffVersion string         `json:"tariff_version,omitempty"`
	Attributes    map[string]any `json:"attributes,omitempty"`
}

// ConsumptionComponent is one unique feature charge within an atomic source event.
type ConsumptionComponent struct {
	FeatureCode         FeatureCode    `json:"feature_code"`
	Quantity            string         `json:"quantity,omitempty"`
	DurationSeconds     ExactAmount    `json:"duration_seconds,omitempty"`
	ProviderAmountMinor ExactAmount    `json:"provider_amount_minor,omitempty"`
	Metadata            *UsageMetadata `json:"metadata,omitempty"`
}

// ConsumptionRequest records either one top-level feature or one to ten Components.
// All components are accepted and charged together or rejected without partial debit.
type ConsumptionRequest struct {
	SourceEventID       string                 `json:"source_event_id"`
	OccurredAt          time.Time              `json:"occurred_at"`
	FeatureCode         FeatureCode            `json:"feature_code,omitempty"`
	Quantity            string                 `json:"quantity,omitempty"`
	DurationSeconds     ExactAmount            `json:"duration_seconds,omitempty"`
	ProviderAmountMinor ExactAmount            `json:"provider_amount_minor,omitempty"`
	Metadata            *UsageMetadata         `json:"metadata,omitempty"`
	Components          []ConsumptionComponent `json:"components,omitempty"`
}

// EligibilityRequest previews a charge without reserving funds or changing state.
type EligibilityRequest struct {
	Quantity            string                 `json:"quantity,omitempty"`
	DurationSeconds     ExactAmount            `json:"duration_seconds,omitempty"`
	ProviderAmountMinor ExactAmount            `json:"provider_amount_minor,omitempty"`
	Metadata            *UsageMetadata         `json:"metadata,omitempty"`
	Components          []ConsumptionComponent `json:"components,omitempty"`
}

// BalanceImpactPreviewRequest projects the exact input for a corresponding mutation.
// Request may be a ConsumptionRequest, ConfirmedTopUp, BudgetRequest, refund, or admin grant input.
type BalanceImpactPreviewRequest struct {
	Operation string `json:"operation"`
	Request   any    `json:"request"`
}

// BalanceImpact is one exact before/delta/after projection.
type BalanceImpact struct {
	Code   string      `json:"code"`
	Unit   string      `json:"unit"`
	Before ExactAmount `json:"before"`
	Delta  ExactAmount `json:"delta"`
	After  ExactAmount `json:"after"`
}

// APIError is a structured Mizan rejection. Retryable is authoritative; for a
// mutation, any retry must preserve the original request and IdempotencyKey.
type APIError struct {
	Status int
	// Code is stable machine vocabulary suitable for errors.Is and business logic.
	Code    ErrorCode
	Message string
	// Retryable permits an unchanged retry; it never permits a new key or altered input.
	Retryable bool
	Details   map[string]any
	RequestID string
	// IdempotencyKey must be retained when the mutation outcome is uncertain.
	IdempotencyKey string
}

func (e *APIError) Error() string { return fmt.Sprintf("mizan: %s: %s", e.Code, e.Message) }
func (e *APIError) Is(target error) bool {
	code, ok := target.(DomainError)
	return ok && ErrorCode(code) == e.Code
}

// NewConfirmedTopUp builds a confirmed SAR top-up. amount is principal in halala;
// paidTotal is the trusted total including VAT.
func NewConfirmedTopUp(amount ExactAmount, paymentEventID string, paidTotal ExactAmount) ConfirmedTopUp {
	return ConfirmedTopUp{AmountMinor: amount, PaymentEventID: paymentEventID, PaymentStatus: PaymentConfirmed, Currency: CurrencySAR, PaidTotalMinor: paidTotal}
}

// NewBudget builds a subscription-month budget with the default 80% warning threshold.
func NewBudget(metric BudgetMetric, limit ExactAmount, action BudgetAction) BudgetRequest {
	return BudgetRequest{Metric: metric, Period: BudgetSubscriptionMonth, Limit: limit, WarningBPS: 8000, Action: action}
}

// TransportError means the request outcome is unknown. A mutation may already
// have committed, so retry only with the identical input and IdempotencyKey.
type TransportError struct {
	Err            error
	RequestID      string
	IdempotencyKey string
}

func (e *TransportError) Error() string { return "mizan: request outcome is unknown: " + e.Err.Error() }
func (e *TransportError) Unwrap() error { return e.Err }

// ProtocolError reports an invalid, non-object, or oversized API response.
type ProtocolError struct {
	Message        string
	RequestID      string
	IdempotencyKey string
}

func (e *ProtocolError) Error() string { return "mizan: protocol error: " + e.Message }

// Logger receives structured SDK lifecycle events. Fields never include the token.
type Logger func(event string, fields map[string]any)

// Client is a calculation-free, concurrency-safe client for the authoritative API.
// Its exported fields may be configured before concurrent use and must not then be mutated.
type Client struct {
	BaseURL string
	// Token is a server credential and must never be logged or shipped to client applications.
	Token string
	// HTTPClient controls timeouts, transports, and connection reuse; nil uses a 10-second client.
	HTTPClient *http.Client
	// MaxAttempts applies to mutation retries only and defaults to three.
	MaxAttempts int
	Logger      Logger
}

// DeliveryEndpointInput configures a ledger or notification receiver. Omit
// AuthSecret to retain the current secret; responses never return the secret.
type DeliveryEndpointInput struct {
	EndpointURL     string `json:"endpoint_url"`
	AuthType        string `json:"auth_type,omitempty"`
	AuthSecret      string `json:"auth_secret,omitempty"`
	ClearAuthSecret bool   `json:"clear_auth_secret,omitempty"`
	Enabled         *bool  `json:"enabled,omitempty"`
	Reason          string `json:"reason"`
}

// AddonRolloutInput governs future selection and admin presentation without changing paid snapshots.
type AddonRolloutInput struct {
	DisplayName      string   `json:"display_name,omitempty"`
	Summary          string   `json:"summary,omitempty"`
	IncludedFeatures []string `json:"included_features,omitempty"`
	RolloutStage     string   `json:"rollout_stage,omitempty"`
	Enabled          *bool    `json:"enabled,omitempty"`
	RolloutNote      *string  `json:"rollout_note,omitempty"`
	DocumentationURL *string  `json:"documentation_url,omitempty"`
	Reason           string   `json:"reason"`
}

// DeliveryEndpoint reports the effective endpoint and whether it came from a
// business override or the global fallback.
type DeliveryEndpoint struct {
	Kind                 string `json:"kind"`
	Scope                string `json:"scope"`
	Source               string `json:"source"`
	EndpointURL          string `json:"endpoint_url"`
	AuthType             string `json:"auth_type"`
	AuthSecretConfigured bool   `json:"auth_secret_configured"`
	Enabled              bool   `json:"enabled"`
	Revision             int    `json:"revision"`
	UpdatedBy            string `json:"updated_by"`
	Reason               string `json:"reason"`
	UpdatedAt            string `json:"updated_at"`
}

// DeliveryConfigurationResult reports effective endpoint precedence and readiness.
type DeliveryConfigurationResult struct {
	Scope     string              `json:"scope"`
	Ready     bool                `json:"ready"`
	Endpoints []*DeliveryEndpoint `json:"endpoints"`
}

// NewClient creates a client with a 10-second HTTP timeout and three mutation attempts.
// baseURL must be absolute HTTP(S) without credentials, query, or fragment.
func NewClient(baseURL, token string) (*Client, error) {
	baseURL = strings.TrimRight(baseURL, "/")
	if baseURL == "" || token == "" {
		return nil, errors.New("mizan: base URL and token are required")
	}
	parsed, err := url.ParseRequestURI(baseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.User != nil {
		if err == nil {
			err = errors.New("URL must be absolute HTTP(S) without credentials, query, or fragment")
		}
		return nil, fmt.Errorf("mizan: invalid base URL: %w", err)
	}
	return &Client{BaseURL: baseURL, Token: token, HTTPClient: &http.Client{Timeout: 10 * time.Second}, MaxAttempts: 3}, nil
}

// AdminClient uses a dedicated Admin Worker token for attributed control-plane operations.
type AdminClient struct {
	*Client
	Actor string
	// Role is retained for source compatibility but ignored. The Admin Worker
	// derives authorization from the matched role-specific credential.
	Role string
}

// NewAdminClient creates an attributed billing_admin client using a dedicated admin token.
func NewAdminClient(baseURL, token, actor string) (*AdminClient, error) {
	client, err := NewClient(baseURL, token)
	if err != nil {
		return nil, err
	}
	if actor == "" {
		return nil, errors.New("mizan: admin actor is required")
	}
	return &AdminClient{Client: client, Actor: actor}, nil
}

func (c *AdminClient) headers() (map[string]string, error) {
	if c.Actor == "" {
		return nil, errors.New("mizan: admin actor is required")
	}
	return map[string]string{"X-Admin-Actor": c.Actor}, nil
}

// GetGlobalDeliveryEndpoints reads masked fallbacks used when no business row exists.
func (c *AdminClient) GetGlobalDeliveryEndpoints(ctx context.Context) (Response, error) {
	headers, err := c.headers()
	if err != nil {
		return nil, err
	}
	return c.requestWithHeaders(ctx, http.MethodGet, "/admin/api/delivery-endpoints", "", nil, "", false, headers)
}

// ListAddons reads global add-on contents, rollout stage, availability, and catalog pricing.
func (c *AdminClient) ListAddons(ctx context.Context) (Response, error) {
	headers, err := c.headers()
	if err != nil {
		return nil, err
	}
	return c.requestWithHeaders(ctx, http.MethodGet, "/admin/api/addons", "", nil, "", false, headers)
}

// ConfigureAddon updates attributed global rollout metadata for one catalog add-on.
func (c *AdminClient) ConfigureAddon(ctx context.Context, addonCode string, in AddonRolloutInput, idempotencyKey string) (Response, error) {
	if addonCode == "" {
		return nil, errors.New("mizan: addon code is required")
	}
	headers, err := c.headers()
	if err != nil {
		return nil, err
	}
	return c.requestWithHeaders(ctx, http.MethodPut, "/admin/api/addons/"+url.PathEscape(addonCode), "", in, idempotencyKey, true, headers)
}

// ListBusinesses reads one page of the admin business directory.
func (c *AdminClient) ListBusinesses(ctx context.Context, search string, offset, limit int) (Response, error) {
	headers, err := c.headers()
	if err != nil {
		return nil, err
	}
	query := url.Values{"search": {search}, "offset": {strconv.Itoa(offset)}, "limit": {strconv.Itoa(limit)}}
	return c.requestWithHeaders(ctx, http.MethodGet, "/admin/api/businesses?"+query.Encode(), "", nil, "", false, headers)
}

// ListUsageDecisions reads one newest-first page of immutable usage decisions.
func (c *AdminClient) ListUsageDecisions(ctx context.Context, businessID string, offset, limit int) (Response, error) {
	headers, err := c.headers()
	if err != nil {
		return nil, err
	}
	query := url.Values{"offset": {strconv.Itoa(offset)}, "limit": {strconv.Itoa(limit)}}
	path := "/admin/api/businesses/" + url.PathEscape(businessID) + "/usage-decisions?" + query.Encode()
	return c.requestWithHeaders(ctx, http.MethodGet, path, businessID, nil, "", false, headers)
}

// ListBusinessAudit reads one newest-first page of immutable attributed admin actions.
func (c *AdminClient) ListBusinessAudit(ctx context.Context, businessID string, offset, limit int) (Response, error) {
	headers, err := c.headers()
	if err != nil {
		return nil, err
	}
	query := url.Values{"offset": {strconv.Itoa(offset)}, "limit": {strconv.Itoa(limit)}}
	path := "/admin/api/businesses/" + url.PathEscape(businessID) + "/audit?" + query.Encode()
	return c.requestWithHeaders(ctx, http.MethodGet, path, businessID, nil, "", false, headers)
}

// ConfigureGlobalDeliveryEndpoint creates, rotates, enables, or disables one global fallback.
func (c *AdminClient) ConfigureGlobalDeliveryEndpoint(ctx context.Context, kind string, in DeliveryEndpointInput, idempotencyKey string) (Response, error) {
	if kind != "ledger" && kind != "notification" {
		return nil, errors.New("mizan: delivery kind must be ledger or notification")
	}
	headers, err := c.headers()
	if err != nil {
		return nil, err
	}
	if idempotencyKey == "" {
		idempotencyKey = newID()
	}
	return c.requestWithHeaders(ctx, http.MethodPut, "/admin/api/delivery-endpoints/"+kind, "", in, idempotencyKey, true, headers)
}

// GetBusinessDeliveryEndpoints reads the effective endpoints and source for a business.
func (c *AdminClient) GetBusinessDeliveryEndpoints(ctx context.Context, businessID string) (Response, error) {
	headers, err := c.headers()
	if err != nil {
		return nil, err
	}
	path := "/admin/api/businesses/" + url.PathEscape(businessID) + "/delivery-endpoints"
	return c.requestWithHeaders(ctx, http.MethodGet, path, businessID, nil, "", false, headers)
}

// ConfigureBusinessDeliveryEndpoint sets an explicit business row. A disabled row
// intentionally suppresses global fallback for that kind.
func (c *AdminClient) ConfigureBusinessDeliveryEndpoint(ctx context.Context, businessID, kind string, in DeliveryEndpointInput, idempotencyKey string) (Response, error) {
	if kind != "ledger" && kind != "notification" {
		return nil, errors.New("mizan: delivery kind must be ledger or notification")
	}
	headers, err := c.headers()
	if err != nil {
		return nil, err
	}
	if idempotencyKey == "" {
		idempotencyKey = newID()
	}
	path := "/admin/api/businesses/" + url.PathEscape(businessID) + "/delivery-endpoints/" + kind
	return c.requestWithHeaders(ctx, http.MethodPut, path, businessID, in, idempotencyKey, true, headers)
}

// ActivateSubscription validates payment against the exact invoice and creates the first period.
func (c *Client) ActivateSubscription(ctx context.Context, businessID string, in ActivationRequest, idempotencyKey string) (Response, error) {
	if err := validatePaymentEventID(in.PaymentEventID); err != nil {
		return nil, err
	}
	return c.mutate(ctx, http.MethodPost, c.businessPath(businessID, "subscriptions/activate"), businessID, in, idempotencyKey)
}

// ChangeSubscription schedules one change for the next renewal boundary.
func (c *Client) ChangeSubscription(ctx context.Context, businessID string, in SubscriptionChangeRequest, idempotencyKey string) (Response, error) {
	return c.mutate(ctx, http.MethodPost, c.businessPath(businessID, "subscriptions/change"), businessID, in, idempotencyKey)
}

// CancelSubscription schedules cancellation at the current paid period end.
func (c *Client) CancelSubscription(ctx context.Context, businessID string, in CancellationRequest, idempotencyKey string) (Response, error) {
	return c.mutate(ctx, http.MethodPost, c.businessPath(businessID, "subscriptions/cancel"), businessID, in, idempotencyKey)
}

// ApplyRenewalEvent records a unique renewal payment outcome and applies any pending change.
func (c *Client) ApplyRenewalEvent(ctx context.Context, businessID string, in RenewalEventRequest, idempotencyKey string) (Response, error) {
	if err := validatePaymentEventID(in.PaymentEventID); err != nil {
		return nil, err
	}
	return c.mutate(ctx, http.MethodPost, c.businessPath(businessID, "subscriptions/renewal-events"), businessID, in, idempotencyKey)
}

// TopUpAzeerUnits purchases a separately expiring exact Azeer Unit lot.
func (c *Client) TopUpAzeerUnits(ctx context.Context, businessID string, in ConfirmedTopUp, idempotencyKey string) (Response, error) {
	if err := validatePaymentRequest(in.PaymentEventID, in.AmountMinor, in.PaidTotalMinor); err != nil {
		return nil, err
	}
	return c.mutate(ctx, http.MethodPost, c.businessPath(businessID, "azeer-units/top-ups"), businessID, in, idempotencyKey)
}

// TopUpProviderBalance funds exact prepaid third-party costs in halala.
func (c *Client) TopUpProviderBalance(ctx context.Context, businessID string, in ConfirmedTopUp, idempotencyKey string) (Response, error) {
	if err := validatePaymentRequest(in.PaymentEventID, in.AmountMinor, in.PaidTotalMinor); err != nil {
		return nil, err
	}
	return c.mutate(ctx, http.MethodPost, c.businessPath(businessID, "provider-balance/top-ups"), businessID, in, idempotencyKey)
}

// RefundProviderBalance records a confirmed refund as compensating ledger history.
func (c *Client) RefundProviderBalance(ctx context.Context, businessID string, in ProviderRefundRequest, idempotencyKey string) (Response, error) {
	if err := validatePaymentRequest(in.PaymentEventID, in.AmountMinor, in.RefundedTotalMinor); err != nil {
		return nil, err
	}
	if strings.TrimSpace(in.Reason) == "" || len(in.Reason) > 1000 {
		return nil, errors.New("mizan: refund reason must contain 1 to 1000 characters")
	}
	return c.mutate(ctx, http.MethodPost, c.businessPath(businessID, "provider-balance/refunds"), businessID, in, idempotencyKey)
}

// SetFeatureBudget replaces one feature's subscription-month budget configuration.
func (c *Client) SetFeatureBudget(ctx context.Context, businessID string, featureCode FeatureCode, in BudgetRequest, idempotencyKey string) (Response, error) {
	return c.mutate(ctx, http.MethodPut, c.businessPath(businessID, "features/"+url.PathEscape(string(featureCode))+"/budget"), businessID, in, idempotencyKey)
}

// CheckEligibility returns an advisory preview that neither reserves balance nor changes state.
func (c *Client) CheckEligibility(ctx context.Context, businessID string, featureCode FeatureCode, in EligibilityRequest) (Response, error) {
	return c.request(ctx, http.MethodPost, c.businessPath(businessID, "features/"+url.PathEscape(string(featureCode))+"/eligibility"), businessID, in, "", false)
}

// GetEntitlement checks a capability in the active immutable subscription snapshot.
func (c *Client) GetEntitlement(ctx context.Context, businessID string, capability Capability) (Response, error) {
	return c.request(ctx, http.MethodGet, c.businessPath(businessID, "entitlements/"+url.PathEscape(string(capability))), businessID, nil, "", false)
}

// GetCatalog returns current plans, prices, versions, and public contract values.
func (c *Client) GetCatalog(ctx context.Context) (Response, error) {
	return c.request(ctx, http.MethodGet, "/v1/catalog", "", nil, "", false)
}

// Consume authoritatively checks and atomically records one source event.
func (c *Client) Consume(ctx context.Context, businessID string, in ConsumptionRequest, idempotencyKey string) (Response, error) {
	return c.mutate(ctx, http.MethodPost, c.businessPath(businessID, "consumptions"), businessID, in, idempotencyKey)
}

// PreviewBalanceImpact returns exact before/delta/after projections without reserving funds or changing state.
func (c *Client) PreviewBalanceImpact(ctx context.Context, businessID string, in BalanceImpactPreviewRequest) (Response, error) {
	return c.request(ctx, http.MethodPost, c.businessPath(businessID, "balance-impact-preview"), businessID, in, "", false)
}

// GetBillingSummary returns current account, subscription, balances, lots, budgets, and replication state.
func (c *Client) GetBillingSummary(ctx context.Context, businessID string) (Response, error) {
	return c.request(ctx, http.MethodGet, c.businessPath(businessID, "billing-summary"), businessID, nil, "", false)
}

// GetLedger returns up to limit entries strictly after afterSequence; limit must be 1..100.
func (c *Client) GetLedger(ctx context.Context, businessID string, afterSequence int64, limit int) (Response, error) {
	if afterSequence < 0 || limit < 1 || limit > 100 {
		return nil, errors.New("mizan: afterSequence must be non-negative and limit must be 1..100")
	}
	path := c.businessPath(businessID, "ledger") + "?after_sequence=" + strconv.FormatInt(afterSequence, 10) + "&limit=" + strconv.Itoa(limit)
	return c.request(ctx, http.MethodGet, path, businessID, nil, "", false)
}

func (c *Client) mutate(ctx context.Context, method, path, businessID string, in any, key string) (Response, error) {
	if key == "" {
		key = newID()
	}
	return c.request(ctx, method, path, businessID, in, key, true)
}

func (c *Client) request(ctx context.Context, method, path, businessID string, in any, key string, mutation bool) (Response, error) {
	return c.requestWithHeaders(ctx, method, path, businessID, in, key, mutation, nil)
}

func (c *Client) requestWithHeaders(ctx context.Context, method, path, businessID string, in any, key string, mutation bool, extraHeaders map[string]string) (Response, error) {
	var payload []byte
	var err error
	if in != nil {
		// Encode once so every uncertain mutation retry sends byte-identical JSON.
		payload, err = json.Marshal(in)
		if err != nil {
			return nil, fmt.Errorf("mizan: encode request: %w", err)
		}
	}
	attempts := c.MaxAttempts
	if attempts < 1 {
		attempts = 1
	}
	client := c.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	correlationID := newID()
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		// Correlation ID, idempotency key, and body remain stable; timestamp reflects this attempt.
		req, reqErr := http.NewRequestWithContext(ctx, method, c.BaseURL+path, bytes.NewReader(payload))
		if reqErr != nil {
			return nil, reqErr
		}
		req.Header.Set("Authorization", "Bearer "+c.Token)
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "mizan-go/"+Version)
		req.Header.Set("X-Business-Id", businessID)
		req.Header.Set("X-Request-ID", correlationID)
		req.Header.Set("X-Request-Timestamp", time.Now().UTC().Format(time.RFC3339Nano))
		if in != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		if key != "" {
			req.Header.Set("Idempotency-Key", key)
		}
		for name, value := range extraHeaders {
			req.Header.Set(name, value)
		}
		resp, sendErr := client.Do(req)
		if sendErr != nil {
			// Mutation outcome may be unknown; read failures are returned without automatic retry.
			lastErr = sendErr
			if !mutation || attempt == attempts {
				return nil, &TransportError{Err: sendErr, RequestID: correlationID, IdempotencyKey: key}
			}
		} else {
			// Read one byte beyond the safety limit so oversized responses are detectable.
			raw, readErr := io.ReadAll(io.LimitReader(resp.Body, (2<<20)+1))
			resp.Body.Close()
			if readErr != nil {
				lastErr = readErr
				if !mutation || attempt == attempts {
					return nil, &TransportError{Err: readErr, RequestID: correlationID, IdempotencyKey: key}
				}
			} else {
				if len(raw) > 2<<20 {
					return nil, &ProtocolError{Message: "response exceeded the 2 MiB safety limit", RequestID: correlationID, IdempotencyKey: key}
				}
				var result Response
				if len(raw) > 0 {
					if decodeErr := json.Unmarshal(raw, &result); decodeErr != nil || result == nil {
						return nil, &ProtocolError{Message: fmt.Sprintf("invalid JSON object response (HTTP %d)", resp.StatusCode), RequestID: correlationID, IdempotencyKey: key}
					}
				} else {
					result = Response{}
				}
				if resp.StatusCode >= 200 && resp.StatusCode < 300 {
					c.log("request_complete", map[string]any{"attempt": attempt, "status": resp.StatusCode, "request_id": correlationID})
					return result, nil
				}
				apiErr := decodeAPIError(resp.StatusCode, result)
				apiErr.IdempotencyKey = key
				if apiErr.RequestID == "" {
					apiErr.RequestID = resp.Header.Get("X-Request-ID")
				}
				lastErr = apiErr
				if !mutation || !apiErr.Retryable || attempt == attempts {
					return nil, apiErr
				}
				// Only authoritative retryable mutation errors continue to another attempt.
			}
		}
		c.log("request_retry", map[string]any{"attempt": attempt, "request_id": correlationID, "idempotency_key": key})
		// Full jitter with exponential growth avoids synchronized retry bursts.
		delay := time.Duration(rand.Int63n(int64(minDuration(2*time.Second, 100*time.Millisecond*time.Duration(1<<(attempt-1)))) + 1))
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
		}
	}
	return nil, &TransportError{Err: lastErr, RequestID: correlationID, IdempotencyKey: key}
}

func decodeAPIError(status int, body Response) *APIError {
	// HTTP_ERROR is a forward-compatible fallback when the server omits a valid envelope.
	e := &APIError{Status: status, Code: ErrorCode("HTTP_ERROR"), Message: fmt.Sprintf("HTTP %d", status), Details: map[string]any{}}
	if raw, ok := body["error"].(map[string]any); ok {
		if v, ok := raw["code"].(string); ok {
			e.Code = ErrorCode(v)
		}
		if v, ok := raw["message"].(string); ok {
			e.Message = v
		}
		if v, ok := raw["retryable"].(bool); ok {
			e.Retryable = v
		}
		if v, ok := raw["details"].(map[string]any); ok {
			e.Details = v
		}
		if v, ok := raw["request_id"].(string); ok {
			e.RequestID = v
		}
	}
	return e
}

func (c *Client) businessPath(businessID, suffix string) string {
	return "/v1/businesses/" + url.PathEscape(businessID) + "/" + suffix
}
func (c *Client) log(event string, fields map[string]any) {
	if c.Logger != nil {
		c.Logger(event, fields)
	}
}
func minDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}
func newID() string { return fmt.Sprintf("%d-%016x", time.Now().UnixNano(), rand.Uint64()) }
