import unittest

try:
    from fastapi import FastAPI
    from fastapi.testclient import TestClient
except ImportError:  # pragma: no cover - optional dependency
    FastAPI = None
    TestClient = None

from mizan import WebhookReceiver

if FastAPI is not None:
    from mizan.fastapi import mount_webhooks
else:  # pragma: no cover - optional dependency
    mount_webhooks = None


@unittest.skipIf(FastAPI is None, "FastAPI optional dependency is not installed")
class FastAPIWebhookTests(unittest.TestCase):
    def test_mounts_receiver_on_a_custom_endpoint(self):
        seen = []
        receiver = WebhookReceiver(
            on_ledger=lambda event, delivery: None,
            on_notification=lambda event, delivery: seen.append(delivery.outbox_id),
        )
        app = FastAPI()
        mount_webhooks(app, receiver, "/my/company/webhooks")
        with TestClient(app) as client:
            response = client.post(
                "/my/company/webhooks",
                headers={"X-Mizan-Outbox-Id": "fastapi-outbox"},
                json={"type": "feature_resumed_manual", "business_id": "business-1", "feature_code": "voip"},
            )
        self.assertEqual(response.status_code, 204)
        self.assertEqual(seen, ["fastapi-outbox"])


if __name__ == "__main__":
    unittest.main()
