package mizan

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
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
