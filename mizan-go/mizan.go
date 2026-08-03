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

const Version = "1.3.0"

type ExactAmount string
type Response map[string]any

type PlanID string

const (
	PlanStart   PlanID = "start"
	PlanGrowth  PlanID = "growth"
	PlanCommand PlanID = "command"
)

type BillingTerm string

const (
	TermMonthly    BillingTerm = "monthly"
	TermQuarterly  BillingTerm = "quarterly"
	TermSemiAnnual BillingTerm = "semi_annual"
	TermAnnual     BillingTerm = "annual"
)

type FeatureCode string

const (
	FeatureConversation24H              FeatureCode = "conversation_24h"
	FeatureOutboundDeliveredMessage     FeatureCode = "outbound_delivered_message"
	FeatureAIAssistOverAllowance        FeatureCode = "ai_assist_action_over_allowance"
	FeatureVoiceAIStartedMinute         FeatureCode = "voice_ai_started_minute"
	FeatureAIReplyHandling              FeatureCode = "ai_reply_handling"
	FeatureWhatsAppMetaMarketingMessage FeatureCode = "whatsapp_meta_marketing_msg"
	FeatureTelephonyVoiceMinute         FeatureCode = "telephony_voice_minute"
	FeatureInboundVoiceMinute           FeatureCode = "inbound_voice_minute"
	FeatureOtherProviderCharge          FeatureCode = "other_provider_charge"
)

type Currency string

const CurrencySAR Currency = "SAR"

type PaymentStatus string

const (
	PaymentConfirmed PaymentStatus = "confirmed"
	PaymentFailed    PaymentStatus = "failed"
)

type RefundStatus string

const RefundConfirmed RefundStatus = "confirmed"

type BudgetMetric string

const (
	BudgetAzeerUnitMillis BudgetMetric = "azeer_unit_millis"
	BudgetMoneyMinor      BudgetMetric = "money_minor"
	BudgetQuantity        BudgetMetric = "quantity"
)

type BudgetPeriod string

const BudgetSubscriptionMonth BudgetPeriod = "subscription_month"

type BudgetAction string

const (
	BudgetAlert BudgetAction = "alert"
	BudgetPause BudgetAction = "pause"
)

type Channel string

const (
	ChannelWhatsApp  Channel = "whatsapp"
	ChannelInstagram Channel = "instagram"
	ChannelFacebook  Channel = "facebook"
	ChannelTikTok    Channel = "tiktok"
	ChannelTelephony Channel = "telephony"
	ChannelWebchat   Channel = "webchat"
)

type RecurringAddonCode string

const (
	AddonWhatsApp011Landline         RecurringAddonCode = "whatsapp_011_landline"
	AddonWhatsApp05Mobile            RecurringAddonCode = "whatsapp_05_mobile"
	AddonConcurrentCalls5            RecurringAddonCode = "concurrent_calls_5"
	AddonConcurrentCalls10           RecurringAddonCode = "concurrent_calls_10"
	AddonConcurrentCalls20           RecurringAddonCode = "concurrent_calls_20"
	AddonAutoDialer                  RecurringAddonCode = "auto_dialer"
	AddonCSATStart                   RecurringAddonCode = "csat_start"
	AddonInstagramAdditionalAccounts RecurringAddonCode = "instagram_additional_accounts"
	AddonWhatsApp9200                RecurringAddonCode = "whatsapp_9200"
	AddonTollFree800                 RecurringAddonCode = "toll_free_800"
	AddonInternationalNumber         RecurringAddonCode = "international_number"
	AddonOutboundMinutes500          RecurringAddonCode = "outbound_minute_bundle_500"
	AddonOutboundMinutes1000         RecurringAddonCode = "outbound_minute_bundle_1000"
	AddonVoiceBroadcast              RecurringAddonCode = "voice_broadcast"
	AddonExtendedRecordingRetention  RecurringAddonCode = "recording_retention_extended"
)

type ErrorCode string

const (
	ErrCodeInvalidRequest                   ErrorCode = "INVALID_REQUEST"
	ErrCodeUnauthorized                     ErrorCode = "UNAUTHORIZED"
	ErrCodeForbidden                        ErrorCode = "FORBIDDEN"
	ErrCodeNotFound                         ErrorCode = "NOT_FOUND"
	ErrCodeAccountInactive                  ErrorCode = "ACCOUNT_INACTIVE"
	ErrCodeSubscriptionInactive             ErrorCode = "SUBSCRIPTION_INACTIVE"
	ErrCodeFeatureDisabled                  ErrorCode = "FEATURE_DISABLED"
	ErrCodeFeaturePausedBudget              ErrorCode = "FEATURE_PAUSED_BUDGET"
	ErrCodeFeaturePausedManual              ErrorCode = "FEATURE_PAUSED_MANUAL"
	ErrCodeInsufficientAzeerUnits           ErrorCode = "INSUFFICIENT_AZEER_UNITS"
	ErrCodeInsufficientProviderBalance      ErrorCode = "INSUFFICIENT_PROVIDER_BALANCE"
	ErrCodePaymentAmountMismatch            ErrorCode = "PAYMENT_AMOUNT_MISMATCH"
	ErrCodeIdempotencyKeyReused             ErrorCode = "IDEMPOTENCY_KEY_REUSED"
	ErrCodeInternalRetryable                ErrorCode = "INTERNAL_RETRYABLE"
	ErrCodeDependencyTemporarilyUnavailable ErrorCode = "DEPENDENCY_TEMPORARILY_UNAVAILABLE"
	ErrCodeDuplicatePaymentEvent            ErrorCode = "DUPLICATE_PAYMENT_EVENT"
	ErrCodeDuplicateProviderEvent           ErrorCode = "DUPLICATE_PROVIDER_EVENT"
	ErrCodeDuplicateSourceEvent             ErrorCode = "DUPLICATE_SOURCE_EVENT"
	ErrCodeEarlyRenewalEvent                ErrorCode = "EARLY_RENEWAL_EVENT"
	ErrCodeInvalidQuantity                  ErrorCode = "INVALID_QUANTITY"
	ErrCodeInvariantViolation               ErrorCode = "INVARIANT_VIOLATION"
	ErrCodeMisconfigured                    ErrorCode = "MISCONFIGURED"
	ErrCodeQuoteRequired                    ErrorCode = "QUOTE_REQUIRED"
	ErrCodeQuoteVerificationUnavailable     ErrorCode = "QUOTE_VERIFICATION_UNAVAILABLE"
	ErrCodeRequestTimestampOutOfRange       ErrorCode = "REQUEST_TIMESTAMP_OUT_OF_RANGE"
	ErrCodeSensitiveReserveReached          ErrorCode = "SENSITIVE_RESERVE_REACHED"
	ErrCodeStalePlanVersion                 ErrorCode = "STALE_PLAN_VERSION"
	ErrCodeSubscriptionChangePending        ErrorCode = "SUBSCRIPTION_CHANGE_PENDING"
)

type DomainError ErrorCode

func (e DomainError) Error() string { return "mizan: " + string(e) }

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

type Balance struct {
	AzeerUnitMillis      ExactAmount `json:"azeer_unit_millis"`
	ProviderBalanceMinor ExactAmount `json:"provider_balance_minor"`
}

type InvoiceLine struct {
	Code               string      `json:"code"`
	Quantity           ExactAmount `json:"quantity"`
	NetMinor           ExactAmount `json:"net_minor"`
	DiscountMinor      ExactAmount `json:"discount_minor"`
	AfterDiscountMinor ExactAmount `json:"after_discount_minor"`
	VATMinor           ExactAmount `json:"vat_minor"`
	TotalMinor         ExactAmount `json:"total_minor"`
}

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

type ConsumptionResult struct {
	Accepted             bool                   `json:"accepted"`
	Code                 string                 `json:"code"`
	SourceEventID        string                 `json:"source_event_id"`
	LedgerEntryID        string                 `json:"ledger_entry_id"`
	BusinessSequence     int64                  `json:"business_sequence"`
	Charges              []map[string]any       `json:"charges"`
	AllocationsByFeature []map[string]any       `json:"allocations_by_feature"`
	Totals               map[string]ExactAmount `json:"totals"`
	Balances             Balance                `json:"balances"`
	Details              map[string]any         `json:"details"`
}

type EntitlementResult struct {
	Capability Capability     `json:"capability"`
	Enabled    bool           `json:"enabled"`
	PlanID     string         `json:"plan_id"`
	FairUse    map[string]any `json:"fair_use"`
}

type LedgerResult struct {
	Entries           []map[string]any `json:"entries"`
	NextAfterSequence *int64           `json:"next_after_sequence"`
}

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

type RecurringAddon struct {
	Code                 RecurringAddonCode `json:"code"`
	Quantity             ExactAmount        `json:"quantity,omitempty"`
	ApprovedQuoteID      string             `json:"approved_quote_id,omitempty"`
	ApprovedMonthlyMinor ExactAmount        `json:"approved_monthly_minor,omitempty"`
}

type ActivationRequest struct {
	CatalogVersion      string           `json:"catalog_version"`
	PlanID              PlanID           `json:"plan_id,omitempty"`
	PlanConfigurationID string           `json:"plan_configuration_id,omitempty"`
	Term                BillingTerm      `json:"term"`
	Seats               int              `json:"seats"`
	Timezone            string           `json:"timezone,omitempty"`
	PaymentStatus       PaymentStatus    `json:"payment_status"`
	PaymentEventID      string           `json:"payment_event_id"`
	Currency            Currency         `json:"currency"`
	PaidTotalMinor      ExactAmount      `json:"paid_total_minor"`
	Addons              []RecurringAddon `json:"addons,omitempty"`
	Services            []map[string]any `json:"services,omitempty"`
}

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

type CancellationRequest struct {
	EventID string `json:"event_id,omitempty"`
	Reason  string `json:"reason,omitempty"`
}

type RenewalEventRequest struct {
	PaymentEventID string        `json:"payment_event_id"`
	PaymentStatus  PaymentStatus `json:"payment_status"`
	Currency       Currency      `json:"currency,omitempty"`
	PaidTotalMinor ExactAmount   `json:"paid_total_minor,omitempty"`
}

type ConfirmedTopUp struct {
	AmountMinor    ExactAmount   `json:"amount_minor"`
	PaymentEventID string        `json:"payment_event_id"`
	PaymentStatus  PaymentStatus `json:"payment_status"`
	Currency       Currency      `json:"currency"`
	PaidTotalMinor ExactAmount   `json:"paid_total_minor"`
}

type ProviderRefundRequest struct {
	AmountMinor        ExactAmount  `json:"amount_minor"`
	PaymentEventID     string       `json:"payment_event_id"`
	RefundStatus       RefundStatus `json:"refund_status"`
	Currency           Currency     `json:"currency"`
	RefundedTotalMinor ExactAmount  `json:"refunded_total_minor"`
	Reason             string       `json:"reason"`
}

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

type UsageMetadata struct {
	Actor            map[string]string `json:"actor,omitempty"`
	Channel          Channel           `json:"channel,omitempty"`
	ChannelAccountID string            `json:"channel_account_id,omitempty"`
	Provider         string            `json:"provider,omitempty"`
	ProviderEventID  string            `json:"provider_event_id,omitempty"`
	ConversationID   string            `json:"conversation_id,omitempty"`
	CampaignID       string            `json:"campaign_id,omitempty"`
	RawQuantity      string            `json:"raw_quantity,omitempty"`
	BillableQuantity string            `json:"billable_quantity,omitempty"`
	Attributes       map[string]any    `json:"attributes,omitempty"`
}

type ConsumptionComponent struct {
	FeatureCode         FeatureCode    `json:"feature_code"`
	Quantity            string         `json:"quantity,omitempty"`
	DurationSeconds     ExactAmount    `json:"duration_seconds,omitempty"`
	ProviderAmountMinor ExactAmount    `json:"provider_amount_minor,omitempty"`
	Metadata            *UsageMetadata `json:"metadata,omitempty"`
}

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

type EligibilityRequest struct {
	Quantity            string                 `json:"quantity,omitempty"`
	DurationSeconds     ExactAmount            `json:"duration_seconds,omitempty"`
	ProviderAmountMinor ExactAmount            `json:"provider_amount_minor,omitempty"`
	Metadata            *UsageMetadata         `json:"metadata,omitempty"`
	Components          []ConsumptionComponent `json:"components,omitempty"`
}

type APIError struct {
	Status         int
	Code           ErrorCode
	Message        string
	Retryable      bool
	Details        map[string]any
	RequestID      string
	IdempotencyKey string
}

func (e *APIError) Error() string { return fmt.Sprintf("mizan: %s: %s", e.Code, e.Message) }
func (e *APIError) Is(target error) bool {
	code, ok := target.(DomainError)
	return ok && ErrorCode(code) == e.Code
}

func NewConfirmedTopUp(amount ExactAmount, paymentEventID string, paidTotal ExactAmount) ConfirmedTopUp {
	return ConfirmedTopUp{AmountMinor: amount, PaymentEventID: paymentEventID, PaymentStatus: PaymentConfirmed, Currency: CurrencySAR, PaidTotalMinor: paidTotal}
}
func NewBudget(metric BudgetMetric, limit ExactAmount, action BudgetAction) BudgetRequest {
	return BudgetRequest{Metric: metric, Period: BudgetSubscriptionMonth, Limit: limit, WarningBPS: 8000, Action: action}
}

type TransportError struct {
	Err            error
	RequestID      string
	IdempotencyKey string
}

func (e *TransportError) Error() string { return "mizan: request outcome is unknown: " + e.Err.Error() }
func (e *TransportError) Unwrap() error { return e.Err }

type ProtocolError struct {
	Message        string
	RequestID      string
	IdempotencyKey string
}

func (e *ProtocolError) Error() string { return "mizan: protocol error: " + e.Message }

type Logger func(event string, fields map[string]any)

type Client struct {
	BaseURL     string
	Token       string
	HTTPClient  *http.Client
	MaxAttempts int
	Logger      Logger
}

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

func (c *Client) ActivateSubscription(ctx context.Context, businessID string, in ActivationRequest, idempotencyKey string) (Response, error) {
	return c.mutate(ctx, http.MethodPost, c.businessPath(businessID, "subscriptions/activate"), businessID, in, idempotencyKey)
}
func (c *Client) ChangeSubscription(ctx context.Context, businessID string, in SubscriptionChangeRequest, idempotencyKey string) (Response, error) {
	return c.mutate(ctx, http.MethodPost, c.businessPath(businessID, "subscriptions/change"), businessID, in, idempotencyKey)
}
func (c *Client) CancelSubscription(ctx context.Context, businessID string, in CancellationRequest, idempotencyKey string) (Response, error) {
	return c.mutate(ctx, http.MethodPost, c.businessPath(businessID, "subscriptions/cancel"), businessID, in, idempotencyKey)
}
func (c *Client) ApplyRenewalEvent(ctx context.Context, businessID string, in RenewalEventRequest, idempotencyKey string) (Response, error) {
	return c.mutate(ctx, http.MethodPost, c.businessPath(businessID, "subscriptions/renewal-events"), businessID, in, idempotencyKey)
}
func (c *Client) TopUpAzeerUnits(ctx context.Context, businessID string, in ConfirmedTopUp, idempotencyKey string) (Response, error) {
	return c.mutate(ctx, http.MethodPost, c.businessPath(businessID, "azeer-units/top-ups"), businessID, in, idempotencyKey)
}
func (c *Client) TopUpProviderBalance(ctx context.Context, businessID string, in ConfirmedTopUp, idempotencyKey string) (Response, error) {
	return c.mutate(ctx, http.MethodPost, c.businessPath(businessID, "provider-balance/top-ups"), businessID, in, idempotencyKey)
}
func (c *Client) RefundProviderBalance(ctx context.Context, businessID string, in ProviderRefundRequest, idempotencyKey string) (Response, error) {
	return c.mutate(ctx, http.MethodPost, c.businessPath(businessID, "provider-balance/refunds"), businessID, in, idempotencyKey)
}
func (c *Client) SetFeatureBudget(ctx context.Context, businessID string, featureCode FeatureCode, in BudgetRequest, idempotencyKey string) (Response, error) {
	return c.mutate(ctx, http.MethodPut, c.businessPath(businessID, "features/"+url.PathEscape(string(featureCode))+"/budget"), businessID, in, idempotencyKey)
}
func (c *Client) CheckEligibility(ctx context.Context, businessID string, featureCode FeatureCode, in EligibilityRequest) (Response, error) {
	return c.request(ctx, http.MethodPost, c.businessPath(businessID, "features/"+url.PathEscape(string(featureCode))+"/eligibility"), businessID, in, "", false)
}

func (c *Client) GetEntitlement(ctx context.Context, businessID string, capability Capability) (Response, error) {
	return c.request(ctx, http.MethodGet, c.businessPath(businessID, "entitlements/"+url.PathEscape(string(capability))), businessID, nil, "", false)
}

func (c *Client) GetCatalog(ctx context.Context) (Response, error) {
	return c.request(ctx, http.MethodGet, "/v1/catalog", "", nil, "", false)
}
func (c *Client) Consume(ctx context.Context, businessID string, in ConsumptionRequest, idempotencyKey string) (Response, error) {
	return c.mutate(ctx, http.MethodPost, c.businessPath(businessID, "consumptions"), businessID, in, idempotencyKey)
}
func (c *Client) GetBillingSummary(ctx context.Context, businessID string) (Response, error) {
	return c.request(ctx, http.MethodGet, c.businessPath(businessID, "billing-summary"), businessID, nil, "", false)
}
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
	var payload []byte
	var err error
	if in != nil {
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
		resp, sendErr := client.Do(req)
		if sendErr != nil {
			lastErr = sendErr
			if !mutation || attempt == attempts {
				return nil, &TransportError{Err: sendErr, RequestID: correlationID, IdempotencyKey: key}
			}
		} else {
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
			}
		}
		c.log("request_retry", map[string]any{"attempt": attempt, "request_id": correlationID, "idempotency_key": key})
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
