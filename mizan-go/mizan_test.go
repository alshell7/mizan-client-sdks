package mizan

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) { return f(request) }

func TestConsumeRetriesWithSameIdempotencyKey(t *testing.T) {
	var mu sync.Mutex
	var keys []string
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		calls++
		keys = append(keys, r.Header.Get("Idempotency-Key"))
		current := calls
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if current == 1 {
			w.WriteHeader(503)
			json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"code": "INTERNAL_RETRYABLE", "message": "retry", "retryable": true}})
			return
		}
		w.WriteHeader(201)
		json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"accepted": true}})
	}))
	defer server.Close()
	client, _ := NewClient(server.URL, "secret")
	client.MaxAttempts = 2
	_, err := client.Consume(context.Background(), "business-1", ConsumptionRequest{
		SourceEventID: "event-1", OccurredAt: time.Now().UTC(), FeatureCode: "outbound_delivered_message", Quantity: "1",
	}, "usage-event-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 2 || keys[0] != "usage-event-1" || keys[1] != "usage-event-1" {
		t.Fatalf("idempotency keys changed: %#v", keys)
	}
}

func TestNonRetryableAPIError(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(422)
		json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"code": "INSUFFICIENT_PROVIDER_BALANCE", "message": "empty", "retryable": false, "details": map[string]any{"available_minor": "0"}}})
	}))
	defer server.Close()
	client, _ := NewClient(server.URL, "secret")
	_, err := client.TopUpProviderBalance(context.Background(), "business-1", ConfirmedTopUp{AmountMinor: "1", PaymentEventID: "p", PaymentStatus: "confirmed", Currency: "SAR", PaidTotalMinor: "1"}, "topup-1")
	apiErr, ok := err.(*APIError)
	if !ok || apiErr.Code != "INSUFFICIENT_PROVIDER_BALANCE" || calls != 1 {
		t.Fatalf("unexpected error=%#v calls=%d", err, calls)
	}
}

func TestCatalogAndEntitlementPaths(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"enabled": true}})
	}))
	defer server.Close()
	client, _ := NewClient(server.URL, "secret")
	if _, err := client.GetCatalog(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := client.GetEntitlement(context.Background(), "business-1", "rbac_audit"); err != nil {
		t.Fatal(err)
	}
	if len(paths) != 2 || paths[0] != "/v1/catalog" || paths[1] != "/v1/businesses/business-1/entitlements/rbac_audit" {
		t.Fatalf("unexpected paths: %#v", paths)
	}
}

func TestGeneratedIdempotencyKeyIsStableAndRecoverable(t *testing.T) {
	var keys, requestIDs []string
	calls := 0
	client, _ := NewClient("https://billing.test", "secret")
	client.MaxAttempts = 2
	client.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		calls++
		keys = append(keys, request.Header.Get("Idempotency-Key"))
		requestIDs = append(requestIDs, request.Header.Get("X-Request-ID"))
		return nil, errors.New("offline")
	})}
	_, err := client.Consume(context.Background(), "business-1", ConsumptionRequest{
		SourceEventID: "event-1", OccurredAt: time.Now().UTC(), FeatureCode: "outbound_delivered_message", Quantity: "1",
	}, "")
	var transportErr *TransportError
	if !errors.As(err, &transportErr) {
		t.Fatalf("expected TransportError, got %#v", err)
	}
	if calls != 2 || keys[0] == "" || keys[0] != keys[1] || requestIDs[0] != requestIDs[1] {
		t.Fatalf("unstable retry metadata: keys=%#v requestIDs=%#v", keys, requestIDs)
	}
	if transportErr.IdempotencyKey != keys[0] || transportErr.RequestID != requestIDs[0] {
		t.Fatalf("missing recovery metadata: %#v", transportErr)
	}
}

func TestReadTransportFailureIsNotRetried(t *testing.T) {
	calls := 0
	client, _ := NewClient("https://billing.test", "secret")
	client.MaxAttempts = 3
	client.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		calls++
		return nil, errors.New("offline")
	})}
	_, err := client.GetCatalog(context.Background())
	var transportErr *TransportError
	if !errors.As(err, &transportErr) || calls != 1 || transportErr.IdempotencyKey != "" {
		t.Fatalf("unexpected error=%#v calls=%d", err, calls)
	}
}

func TestProtocolErrorsAreBoundedAndRecoverable(t *testing.T) {
	for _, body := range []string{"[1]", "{", strings.Repeat("x", (2<<20)+1)} {
		t.Run(fmt.Sprintf("bytes-%d", len(body)), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(201)
				_, _ = w.Write([]byte(body))
			}))
			defer server.Close()
			client, _ := NewClient(server.URL, "secret")
			_, err := client.Consume(context.Background(), "business-1", ConsumptionRequest{
				SourceEventID: "event-1", OccurredAt: time.Now().UTC(), FeatureCode: "outbound_delivered_message", Quantity: "1",
			}, "protocol-key")
			var protocolErr *ProtocolError
			if !errors.As(err, &protocolErr) || protocolErr.IdempotencyKey != "protocol-key" || protocolErr.RequestID == "" {
				t.Fatalf("unexpected protocol error: %#v", err)
			}
		})
	}
}

func TestAPIErrorUsesHeaderRequestIDAndExposesKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Request-ID", "server-request")
		w.WriteHeader(409)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{
			"code": "IDEMPOTENCY_KEY_REUSED", "message": "changed", "retryable": false,
		}})
	}))
	defer server.Close()
	client, _ := NewClient(server.URL, "secret")
	_, err := client.CancelSubscription(context.Background(), "business-1", CancellationRequest{}, "cancel-key")
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.RequestID != "server-request" || apiErr.IdempotencyKey != "cancel-key" {
		t.Fatalf("unexpected API error: %#v", err)
	}
}

func TestDecodeDataPreservesExactAmounts(t *testing.T) {
	response := Response{"data": map[string]any{
		"accepted": true,
		"code":     "ACCEPTED",
		"balances": map[string]any{"azeer_unit_millis": "9223372036854775807", "provider_balance_minor": "75"},
	}}
	result, err := DecodeData[ConsumptionResult](response)
	if err != nil {
		t.Fatal(err)
	}
	if result.Balances.AzeerUnitMillis != "9223372036854775807" || result.Balances.ProviderBalanceMinor != "75" {
		t.Fatalf("precision changed: %#v", result.Balances)
	}
	if _, err := DecodeData[ConsumptionResult](Response{}); err == nil {
		t.Fatal("expected missing data error")
	}
}

func TestConfigurationURLsPaginationAndHeaders(t *testing.T) {
	for _, raw := range []string{"", "billing.test", "ftp://billing.test", "https://billing.test?q=x", "https://user@billing.test"} {
		if _, err := NewClient(raw, "secret"); err == nil {
			t.Fatalf("expected invalid URL error for %q", raw)
		}
	}
	if _, err := NewClient("https://billing.test", ""); err == nil {
		t.Fatal("expected empty token error")
	}
	var requestURI string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestURI = r.RequestURI
		if r.Header.Get("Authorization") != "Bearer secret" || r.Header.Get("User-Agent") != "mizan-go/"+Version || r.Header.Get("X-Request-Timestamp") == "" {
			t.Errorf("missing standard headers: %#v", r.Header)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{})
	}))
	defer server.Close()
	client, _ := NewClient(server.URL, "secret")
	if _, err := client.GetEntitlement(context.Background(), "business:one", "capability/with slash"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(requestURI, "capability%2Fwith%20slash") {
		t.Fatalf("path was not escaped: %s", requestURI)
	}
	for _, input := range []struct {
		after int64
		limit int
	}{{-1, 1}, {0, 0}, {0, 101}} {
		if _, err := client.GetLedger(context.Background(), "business-1", input.after, input.limit); err == nil {
			t.Fatalf("expected pagination error for %#v", input)
		}
	}
}

func TestContractVocabularyAndBuilders(t *testing.T) {
	if got := AllFeatureCodes(); len(got) != 9 || got[1] != FeatureOutboundDeliveredMessage {
		t.Fatalf("unexpected feature vocabulary: %#v", got)
	}
	topUp := NewConfirmedTopUp("100", "pay_1", "115")
	if topUp.PaymentStatus != PaymentConfirmed || topUp.Currency != CurrencySAR {
		t.Fatalf("fixed protocol values were not populated: %#v", topUp)
	}
	budget := NewBudget(BudgetQuantity, "10", BudgetPause)
	if budget.Period != BudgetSubscriptionMonth || budget.WarningBPS != 8000 {
		t.Fatalf("budget defaults were not populated: %#v", budget)
	}
	apiErr := &APIError{Code: ErrCodeInsufficientProviderBalance}
	if !errors.Is(apiErr, ErrInsufficientProviderBalance) {
		t.Fatal("typed API error did not match its domain sentinel")
	}
}

func TestCustomPlanActivationJSONUsesConfigurationID(t *testing.T) {
	body, err := json.Marshal(ActivationRequest{
		CatalogVersion: "azeer-2026-08-03-v2", PlanConfigurationID: "plan_cfg_1",
		Term: TermMonthly, Seats: 10, PaymentStatus: PaymentConfirmed,
		PaymentEventID: "pay_1", Currency: CurrencySAR, PaidTotalMinor: "1000",
	})
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["plan_configuration_id"] != "plan_cfg_1" {
		t.Fatalf("missing plan configuration: %s", body)
	}
	if _, exists := decoded["plan_id"]; exists {
		t.Fatalf("empty template plan should not be serialized: %s", body)
	}
}

func TestFeatureSpecificConsumptionContracts(t *testing.T) {
	var bodies []map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		bodies = append(bodies, body)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"data":{"accepted":true}}`))
	}))
	defer server.Close()
	client, _ := NewClient(server.URL, "secret")
	now := time.Date(2026, 8, 4, 0, 0, 0, 0, time.UTC)
	if _, err := client.ConsumeConversation24H(context.Background(), "business-1", Conversation24HUsage{
		SourceEventID: "conversation-1", OccurredAt: now,
	}, "conversation-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := client.ConsumeOutboundDeliveredMessage(context.Background(), "business-1", OutboundDeliveredMessageUsage{
		SourceEventID: "msg-1", OccurredAt: now,
	}, "msg-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := client.ConsumeAIAssistActionOverAllowance(context.Background(), "business-1", AIAssistActionOverAllowanceUsage{
		SourceEventID: "assist-1", OccurredAt: now,
	}, "assist-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := client.ConsumeVoiceAIStartedMinute(context.Background(), "business-1", VoiceAIStartedMinuteUsage{
		SourceEventID: "voice-ai-1", OccurredAt: now, DurationSeconds: "61",
	}, "voice-ai-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := client.ConsumeAIReplyHandling(context.Background(), "business-1", AIReplyHandlingUsage{
		SourceEventID: "reply-1", OccurredAt: now,
	}, "reply-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := client.ConsumeWhatsAppMetaMarketingMessage(context.Background(), "business-1", WhatsAppMetaMarketingMessageUsage{
		SourceEventID: "wamid-1", OccurredAt: now, ProviderEventID: "wamid.1",
		Metadata: &UsageMetadata{Provider: "wrong", ProviderEventID: "wrong"},
	}, "wamid-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := client.ConsumeTelephonyVoiceMinute(context.Background(), "business-1", TelephonyVoiceMinuteUsage{
		SourceEventID: "call-1", OccurredAt: now, Provider: "Twilio", ProviderEventID: "CA1",
	}, "call-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := client.ConsumeInboundVoiceMinute(context.Background(), "business-1", InboundVoiceMinuteUsage{
		SourceEventID: "inbound-1", OccurredAt: now, Provider: "Carrier", ProviderEventID: "IN1",
		BillableMinutes: "1.250",
	}, "inbound-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := client.ConsumeOtherProviderCharge(context.Background(), "business-1", OtherProviderChargeUsage{
		SourceEventID: "fee-1", OccurredAt: now, Provider: "Carrier", ProviderEventID: "INV1",
		ProviderAmountMinor: "0",
	}, "fee-1"); err != nil {
		t.Fatal(err)
	}
	wantFeatures := AllFeatureCodes()
	wantCodes := make([]string, len(wantFeatures))
	for index, feature := range wantFeatures {
		wantCodes[index] = string(feature)
	}
	if len(bodies) != len(wantCodes) {
		t.Fatalf("got %d requests, want %d", len(bodies), len(wantCodes))
	}
	for index, code := range wantCodes {
		if bodies[index]["feature_code"] != code {
			t.Fatalf("request %d feature = %#v, want %s", index, bodies[index]["feature_code"], code)
		}
	}
	for _, index := range []int{0, 1, 2, 4, 5, 6} {
		if bodies[index]["quantity"] != "1" {
			t.Fatalf("request %d quantity did not default to one: %#v", index, bodies[index])
		}
	}
	if bodies[3]["duration_seconds"] != "61" || bodies[3]["quantity"] != nil {
		t.Fatalf("started-minute wire contract drifted: %#v", bodies[3])
	}
	if bodies[7]["quantity"] != "1.250" {
		t.Fatalf("provider-normalized minutes drifted: %#v", bodies[7])
	}
	if bodies[8]["provider_amount_minor"] != "0" || bodies[8]["quantity"] != nil {
		t.Fatalf("provider amount wire contract drifted: %#v", bodies[8])
	}
	metadata := bodies[5]["metadata"].(map[string]any)
	if metadata["provider"] != "Meta" || metadata["provider_event_id"] != "wamid.1" {
		t.Fatalf("provider attribution missing: %#v", metadata)
	}
}

func TestFeatureSpecificConsumptionRejectsInvalidInputsBeforeHTTP(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"data":{"accepted":true}}`))
	}))
	defer server.Close()
	client, _ := NewClient(server.URL, "secret")
	ctx := context.Background()
	now := time.Date(2026, 8, 4, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name string
		call func() error
	}{
		{"zero count", func() error {
			_, err := client.ConsumeConversation24H(ctx, "business-1", Conversation24HUsage{SourceEventID: "x", OccurredAt: now, Quantity: "0"}, "x")
			return err
		}},
		{"too precise", func() error {
			_, err := client.ConsumeOutboundDeliveredMessage(ctx, "business-1", OutboundDeliveredMessageUsage{SourceEventID: "x", OccurredAt: now, Quantity: "1.0001"}, "x")
			return err
		}},
		{"missing source", func() error {
			_, err := client.ConsumeAIReplyHandling(ctx, "business-1", AIReplyHandlingUsage{OccurredAt: now}, "x")
			return err
		}},
		{"missing time", func() error {
			_, err := client.ConsumeAIAssistActionOverAllowance(ctx, "business-1", AIAssistActionOverAllowanceUsage{SourceEventID: "x"}, "x")
			return err
		}},
		{"zero duration", func() error {
			_, err := client.ConsumeVoiceAIStartedMinute(ctx, "business-1", VoiceAIStartedMinuteUsage{SourceEventID: "x", OccurredAt: now, DurationSeconds: "0"}, "x")
			return err
		}},
		{"fractional duration", func() error {
			_, err := client.ConsumeVoiceAIStartedMinute(ctx, "business-1", VoiceAIStartedMinuteUsage{SourceEventID: "x", OccurredAt: now, DurationSeconds: "1.5"}, "x")
			return err
		}},
		{"missing meta event", func() error {
			_, err := client.ConsumeWhatsAppMetaMarketingMessage(ctx, "business-1", WhatsAppMetaMarketingMessageUsage{SourceEventID: "x", OccurredAt: now}, "x")
			return err
		}},
		{"missing carrier", func() error {
			_, err := client.ConsumeTelephonyVoiceMinute(ctx, "business-1", TelephonyVoiceMinuteUsage{SourceEventID: "x", OccurredAt: now, ProviderEventID: "call"}, "x")
			return err
		}},
		{"negative provider amount", func() error {
			_, err := client.ConsumeOtherProviderCharge(ctx, "business-1", OtherProviderChargeUsage{SourceEventID: "x", OccurredAt: now, Provider: "carrier", ProviderEventID: "fee", ProviderAmountMinor: "-1"}, "x")
			return err
		}},
		{"overflow provider amount", func() error {
			_, err := client.ConsumeOtherProviderCharge(ctx, "business-1", OtherProviderChargeUsage{SourceEventID: "x", OccurredAt: now, Provider: "carrier", ProviderEventID: "fee", ProviderAmountMinor: "9223372036854775808"}, "x")
			return err
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.call(); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
	if requests != 0 {
		t.Fatalf("invalid contracts issued %d HTTP requests", requests)
	}
}

func TestFeatureExactBoundariesMatchWorkerContract(t *testing.T) {
	for _, quantity := range []ExactAmount{"0.001", "1", "9223372036854775.807"} {
		if err := validateQuantity(quantity); err != nil {
			t.Fatalf("valid quantity %s rejected: %v", quantity, err)
		}
	}
	for _, quantity := range []ExactAmount{"0", "-1", "1.0001", "9223372036854775.808"} {
		if err := validateQuantity(quantity); err == nil {
			t.Fatalf("invalid quantity %s accepted", quantity)
		}
	}
	if err := validateExactInteger("9223372036854775807", "amount", true); err != nil {
		t.Fatalf("maximum exact amount rejected: %v", err)
	}
	if err := validateExactInteger("9223372036854775808", "amount", true); err == nil {
		t.Fatal("overflowing exact amount accepted")
	}
}

func TestFeatureREADMEReferencesRealContractsAndMethods(t *testing.T) {
	contents, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatal(err)
	}
	readme := string(contents)
	rows := [][]string{
		{"conversation_24h", "Conversation24HUsage", "ConsumeConversation24H"},
		{"outbound_delivered_message", "OutboundDeliveredMessageUsage", "ConsumeOutboundDeliveredMessage"},
		{"ai_assist_action_over_allowance", "AIAssistActionOverAllowanceUsage", "ConsumeAIAssistActionOverAllowance"},
		{"voice_ai_started_minute", "VoiceAIStartedMinuteUsage", "ConsumeVoiceAIStartedMinute"},
		{"ai_reply_handling", "AIReplyHandlingUsage", "ConsumeAIReplyHandling"},
		{"whatsapp_meta_marketing_msg", "WhatsAppMetaMarketingMessageUsage", "ConsumeWhatsAppMetaMarketingMessage"},
		{"telephony_voice_minute", "TelephonyVoiceMinuteUsage", "ConsumeTelephonyVoiceMinute"},
		{"inbound_voice_minute", "InboundVoiceMinuteUsage", "ConsumeInboundVoiceMinute"},
		{"other_provider_charge", "OtherProviderChargeUsage", "ConsumeOtherProviderCharge"},
	}
	for _, row := range rows {
		for _, symbol := range row {
			if !strings.Contains(readme, "`"+symbol+"`") {
				t.Fatalf("README does not document %s for %s", symbol, row[0])
			}
		}
	}
	_ = []any{Conversation24HUsage{}, OutboundDeliveredMessageUsage{}, AIAssistActionOverAllowanceUsage{},
		VoiceAIStartedMinuteUsage{}, AIReplyHandlingUsage{}, WhatsAppMetaMarketingMessageUsage{},
		TelephonyVoiceMinuteUsage{}, InboundVoiceMinuteUsage{}, OtherProviderChargeUsage{}}
	_ = []any{(*Client).ConsumeConversation24H, (*Client).ConsumeOutboundDeliveredMessage,
		(*Client).ConsumeAIAssistActionOverAllowance, (*Client).ConsumeVoiceAIStartedMinute,
		(*Client).ConsumeAIReplyHandling, (*Client).ConsumeWhatsAppMetaMarketingMessage,
		(*Client).ConsumeTelephonyVoiceMinute, (*Client).ConsumeInboundVoiceMinute,
		(*Client).ConsumeOtherProviderCharge}
}

func TestAdminClientDeliveryConfiguration(t *testing.T) {
	var path, actor, key string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path, actor, key = r.URL.Path, r.Header.Get("X-Admin-Actor"), r.Header.Get("Idempotency-Key")
		_, _ = w.Write([]byte(`{"data":{"ready":true,"endpoints":[]}}`))
	}))
	defer server.Close()
	client, err := NewAdminClient(server.URL, "admin-secret", "ops@example.com")
	if err != nil {
		t.Fatal(err)
	}
	enabled := true
	_, err = client.ConfigureGlobalDeliveryEndpoint(context.Background(), "ledger", DeliveryEndpointInput{
		EndpointURL: "https://ledger.example/events", AuthType: "none", Enabled: &enabled,
		Reason: "Production ledger receiver",
	}, "global-ledger-v1")
	if err != nil {
		t.Fatal(err)
	}
	if path != "/admin/api/delivery-endpoints/ledger" || actor != "ops@example.com" || key != "global-ledger-v1" {
		t.Fatalf("unexpected admin request path=%q actor=%q key=%q", path, actor, key)
	}
}
