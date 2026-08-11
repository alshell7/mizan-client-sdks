// Command end_to_end runs a complete customer billing lifecycle against Mizan.
//
// It is intentionally configured only through trusted server-side environment
// variables. Payment totals must come from a verified payment-provider event;
// never forward values supplied by a browser or mobile client.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	mizan "github.com/alshell7/mizan-client-sdks/mizan-go"
)

type eligibilityResult struct {
	Eligible bool           `json:"eligible"`
	Reason   string         `json:"reason"`
	Details  map[string]any `json:"details"`
}

func main() {
	if err := run(); err != nil {
		log.Fatal(describeError(err))
	}
}

func run() error {
	baseURL := requiredEnv("MIZAN_BASE_URL")
	token := requiredEnv("MIZAN_API_TOKEN")
	businessID := requiredEnv("MIZAN_BUSINESS_ID")
	paidTotal := requiredEnv("MIZAN_ACTIVATION_PAID_TOTAL_MINOR")
	runID := envOr("MIZAN_EXAMPLE_RUN_ID", "checkout-001")

	client, err := mizan.NewBusinessClient(baseURL, token, businessID)
	if err != nil {
		return fmt.Errorf("configure client: %w", err)
	}
	client.Logger = func(event string, fields map[string]any) {
		// SDK lifecycle fields contain no token or request body.
		log.Printf("mizan event=%s fields=%v", event, fields)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 1. Load the live catalog. Persist this version with checkout state before
	// taking payment; the example uses it immediately.
	catalog, err := client.GetCatalog(ctx)
	if err != nil {
		return fmt.Errorf("load catalog: %w", err)
	}
	catalogVersion, ok := catalog["catalog_version"].(string)
	if !ok || catalogVersion == "" {
		return errors.New("catalog response has no catalog_version")
	}
	fmt.Printf("catalog_version=%s\n", catalogVersion)

	// 2. Activate from a trusted, confirmed payment event. The run ID is part of
	// both the provider identity and idempotency key so an unchanged replay is safe.
	paymentEventID := "example-activation:" + runID
	activationRequest := mizan.ActivationRequest{
		CatalogVersion: catalogVersion,
		PlanID:         mizan.PlanStart,
		Term:           mizan.TermMonthly,
		Seats:          1,
		Timezone:       envOr("MIZAN_BUSINESS_TIMEZONE", "Asia/Riyadh"),
		PaymentStatus:  mizan.PaymentConfirmed,
		PaymentEventID: paymentEventID,
		Currency:       mizan.CurrencySAR,
		PaidTotalMinor: mizan.ExactAmount(paidTotal),
	}
	activationKey := "activate:" + businessID + ":" + runID
	response, err := client.ActivateSubscription(ctx, businessID, activationRequest, activationKey)
	if err != nil {
		return fmt.Errorf("activate subscription: %w", err)
	}
	activation, err := mizan.DecodeData[mizan.ActivationResult](response)
	if err != nil {
		return err
	}
	fmt.Printf("subscription=%s period=%s..%s included_millis=%s\n",
		activation.SubscriptionID,
		activation.CurrentPeriodStart.Format(time.RFC3339Nano),
		activation.CurrentPeriodEnd.Format(time.RFC3339Nano),
		activation.IncludedUnitMillisGranted,
	)

	// 3. Optionally fund the provider rail. Both exact values must be obtained
	// from the trusted payment record; the SDK never calculates VAT.
	if err := maybeFundProvider(ctx, client, businessID, runID); err != nil {
		return err
	}

	// 4. Entitlement answers whether a subscription snapshot contains a product
	// capability. Eligibility is only an advisory, short-lived usage preview.
	response, err = client.GetEntitlement(ctx, businessID, mizan.CapabilityBasicAnalytics)
	if err != nil {
		return fmt.Errorf("check entitlement: %w", err)
	}
	entitlement, err := mizan.DecodeData[mizan.EntitlementResult](response)
	if err != nil {
		return err
	}
	fmt.Printf("entitlement capability=%s enabled=%t\n", entitlement.Capability, entitlement.Enabled)

	usageMetadata := &mizan.UsageMetadata{
		Channel:        mizan.ChannelWhatsApp,
		ConversationID: "example-conversation:" + runID,
	}
	response, err = client.CheckEligibility(ctx, businessID, mizan.FeatureOutboundDeliveredMessage, mizan.EligibilityRequest{
		Quantity: "1",
		Metadata: usageMetadata,
	})
	if err != nil {
		return fmt.Errorf("check eligibility: %w", err)
	}
	eligibility, err := mizan.DecodeData[eligibilityResult](response)
	if err != nil {
		return err
	}
	fmt.Printf("eligibility eligible=%t reason=%s\n", eligibility.Eligible, eligibility.Reason)
	if !eligibility.Eligible {
		return fmt.Errorf("advisory eligibility rejected usage: %s (%v)", eligibility.Reason, eligibility.Details)
	}

	// 5. Consumption remains authoritative. Persist the product event's real
	// timestamp and report it only within the subscription's current open month.
	sourceEventID := "example-message:" + runID
	consumeKey := "consume:" + businessID + ":" + runID
	usage := mizan.OutboundDeliveredMessageUsage{
		SourceEventID: sourceEventID,
		OccurredAt:    time.Now().UTC(),
		Quantity:      "1",
		Metadata:      usageMetadata,
	}
	response, err = client.ConsumeOutboundDeliveredMessage(ctx, businessID, usage, consumeKey)
	if err != nil {
		return fmt.Errorf("consume usage: %w", err)
	}
	decision, err := mizan.DecodeData[mizan.ConsumptionResult](response)
	if err != nil {
		return err
	}
	fmt.Printf("consumption accepted=%t ledger=%s sequence=%d balance_millis=%s\n",
		decision.Accepted, decision.LedgerEntryID, decision.BusinessSequence, decision.Balances.AzeerUnitMillis)

	// Replay the identical domain operation to make idempotency observable. It
	// must return the original ledger identity, not create a second debit.
	replay, err := client.ConsumeOutboundDeliveredMessage(ctx, businessID, usage, consumeKey)
	if err != nil {
		return fmt.Errorf("replay consumption: %w", err)
	}
	replayedDecision, err := mizan.DecodeData[mizan.ConsumptionResult](replay)
	if err != nil {
		return err
	}
	if replayedDecision.LedgerEntryID != decision.LedgerEntryID || replayedDecision.BusinessSequence != decision.BusinessSequence {
		return errors.New("idempotent replay returned a different ledger identity")
	}
	fmt.Println("idempotency_replay=original_result")

	// 6. Read the materialized billing view, then page immutable history by the
	// business sequence. Persist the last exported sequence in real integrations.
	response, err = client.GetBillingSummary(ctx, businessID)
	if err != nil {
		return fmt.Errorf("read billing summary: %w", err)
	}
	summary, err := mizan.DecodeData[mizan.BillingSummaryResult](response)
	if err != nil {
		return err
	}
	fmt.Printf("summary azeer_unit_millis=%s provider_balance_minor=%s\n",
		summary.Balances.AzeerUnitMillis, summary.Balances.ProviderBalanceMinor)

	entryCount, lastSequence, err := exportLedger(ctx, client, businessID)
	if err != nil {
		return err
	}
	fmt.Printf("ledger entries=%d last_sequence=%d\n", entryCount, lastSequence)
	return nil
}

func maybeFundProvider(ctx context.Context, client *mizan.Client, businessID, runID string) error {
	amount := os.Getenv("MIZAN_PROVIDER_TOP_UP_MINOR")
	paidTotal := os.Getenv("MIZAN_PROVIDER_TOP_UP_PAID_TOTAL_MINOR")
	if amount == "" && paidTotal == "" {
		fmt.Println("provider_funding=skipped (set both provider top-up environment variables to exercise it)")
		return nil
	}
	if amount == "" || paidTotal == "" {
		return errors.New("MIZAN_PROVIDER_TOP_UP_MINOR and MIZAN_PROVIDER_TOP_UP_PAID_TOTAL_MINOR must be set together")
	}
	paymentEventID := "example-provider-topup:" + runID
	request := mizan.NewConfirmedTopUp(mizan.ExactAmount(amount), paymentEventID, mizan.ExactAmount(paidTotal))
	response, err := client.TopUpProviderBalance(ctx, businessID, request, "provider-topup:"+businessID+":"+runID)
	if err != nil {
		return fmt.Errorf("fund provider balance: %w", err)
	}
	data, ok := response["data"].(map[string]any)
	if !ok {
		return errors.New("provider top-up response has no data object")
	}
	fmt.Printf("provider_funding=%v\n", data)
	return nil
}

func exportLedger(ctx context.Context, client *mizan.Client, businessID string) (int, int64, error) {
	var after int64
	count := 0
	for {
		response, err := client.GetLedger(ctx, businessID, after, 100)
		if err != nil {
			return count, after, fmt.Errorf("read ledger after sequence %d: %w", after, err)
		}
		page, err := mizan.DecodeData[mizan.LedgerResult](response)
		if err != nil {
			return count, after, err
		}
		count += len(page.Entries)
		if page.NextAfterSequence == nil || *page.NextAfterSequence <= after {
			return count, after, nil
		}
		after = *page.NextAfterSequence
	}
}

func describeError(err error) string {
	var apiErr *mizan.APIError
	var transportErr *mizan.TransportError
	var protocolErr *mizan.ProtocolError
	switch {
	case errors.Is(err, mizan.ErrInsufficientAzeerUnits):
		return "Azeer Units are insufficient; fund a supported catalog package, then submit the same source event with its original body and key"
	case errors.Is(err, mizan.ErrInsufficientProviderBalance):
		return "provider balance is insufficient; stop provider work and complete a trusted provider-balance top-up"
	case errors.As(err, &apiErr):
		return fmt.Sprintf("Mizan rejected the request: code=%s retryable=%t request_id=%s idempotency_key=%s details=%v",
			apiErr.Code, apiErr.Retryable, apiErr.RequestID, apiErr.IdempotencyKey, apiErr.Details)
	case errors.As(err, &transportErr):
		return fmt.Sprintf("mutation outcome is unknown; retry only the identical body and key=%s (request_id=%s): %v",
			transportErr.IdempotencyKey, transportErr.RequestID, err)
	case errors.As(err, &protocolErr):
		return fmt.Sprintf("invalid API response; preserve key=%s and investigate request_id=%s: %v",
			protocolErr.IdempotencyKey, protocolErr.RequestID, err)
	default:
		return err.Error()
	}
}

func requiredEnv(name string) string {
	value := os.Getenv(name)
	if value == "" {
		log.Fatalf("%s is required", name)
	}
	return value
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
