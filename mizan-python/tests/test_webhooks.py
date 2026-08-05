import json
import unittest

from mizan import ACK_SEQUENCE_HEADER, OUTBOX_ID_HEADER, WebhookReceiver


def ledger_payload() -> dict:
    return {
        "event_id": "8ceae0fe-64b0-4c36-a239-c46d2a3ab777",
        "business_id": "business-1",
        "business_sequence": 42,
        "entry": {
            "id": "8ceae0fe-64b0-4c36-a239-c46d2a3ab777",
            "entry_type": "usage_consumed",
            "source_command": "consume",
            "source_event_id": "usage-1",
            "subscription_id": "subscription-1",
            "feature_code": "outbound_delivered_message",
            "effective_at": "2026-08-05T12:00:00Z",
            "catalog_version": "catalog-v1",
            "policy_version": "policy-v1",
            "metadata": {"components": ["outbound_delivered_message"]},
        },
        "postings": [
            {"rail": "azeer_units", "account_code": "azeer_units", "amount": "-1000", "unit": "milliunit"},
            {"rail": "azeer_units", "account_code": "usage:outbound_delivered_message", "amount": "1000", "unit": "milliunit"},
        ],
    }


class WebhookReceiverTests(unittest.IsolatedAsyncioTestCase):
    async def test_dispatches_ledger_and_acknowledges_only_after_success(self):
        seen = []

        async def on_ledger(event, delivery):
            self.assertNotIn("authorization", delivery.headers)
            seen.append((event["event_id"], delivery.outbox_id))

        receiver = WebhookReceiver(
            on_ledger=on_ledger,
            on_notification=lambda event, delivery: None,
            bearer_token="receiver-secret",
        )
        response = await receiver.receive(
            {"Authorization": "Bearer receiver-secret", OUTBOX_ID_HEADER: "outbox-1"},
            ledger_payload(),
        )
        self.assertEqual(response.status_code, 204)
        self.assertEqual(response.headers[ACK_SEQUENCE_HEADER], "42")
        self.assertEqual(seen, [(ledger_payload()["event_id"], "outbox-1")])

    async def test_callback_failure_returns_retryable_response_without_ack(self):
        def on_ledger(event, delivery):
            raise RuntimeError("database unavailable")

        receiver = WebhookReceiver(on_ledger=on_ledger, on_notification=lambda event, delivery: None)
        response = await receiver.receive({OUTBOX_ID_HEADER: "outbox-1"}, ledger_payload())
        self.assertEqual(response.status_code, 500)
        self.assertNotIn(ACK_SEQUENCE_HEADER, response.headers)

    async def test_notification_dispatch_and_custom_raw_json_integration(self):
        seen = []

        def on_notification(event, delivery):
            seen.append((event["type"], delivery.outbox_id, json.loads(delivery.raw_body)))

        receiver = WebhookReceiver(on_ledger=lambda event, delivery: None, on_notification=on_notification)
        payload = {
            "type": "budget_warning", "business_id": "business-1",
            "feature_code": "outbound_delivered_message", "period": "2026-08",
            "projected": "8000", "limit": "10000",
        }
        response = await receiver.receive({"X-Mizan-Outbox-Id": "notification-1"}, json.dumps(payload))
        self.assertEqual(response.status_code, 204)
        self.assertEqual(seen[0][0:2], ("budget_warning", "notification-1"))
        self.assertEqual(seen[0][2], payload)

    async def test_rejects_auth_and_unbalanced_postings(self):
        receiver = WebhookReceiver(
            on_ledger=lambda event, delivery: None,
            on_notification=lambda event, delivery: None,
            bearer_token="secret",
        )
        response = await receiver.receive({OUTBOX_ID_HEADER: "outbox-1"}, ledger_payload())
        self.assertEqual(response.status_code, 401)
        payload = ledger_payload()
        payload["postings"][1]["amount"] = "999"
        response = await receiver.receive(
            {OUTBOX_ID_HEADER: "outbox-1", "authorization": "Bearer secret"}, payload
        )
        self.assertEqual(response.status_code, 422)


if __name__ == "__main__":
    unittest.main()
