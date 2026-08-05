package mizan

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
)

func validLedgerWebhookJSON(t *testing.T) []byte {
	t.Helper()
	body, err := json.Marshal(LedgerWebhook{
		EventID: "8ceae0fe-64b0-4c36-a239-c46d2a3ab777", BusinessID: "business-1", BusinessSequence: 42,
		Entry: LedgerEntryWebhook{
			ID: "8ceae0fe-64b0-4c36-a239-c46d2a3ab777", EntryType: LedgerUsageConsumed,
			SourceCommand: "consume", SourceEventID: "usage-1", EffectiveAt: "2026-08-05T12:00:00Z",
			CatalogVersion: "catalog-v1", PolicyVersion: "policy-v1", Metadata: json.RawMessage(`{"components":["outbound_delivered_message"]}`),
		},
		Postings: []LedgerPostingWebhook{
			{Rail: "azeer_units", AccountCode: "azeer_units", Amount: "-1000", Unit: "milliunit"},
			{Rail: "azeer_units", AccountCode: "usage:outbound_delivered_message", Amount: "1000", Unit: "milliunit"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func TestWebhookReceiverDispatchesAndAcknowledgesLedgerAfterSuccess(t *testing.T) {
	called := false
	receiver := WebhookReceiver{
		BearerToken: "receiver-secret",
		Handler: WebhookHandlerFuncs{
			Ledger: func(_ context.Context, event LedgerWebhook, delivery WebhookContext) error {
				called = true
				if event.EventID != event.Entry.ID || delivery.OutboxID != "outbox-1" {
					t.Fatalf("unexpected event or delivery: %#v %#v", event, delivery)
				}
				if delivery.Headers.Get("Authorization") != "" {
					t.Fatal("callback headers must not expose the bearer secret")
				}
				return nil
			},
			Notification: func(context.Context, NotificationWebhook, WebhookContext) error { return nil },
		},
	}
	headers := http.Header{"Authorization": []string{"Bearer receiver-secret"}, HeaderOutboxID: []string{"outbox-1"}}
	response := receiver.Receive(context.Background(), headers, validLedgerWebhookJSON(t))
	if !called || response.StatusCode != http.StatusNoContent || response.Headers.Get(HeaderAckSequence) != "42" {
		t.Fatalf("expected successful acknowledged delivery, got %#v", response)
	}
}

func TestWebhookReceiverDoesNotAcknowledgeFailedLedgerCallback(t *testing.T) {
	receiver := WebhookReceiver{Handler: WebhookHandlerFuncs{
		Ledger:       func(context.Context, LedgerWebhook, WebhookContext) error { return errors.New("database unavailable") },
		Notification: func(context.Context, NotificationWebhook, WebhookContext) error { return nil },
	}}
	response := receiver.Receive(context.Background(), http.Header{HeaderOutboxID: []string{"outbox-1"}}, validLedgerWebhookJSON(t))
	if response.StatusCode != http.StatusInternalServerError || response.Headers.Get(HeaderAckSequence) != "" {
		t.Fatalf("failed processing must remain retryable without acknowledgement: %#v", response)
	}
}

func TestWebhookReceiverDispatchesNotificationAndExposesStableOutboxID(t *testing.T) {
	seen := ""
	receiver := WebhookReceiver{Handler: WebhookHandlerFuncs{
		Ledger: func(context.Context, LedgerWebhook, WebhookContext) error { return nil },
		Notification: func(_ context.Context, event NotificationWebhook, delivery WebhookContext) error {
			seen = delivery.OutboxID
			if event.Type != NotificationBudgetWarning || event.Projected != "8000" {
				t.Fatalf("unexpected notification: %#v", event)
			}
			return nil
		},
	}}
	body := []byte(`{"type":"budget_warning","business_id":"business-1","feature_code":"outbound_delivered_message","period":"2026-08","projected":"8000","limit":"10000"}`)
	response := receiver.Receive(context.Background(), http.Header{HeaderOutboxID: []string{"notification-outbox"}}, body)
	if response.StatusCode != http.StatusNoContent || seen != "notification-outbox" || response.Headers.Get(HeaderAckSequence) != "" {
		t.Fatalf("unexpected notification response: %#v", response)
	}
}

func TestWebhookReceiverRejectsAuthMalformedAndUnbalancedContracts(t *testing.T) {
	receiver := WebhookReceiver{BearerToken: "secret", Handler: WebhookHandlerFuncs{
		Ledger:       func(context.Context, LedgerWebhook, WebhookContext) error { return nil },
		Notification: func(context.Context, NotificationWebhook, WebhookContext) error { return nil },
	}}
	if got := receiver.Receive(context.Background(), http.Header{HeaderOutboxID: []string{"x"}}, validLedgerWebhookJSON(t)); got.StatusCode != 401 {
		t.Fatalf("expected auth rejection, got %d", got.StatusCode)
	}
	headers := http.Header{"Authorization": []string{"Bearer secret"}, HeaderOutboxID: []string{"x"}}
	var event map[string]any
	if err := json.Unmarshal(validLedgerWebhookJSON(t), &event); err != nil {
		t.Fatal(err)
	}
	postings := event["postings"].([]any)
	postings[1].(map[string]any)["amount"] = "999"
	body, _ := json.Marshal(event)
	if got := receiver.Receive(context.Background(), headers, body); got.StatusCode != 422 {
		t.Fatalf("expected invariant rejection, got %d: %s", got.StatusCode, got.Body)
	}
}
