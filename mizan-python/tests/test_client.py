import json
import unittest
from urllib.error import URLError

from mizan import MizanAdminClient, MizanAPIError, MizanClient, MizanProtocolError, MizanTransportError, __version__


class ClientTests(unittest.TestCase):
    def test_business_scoped_client_rejects_route_scope_mismatch_before_network(self):
        requests = []
        client = MizanClient(
            "https://billing.test", "business-1-secret", business_id="business-1",
            transport=lambda request, timeout: (requests.append(request) or (200, {}, b'{}')),
        )
        client.get_catalog()
        client.get_billing_summary("business-1")
        with self.assertRaises(ValueError):
            client.get_billing_summary("business-2")
        self.assertEqual(len(requests), 2)
        self.assertEqual(requests[1].get_header("X-business-id"), "business-1")

    def test_feature_convenience_method_defaults_quantity_and_provider_contract(self):
        requests = []
        client = MizanClient("https://billing.test", "secret",
            transport=lambda request, timeout: (requests.append(request) or (201, {}, b'{"data":{"accepted":true}}')))
        client.consume_outbound_delivered_message("business-1", source_event_id="msg-1",
            occurred_at="2026-08-04T00:00:00Z", idempotency_key="msg-1")
        client.consume_whatsapp_meta_marketing_message("business-1", source_event_id="wamid-1",
            occurred_at="2026-08-04T00:00:00Z", provider_event_id="wamid.1", idempotency_key="wamid-1")
        self.assertEqual(json.loads(requests[0].data)["quantity"], "1")
        self.assertEqual(json.loads(requests[1].data)["metadata"], {"provider": "Meta", "provider_event_id": "wamid.1"})

    def test_admin_client_configures_global_delivery_with_attribution(self):
        requests = []
        client = MizanAdminClient("https://admin.test", "admin-secret", actor="ops@example.com",
            transport=lambda request, timeout: (requests.append(request) or (200, {}, b'{"data":{"ready":true,"endpoints":[]}}')))
        client.configure_global_delivery_endpoint("ledger", {
            "endpoint_url": "https://ledger.example/events", "auth_type": "none", "enabled": True,
            "reason": "Production ledger receiver",
        }, idempotency_key="global-ledger-v1")
        self.assertTrue(requests[0].full_url.endswith("/admin/api/delivery-endpoints/ledger"))
        self.assertEqual(requests[0].get_header("X-admin-actor"), "ops@example.com")
        self.assertIsNone(requests[0].get_header("X-admin-role"))
        self.assertEqual(requests[0].get_header("Idempotency-key"), "global-ledger-v1")

    def test_mutation_retries_with_same_idempotency_key(self):
        requests = []

        def transport(request, timeout):
            requests.append(request)
            if len(requests) == 1:
                raise URLError("temporary")
            return 201, {}, json.dumps({"data": {"accepted": True}}).encode()

        client = MizanClient("https://billing.test", "secret", transport=transport, max_attempts=2)
        result = client.consume(
            "business-1",
            {"source_event_id": "event-1", "occurred_at": "2026-08-03T00:00:00Z", "feature_code": "outbound_delivered_message"},
            idempotency_key="usage-event-1",
        )
        self.assertTrue(result["data"]["accepted"])
        self.assertEqual(requests[0].get_header("Idempotency-key"), "usage-event-1")
        self.assertEqual(requests[1].get_header("Idempotency-key"), "usage-event-1")

    def test_stable_api_error_is_typed_and_not_retried(self):
        calls = 0

        def transport(request, timeout):
            nonlocal calls
            calls += 1
            return 422, {}, json.dumps({"error": {"code": "INSUFFICIENT_PROVIDER_BALANCE", "message": "empty", "retryable": False, "details": {"available_minor": "0"}}}).encode()

        client = MizanClient("https://billing.test", "secret", transport=transport)
        with self.assertRaises(MizanAPIError) as raised:
            client.top_up_provider_balance("business-1", {"amount_minor": "1", "payment_event_id": "p", "payment_status": "confirmed", "currency": "SAR", "paid_total_minor": "1"})
        self.assertEqual(raised.exception.code, "INSUFFICIENT_PROVIDER_BALANCE")
        self.assertEqual(raised.exception.details["available_minor"], "0")
        self.assertEqual(calls, 1)

    def test_catalog_and_entitlement_are_read_only_requests(self):
        requests = []

        def transport(request, timeout):
            requests.append(request)
            return 200, {}, json.dumps({"data": {"enabled": True}}).encode()

        client = MizanClient("https://billing.test", "secret", transport=transport)
        client.get_catalog()
        client.get_entitlement("business-1", "rbac_audit")
        self.assertTrue(requests[0].full_url.endswith("/v1/catalog"))
        self.assertTrue(requests[1].full_url.endswith("/v1/businesses/business-1/entitlements/rbac_audit"))
        self.assertIsNone(requests[0].get_header("Idempotency-key"))

    def test_balance_preview_and_admin_pagination_are_read_only(self):
        requests = []
        transport = lambda request, timeout: (requests.append(request) or (200, {}, b'{"data":{}}'))
        client = MizanClient("https://billing.test", "secret", transport=transport)
        client.preview_balance_impact("business-1", {
            "operation": "top_up_provider_balance", "request": {"amount_minor": "1000"},
        })
        admin = MizanAdminClient("https://admin.test", "admin-secret", actor="ops@example.com", transport=transport)
        admin.list_usage_decisions("business-1", offset=25, limit=25)
        admin.configure_addon("voice_broadcast", {
            "rollout_stage": "pilot", "enabled": True, "reason": "Pilot",
        }, idempotency_key="addon-pilot")
        self.assertTrue(requests[0].full_url.endswith("/v1/businesses/business-1/balance-impact-preview"))
        self.assertIsNone(requests[0].get_header("Idempotency-key"))
        self.assertIn("/admin/api/businesses/business-1/usage-decisions?", requests[1].full_url)
        self.assertIsNone(requests[1].get_header("Idempotency-key"))
        self.assertTrue(requests[2].full_url.endswith("/admin/api/addons/voice_broadcast"))
        self.assertEqual(requests[2].get_header("Idempotency-key"), "addon-pilot")

    def test_custom_plan_activation_serializes_the_immutable_configuration_id(self):
        requests = []
        client = MizanClient(
            "https://billing.test", "secret",
            transport=lambda request, timeout: (requests.append(request) or (201, {}, b'{"data":{}}')),
        )
        client.activate_subscription("business-1", {
            "catalog_version": "azeer-2026-08-03-v2",
            "plan_configuration_id": "plan_cfg_1",
            "term": "monthly", "seats": 10, "payment_status": "confirmed",
            "payment_event_id": "pay_1", "currency": "SAR", "paid_total_minor": "1000",
        }, idempotency_key="activate-custom")
        body = json.loads(requests[0].data)
        self.assertEqual(body["plan_configuration_id"], "plan_cfg_1")
        self.assertNotIn("plan_id", body)

    def test_generated_idempotency_key_is_reused_and_exposed_after_unknown_outcome(self):
        requests = []

        def transport(request, timeout):
            requests.append(request)
            raise URLError("network down")

        client = MizanClient("https://billing.test", "secret", transport=transport, max_attempts=2)
        with self.assertRaises(MizanTransportError) as raised:
            client.consume("business-1", {
                "source_event_id": "event-1", "occurred_at": "2026-08-03T00:00:00Z",
                "feature_code": "outbound_delivered_message", "quantity": "1",
            })
        keys = [request.get_header("Idempotency-key") for request in requests]
        self.assertEqual(len(requests), 2)
        self.assertTrue(keys[0])
        self.assertEqual(keys[0], keys[1])
        self.assertEqual(raised.exception.idempotency_key, keys[0])
        self.assertTrue(raised.exception.request_id)

    def test_read_transport_failure_is_not_retried(self):
        calls = 0

        def transport(request, timeout):
            nonlocal calls
            calls += 1
            raise URLError("offline")

        client = MizanClient("https://billing.test", "secret", transport=transport, max_attempts=3)
        with self.assertRaises(MizanTransportError) as raised:
            client.get_catalog()
        self.assertEqual(calls, 1)
        self.assertIsNone(raised.exception.idempotency_key)

    def test_retryable_api_error_reuses_identical_body_headers_and_key(self):
        requests = []

        def transport(request, timeout):
            requests.append(request)
            if len(requests) == 1:
                return 503, {}, json.dumps({"error": {"code": "INTERNAL_RETRYABLE", "message": "retry", "retryable": True}}).encode()
            return 201, {}, json.dumps({"data": {"accepted": True}}).encode()

        client = MizanClient("https://billing.test", "secret", transport=transport, max_attempts=2)
        client.consume("business-1", {
            "source_event_id": "event-1", "occurred_at": "2026-08-03T00:00:00Z",
            "feature_code": "outbound_delivered_message", "quantity": "1",
        }, idempotency_key="stable")
        self.assertEqual(requests[0].data, requests[1].data)
        self.assertEqual(requests[0].get_header("Idempotency-key"), requests[1].get_header("Idempotency-key"))
        self.assertEqual(requests[0].get_header("X-request-id"), requests[1].get_header("X-request-id"))

    def test_protocol_errors_include_recovery_identifiers(self):
        for raw in (b"{", b"[]", b"x" * 2_097_153):
            with self.subTest(size=len(raw)):
                client = MizanClient("https://billing.test", "secret", transport=lambda request, timeout: (201, {}, raw))
                with self.assertRaises(MizanProtocolError) as raised:
                    client.consume("business-1", {
                        "source_event_id": "event-1", "occurred_at": "2026-08-03T00:00:00Z",
                        "feature_code": "outbound_delivered_message", "quantity": "1",
                    }, idempotency_key="protocol-key")
                self.assertEqual(raised.exception.idempotency_key, "protocol-key")
                self.assertTrue(raised.exception.request_id)

    def test_api_error_reads_request_id_case_insensitively_and_exposes_key(self):
        def transport(request, timeout):
            return 409, {"X-Request-ID": "server-request"}, json.dumps({
                "error": {"code": "IDEMPOTENCY_KEY_REUSED", "message": "changed", "retryable": False}
            }).encode()

        client = MizanClient("https://billing.test", "secret", transport=transport)
        with self.assertRaises(MizanAPIError) as raised:
            client.cancel_subscription("business-1", {}, idempotency_key="cancel-key")
        self.assertEqual(raised.exception.request_id, "server-request")
        self.assertEqual(raised.exception.idempotency_key, "cancel-key")

    def test_urls_are_escaped_and_ledger_pagination_is_validated(self):
        requests = []

        def transport(request, timeout):
            requests.append(request)
            return 200, {}, b"{}"

        client = MizanClient("https://billing.test/root", "secret", transport=transport)
        client.get_entitlement("business:one", "capability/with slash")
        client.get_ledger("business:one", after_sequence=12, limit=100)
        self.assertIn("capability%2Fwith%20slash", requests[0].full_url)
        self.assertTrue(requests[1].full_url.endswith("after_sequence=12&limit=100"))
        for after, limit in ((-1, 1), (0, 0), (0, 101)):
            with self.assertRaises(ValueError):
                client.get_ledger("business-1", after_sequence=after, limit=limit)

    def test_configuration_and_standard_headers_fail_safe(self):
        for base_url in ("", "billing.test", "ftp://billing.test", "https://billing.test?token=x"):
            with self.subTest(base_url=base_url), self.assertRaises(ValueError):
                MizanClient(base_url, "secret")
        with self.assertRaises(ValueError):
            MizanClient("https://billing.test", "")
        captured = []
        client = MizanClient("https://billing.test", "secret", transport=lambda request, timeout: (captured.append(request) or (200, {}, b"{}")))
        client.get_catalog()
        self.assertEqual(captured[0].get_header("Authorization"), "Bearer secret")
        self.assertEqual(captured[0].get_header("User-agent"), f"mizan-python/{__version__}")
        self.assertTrue(captured[0].get_header("X-request-timestamp"))

    def test_logger_receives_operational_metadata_but_not_the_token(self):
        events = []
        client = MizanClient(
            "https://billing.test", "super-secret",
            transport=lambda request, timeout: (200, {}, b"{}"),
            logger=lambda event, fields: events.append((event, fields)),
        )
        client.get_catalog()
        self.assertEqual(events[0][0], "request_complete")
        self.assertNotIn("super-secret", json.dumps(events))


if __name__ == "__main__":
    unittest.main()
