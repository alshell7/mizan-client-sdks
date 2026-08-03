import json
import unittest
from urllib.error import URLError

from mizan import MizanAPIError, MizanClient, MizanProtocolError, MizanTransportError, __version__


class ClientTests(unittest.TestCase):
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
