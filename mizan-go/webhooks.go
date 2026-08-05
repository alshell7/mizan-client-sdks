package mizan

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
)

const (
	// HeaderOutboxID is stable across retries and is the receiver deduplication key.
	HeaderOutboxID = "X-Mizan-Outbox-Id"
	// HeaderAckSequence acknowledges durable application of exactly one ledger sequence.
	HeaderAckSequence = "X-Mizan-Ack-Sequence"
	// defaultWebhookBodyLimit bounds memory use when callers do not configure a limit.
	defaultWebhookBodyLimit = 1 << 20
)

// LedgerEntryType identifies why an immutable ledger entry was committed.
type LedgerEntryType string

const (
	LedgerSubscriptionActivated             LedgerEntryType = "subscription_activated"
	LedgerSubscriptionChangeScheduled       LedgerEntryType = "subscription_change_scheduled"
	LedgerSubscriptionCancellationScheduled LedgerEntryType = "subscription_cancellation_scheduled"
	LedgerSubscriptionCancelled             LedgerEntryType = "subscription_cancelled"
	LedgerSubscriptionCancellationRevoked   LedgerEntryType = "subscription_cancellation_revoked"
	LedgerRenewalFailed                     LedgerEntryType = "renewal_failed"
	LedgerSubscriptionRenewed               LedgerEntryType = "subscription_renewed"
	LedgerAzeerUnitsToppedUp                LedgerEntryType = "azeer_units_topped_up"
	LedgerProviderBalanceToppedUp           LedgerEntryType = "provider_balance_topped_up"
	LedgerProviderBalanceRefunded           LedgerEntryType = "provider_balance_refunded"
	LedgerFeatureBudgetUpdated              LedgerEntryType = "feature_budget_updated"
	LedgerBillingAccountDepleted            LedgerEntryType = "billing_account_depleted"
	LedgerFeaturePausedBudget               LedgerEntryType = "feature_paused_budget"
	LedgerFeaturePausedManual               LedgerEntryType = "feature_paused_manual"
	LedgerFeatureResumedManual              LedgerEntryType = "feature_resumed_manual"
	LedgerUsageRejected                     LedgerEntryType = "usage_rejected"
	LedgerUsageConsumed                     LedgerEntryType = "usage_consumed"
	LedgerIncludedUnitsExpired              LedgerEntryType = "included_units_expired"
	LedgerPurchasedUnitsExpired             LedgerEntryType = "purchased_units_expired"
	LedgerPromotionalUnitsExpired           LedgerEntryType = "promotional_units_expired"
	LedgerPromotionalUnitsGranted           LedgerEntryType = "promotional_units_granted"
	LedgerMonthlyIncludedUnitsGranted       LedgerEntryType = "monthly_included_units_granted"
	LedgerOutboxRetryRequested              LedgerEntryType = "outbox_retry_requested"
)

// NotificationType identifies a non-ledger operational notification.
type NotificationType string

const (
	NotificationBudgetWarning        NotificationType = "budget_warning"
	NotificationBudgetBreached       NotificationType = "budget_breached"
	NotificationBudgetPaused         NotificationType = "budget_paused"
	NotificationFeaturePausedManual  NotificationType = "feature_paused_manual"
	NotificationFeatureResumedManual NotificationType = "feature_resumed_manual"
)

// LedgerEntryWebhook is immutable financial history with versioned effective-time metadata.
type LedgerEntryWebhook struct {
	ID             string          `json:"id"`
	EntryType      LedgerEntryType `json:"entry_type"`
	SourceCommand  string          `json:"source_command"`
	SourceEventID  string          `json:"source_event_id"`
	SubscriptionID *string         `json:"subscription_id,omitempty"`
	FeatureCode    *FeatureCode    `json:"feature_code,omitempty"`
	EffectiveAt    string          `json:"effective_at"`
	CatalogVersion string          `json:"catalog_version"`
	PolicyVersion  string          `json:"policy_version"`
	Metadata       json.RawMessage `json:"metadata"`
}

// LedgerPostingWebhook is one exact debit or credit. Postings balance per rail and unit.
type LedgerPostingWebhook struct {
	Rail        string          `json:"rail"`
	AccountCode string          `json:"account_code"`
	Amount      ExactAmount     `json:"amount"`
	Unit        string          `json:"unit"`
	LotID       *string         `json:"lot_id,omitempty"`
	Metadata    json.RawMessage `json:"metadata,omitempty"`
}

// LedgerWebhook is delivered strictly by BusinessSequence and at least once.
type LedgerWebhook struct {
	EventID          string                 `json:"event_id"`
	BusinessID       string                 `json:"business_id"`
	BusinessSequence int64                  `json:"business_sequence"`
	Entry            LedgerEntryWebhook     `json:"entry"`
	Postings         []LedgerPostingWebhook `json:"postings"`
}

// NotificationWebhook is an at-least-once budget or feature-state notification.
type NotificationWebhook struct {
	Type        NotificationType `json:"type"`
	BusinessID  string           `json:"business_id"`
	FeatureCode FeatureCode      `json:"feature_code"`
	Period      string           `json:"period,omitempty"`
	Projected   ExactAmount      `json:"projected,omitempty"`
	Limit       ExactAmount      `json:"limit,omitempty"`
}

// WebhookContext contains transport identity. OutboxID is stable across retries
// and is the primary key applications should persist for duplicate suppression.
type WebhookContext struct {
	// OutboxID is stable across retries and should be persisted before returning success.
	OutboxID string
	// Headers excludes Authorization so application callbacks cannot leak the bearer token.
	Headers http.Header
	// RawBody is a defensive copy of the exact received JSON bytes.
	RawBody []byte
}

// WebhookHandler receives validated events. Returning nil means the application
// durably accepted the event. Returning an error causes a non-2xx response so
// Mizan retries it; ledger acknowledgement is never emitted on an error.
type WebhookHandler interface {
	HandleLedger(context.Context, LedgerWebhook, WebhookContext) error
	HandleNotification(context.Context, NotificationWebhook, WebhookContext) error
}

// WebhookHandlerFuncs adapts two functions to WebhookHandler.
type WebhookHandlerFuncs struct {
	Ledger       func(context.Context, LedgerWebhook, WebhookContext) error
	Notification func(context.Context, NotificationWebhook, WebhookContext) error
}

// HandleLedger dispatches to Ledger and returns an error when it is not configured.
func (h WebhookHandlerFuncs) HandleLedger(ctx context.Context, event LedgerWebhook, delivery WebhookContext) error {
	if h.Ledger == nil {
		return errors.New("mizan: ledger webhook handler is not configured")
	}
	return h.Ledger(ctx, event, delivery)
}

// HandleNotification dispatches to Notification and returns an error when it is not configured.
func (h WebhookHandlerFuncs) HandleNotification(ctx context.Context, event NotificationWebhook, delivery WebhookContext) error {
	if h.Notification == nil {
		return errors.New("mizan: notification webhook handler is not configured")
	}
	return h.Notification(ctx, event, delivery)
}

// WebhookResponse is framework-neutral. WriteHTTP can be used with net/http;
// the Fiber adapter maps the same response without changing protocol behavior.
type WebhookResponse struct {
	StatusCode int
	Headers    http.Header
	Body       []byte
}

// WriteHTTP writes the framework-neutral status, headers, and body to net/http.
func (r WebhookResponse) WriteHTTP(w http.ResponseWriter) {
	for name, values := range r.Headers {
		for _, value := range values {
			w.Header().Add(name, value)
		}
	}
	w.WriteHeader(r.StatusCode)
	_, _ = w.Write(r.Body)
}

// WebhookReceiver validates and dispatches both Mizan webhook streams. Set
// BearerToken when the delivery endpoint is configured with bearer auth.
type WebhookReceiver struct {
	Handler      WebhookHandler
	BearerToken  string
	MaxBodyBytes int
}

// ServeHTTP receives both webhook streams as a standard net/http handler.
func (r WebhookReceiver) ServeHTTP(w http.ResponseWriter, request *http.Request) {
	limit := r.bodyLimit()
	// Read one bounded body before delegating to the framework-neutral receiver.
	body, err := readBoundedBody(request, limit)
	if err != nil {
		r.errorResponse(http.StatusRequestEntityTooLarge, "request body exceeds the configured limit").WriteHTTP(w)
		return
	}
	r.Receive(request.Context(), request.Header, body).WriteHTTP(w)
}

// Receive lets any framework pass request headers and raw JSON to the SDK.
func (r WebhookReceiver) Receive(ctx context.Context, headers http.Header, body []byte) WebhookResponse {
	if len(body) == 0 {
		return r.errorResponse(http.StatusBadRequest, "request body is required")
	}
	if len(body) > r.bodyLimit() {
		return r.errorResponse(http.StatusRequestEntityTooLarge, "request body exceeds the configured limit")
	}
	if !r.authorized(headers.Get("Authorization")) {
		return r.errorResponse(http.StatusUnauthorized, "webhook authorization failed")
	}
	outboxID := strings.TrimSpace(headers.Get(HeaderOutboxID))
	if outboxID == "" {
		return r.errorResponse(http.StatusBadRequest, HeaderOutboxID+" is required")
	}
	if r.Handler == nil {
		return r.errorResponse(http.StatusServiceUnavailable, "webhook handler is not configured")
	}

	var envelope map[string]json.RawMessage
	// Decode only the top-level keys first so stream type can be selected safely.
	if err := json.Unmarshal(body, &envelope); err != nil || envelope == nil {
		return r.errorResponse(http.StatusBadRequest, "request body must be a JSON object")
	}
	callbackHeaders := headers.Clone()
	// Never expose the bearer credential to application handlers or their logs.
	callbackHeaders.Del("Authorization")
	delivery := WebhookContext{OutboxID: outboxID, Headers: callbackHeaders, RawBody: append([]byte(nil), body...)}
	if _, ledger := envelope["business_sequence"]; ledger {
		// Ledger shape takes precedence and is acknowledged only after handler success.
		var event LedgerWebhook
		if err := json.Unmarshal(body, &event); err != nil {
			return r.errorResponse(http.StatusUnprocessableEntity, "ledger webhook does not match the contract")
		}
		if err := validateLedgerWebhook(event); err != nil {
			return r.errorResponse(http.StatusUnprocessableEntity, err.Error())
		}
		if err := r.Handler.HandleLedger(ctx, event, delivery); err != nil {
			return r.errorResponse(http.StatusInternalServerError, "ledger webhook processing failed")
		}
		return WebhookResponse{StatusCode: http.StatusNoContent, Headers: http.Header{HeaderAckSequence: []string{fmt.Sprint(event.BusinessSequence)}}}
	}
	if _, notification := envelope["type"]; notification {
		// Notifications use outbox deduplication but have no sequence acknowledgement.
		var event NotificationWebhook
		if err := json.Unmarshal(body, &event); err != nil {
			return r.errorResponse(http.StatusUnprocessableEntity, "notification webhook does not match the contract")
		}
		if err := validateNotificationWebhook(event); err != nil {
			return r.errorResponse(http.StatusUnprocessableEntity, err.Error())
		}
		if err := r.Handler.HandleNotification(ctx, event, delivery); err != nil {
			return r.errorResponse(http.StatusInternalServerError, "notification webhook processing failed")
		}
		return WebhookResponse{StatusCode: http.StatusNoContent, Headers: make(http.Header)}
	}
	return r.errorResponse(http.StatusUnprocessableEntity, "unknown Mizan webhook payload")
}

func (r WebhookReceiver) bodyLimit() int {
	if r.MaxBodyBytes > 0 {
		return r.MaxBodyBytes
	}
	return defaultWebhookBodyLimit
}

func (r WebhookReceiver) authorized(value string) bool {
	if r.BearerToken == "" {
		return true
	}
	expected := "Bearer " + r.BearerToken
	// Equal-length constant-time comparison avoids token-dependent timing differences.
	return len(value) == len(expected) && subtle.ConstantTimeCompare([]byte(value), []byte(expected)) == 1
}

func (r WebhookReceiver) errorResponse(status int, message string) WebhookResponse {
	body, _ := json.Marshal(map[string]any{"error": map[string]string{"message": message}})
	return WebhookResponse{StatusCode: status, Headers: http.Header{"Content-Type": []string{"application/json"}}, Body: body}
}

func validateLedgerWebhook(event LedgerWebhook) error {
	if event.EventID == "" || event.Entry.ID == "" || event.EventID != event.Entry.ID {
		return errors.New("event_id must be present and equal entry.id")
	}
	if event.BusinessID == "" || event.BusinessSequence < 1 {
		return errors.New("business_id and a positive business_sequence are required")
	}
	if !validLedgerEntryType(event.Entry.EntryType) || event.Entry.SourceCommand == "" || event.Entry.SourceEventID == "" || event.Entry.EffectiveAt == "" || event.Entry.CatalogVersion == "" || event.Entry.PolicyVersion == "" || len(event.Entry.Metadata) == 0 {
		return errors.New("ledger entry fields do not match the contract")
	}
	balances := map[string]*big.Int{}
	for _, posting := range event.Postings {
		if posting.AccountCode == "" || (posting.Rail != "azeer_units" && posting.Rail != "provider_balance" && posting.Rail != "invoice") || (posting.Unit != "milliunit" && posting.Unit != "halala") {
			return errors.New("ledger posting fields do not match the contract")
		}
		amount, ok := new(big.Int).SetString(string(posting.Amount), 10)
		if !ok || !amount.IsInt64() || !isCanonicalInteger(string(posting.Amount)) {
			return errors.New("ledger posting amounts must be exact integer strings")
		}
		key := posting.Rail + "\x00" + posting.Unit
		// Rail and unit form separate balancing domains; money cannot offset milliunits.
		if balances[key] == nil {
			balances[key] = new(big.Int)
		}
		balances[key].Add(balances[key], amount)
	}
	for _, balance := range balances {
		if balance.Sign() != 0 {
			return errors.New("ledger postings must balance to zero per rail and unit")
		}
	}
	return nil
}

func validateNotificationWebhook(event NotificationWebhook) error {
	if event.BusinessID == "" || event.FeatureCode == "" {
		return errors.New("notification business_id and feature_code are required")
	}
	switch event.Type {
	case NotificationBudgetWarning, NotificationBudgetBreached:
		if event.Period == "" || !isUnsignedExactAmount(string(event.Projected)) || !isUnsignedExactAmount(string(event.Limit)) {
			return errors.New("budget threshold notifications require period, projected, and limit")
		}
	case NotificationBudgetPaused:
		if event.Period == "" {
			return errors.New("budget_paused notifications require period")
		}
	case NotificationFeaturePausedManual, NotificationFeatureResumedManual:
	default:
		return errors.New("notification type is not supported")
	}
	return nil
}

func validLedgerEntryType(value LedgerEntryType) bool {
	switch value {
	case LedgerSubscriptionActivated, LedgerSubscriptionChangeScheduled, LedgerSubscriptionCancellationScheduled,
		LedgerSubscriptionCancelled, LedgerSubscriptionCancellationRevoked, LedgerRenewalFailed,
		LedgerSubscriptionRenewed, LedgerAzeerUnitsToppedUp, LedgerProviderBalanceToppedUp,
		LedgerProviderBalanceRefunded, LedgerFeatureBudgetUpdated, LedgerBillingAccountDepleted,
		LedgerFeaturePausedBudget, LedgerFeaturePausedManual, LedgerFeatureResumedManual,
		LedgerUsageRejected, LedgerUsageConsumed, LedgerIncludedUnitsExpired, LedgerPurchasedUnitsExpired,
		LedgerPromotionalUnitsExpired, LedgerPromotionalUnitsGranted, LedgerMonthlyIncludedUnitsGranted,
		LedgerOutboxRetryRequested:
		return true
	default:
		return false
	}
}

func isCanonicalInteger(value string) bool {
	if strings.HasPrefix(value, "-") {
		return len(value) > 1 && isUnsignedCanonicalInteger(value[1:])
	}
	return isUnsignedCanonicalInteger(value)
}

func isUnsignedCanonicalInteger(value string) bool {
	if value == "0" {
		return true
	}
	if value == "" || value[0] < '1' || value[0] > '9' {
		return false
	}
	for i := 1; i < len(value); i++ {
		if value[i] < '0' || value[i] > '9' {
			return false
		}
	}
	return true
}

func isUnsignedExactAmount(value string) bool {
	amount, ok := new(big.Int).SetString(value, 10)
	return ok && amount.IsInt64() && amount.Sign() >= 0 && isUnsignedCanonicalInteger(value)
}

func readBoundedBody(request *http.Request, limit int) ([]byte, error) {
	defer request.Body.Close()
	// Read one extra byte so the exact limit and an oversized body are distinguishable.
	body, err := io.ReadAll(io.LimitReader(request.Body, int64(limit)+1))
	if err != nil {
		return nil, err
	}
	if len(body) > limit {
		return nil, errors.New("body too large")
	}
	return body, nil
}
