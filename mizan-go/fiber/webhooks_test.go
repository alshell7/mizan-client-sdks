package fiber

import (
	"context"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alshell7/mizan-client-sdks/mizan-go"
	fiberlib "github.com/gofiber/fiber/v2"
)

func TestMiddlewareMountsOnAnyFiberEndpoint(t *testing.T) {
	receiver := mizan.WebhookReceiver{Handler: mizan.WebhookHandlerFuncs{
		Ledger: func(context.Context, mizan.LedgerWebhook, mizan.WebhookContext) error { return nil },
		Notification: func(_ context.Context, event mizan.NotificationWebhook, delivery mizan.WebhookContext) error {
			if event.Type != mizan.NotificationFeaturePausedManual || delivery.OutboxID != "fiber-outbox" {
				t.Fatalf("unexpected callback values: %#v %#v", event, delivery)
			}
			return nil
		},
	}}
	app := fiberlib.New()
	app.Post("/my/company/webhooks", Middleware(receiver))
	request := httptest.NewRequest("POST", "/my/company/webhooks", strings.NewReader(`{"type":"feature_paused_manual","business_id":"business-1","feature_code":"voip"}`))
	request.Header.Set(mizan.HeaderOutboxID, "fiber-outbox")
	request.Header.Set("Content-Type", "application/json")
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	_, _ = io.ReadAll(response.Body)
	if response.StatusCode != 204 {
		t.Fatalf("expected 204, got %d", response.StatusCode)
	}
}
