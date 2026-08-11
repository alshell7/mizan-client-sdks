import json
import math
import unittest
from pathlib import Path

import mizan
from mizan import (
    Channel,
    FeatureCode,
    MizanClient,
    ai_assist_action,
    ai_assist_action_over_allowance,
    ai_reply_handling,
    conversation_24h,
    inbound_voice_minute,
    other_provider_charge,
    outbound_delivered_message,
    telephony_voice_minute,
    values,
    voice_ai_started_minute,
    whatsapp_meta_marketing_message,
)

NOW = "2026-08-04T00:00:00Z"


def conversation(**overrides):
    values = {"source_event_id": "conversation-1", "occurred_at": NOW,
              "conversation_id": "conversation-1", "channel": Channel.WHATSAPP}
    values.update(overrides)
    return conversation_24h(**values)


def pass_through(**overrides):
    values = {"source_event_id": "provider-fee-1", "occurred_at": NOW,
              "provider": "Carrier", "provider_event_id": "provider-event-1",
              "provider_amount_minor": "337", "provider_invoice_id": "INV-1",
              "original_amount_minor": "337", "original_currency": "SAR",
              "tariff_version": "carrier-v1"}
    values.update(overrides)
    return other_provider_charge(**values)


class FeatureBuilderContractTests(unittest.TestCase):
    def test_billing_summary_exports_server_effective_lifecycle_contract(self):
        self.assertTrue({"business_id", "as_of", "subscription", "delivery"}
                        <= mizan.BillingSummaryResult.__required_keys__)
        self.assertTrue({"status", "effective_status", "current_period_start", "current_period_end"}
                        <= mizan.SubscriptionSummary.__required_keys__)

    def test_every_feature_code_has_a_named_exported_contract_and_builder(self):
        cases = [
            ("conversation_24h", "Conversation24HConsumptionRequest",
             conversation),
            ("outbound_delivered_message", "OutboundDeliveredMessageConsumptionRequest",
             lambda: outbound_delivered_message(source_event_id="message-1", occurred_at=NOW)),
            ("ai_assist_action_over_allowance", "AIAssistActionConsumptionRequest",
             lambda: ai_assist_action(source_event_id="assist-1", occurred_at=NOW)),
            ("ai_reply_handling", "AIReplyHandlingConsumptionRequest",
             lambda: ai_reply_handling(source_event_id="reply-1", occurred_at=NOW)),
            ("voice_ai_started_minute", "VoiceAIStartedMinuteConsumptionRequest",
             lambda: voice_ai_started_minute(source_event_id="voice-1", occurred_at=NOW, duration_seconds="61")),
            ("whatsapp_meta_marketing_msg", "WhatsAppMetaMarketingMessageConsumptionRequest",
             lambda: whatsapp_meta_marketing_message(source_event_id="meta-1", occurred_at=NOW,
                                                       provider_event_id="wamid.1")),
            ("telephony_voice_minute", "TelephonyVoiceMinuteConsumptionRequest",
             lambda: telephony_voice_minute(source_event_id="call-1", occurred_at=NOW,
                                             provider="Twilio", provider_event_id="CA1")),
            ("inbound_voice_minute", "InboundVoiceMinuteConsumptionRequest",
             lambda: inbound_voice_minute(source_event_id="inbound-1", occurred_at=NOW,
                                           provider="Carrier", provider_event_id="IN1")),
            ("other_provider_charge", "OtherProviderChargeConsumptionRequest",
             pass_through),
        ]
        self.assertEqual({item[0] for item in cases}, set(values(FeatureCode)))
        for feature_code, contract_name, build in cases:
            with self.subTest(feature_code=feature_code):
                self.assertTrue(hasattr(mizan, contract_name), f"{contract_name} is not exported")
                self.assertIn(contract_name, mizan.__all__)
                request = build()
                self.assertEqual(request["feature_code"], feature_code)
                self.assertEqual(request["occurred_at"], NOW)

    def test_quantity_defaults_only_on_quantity_based_contracts(self):
        requests = [
            conversation(source_event_id="a"),
            outbound_delivered_message(source_event_id="b", occurred_at=NOW),
            ai_assist_action_over_allowance(source_event_id="c", occurred_at=NOW),
            ai_reply_handling(source_event_id="d", occurred_at=NOW),
            whatsapp_meta_marketing_message(source_event_id="e", occurred_at=NOW, provider_event_id="meta-e"),
            telephony_voice_minute(source_event_id="f", occurred_at=NOW, provider="Twilio", provider_event_id="call-f"),
            inbound_voice_minute(source_event_id="g", occurred_at=NOW, provider="Carrier", provider_event_id="call-g"),
        ]
        self.assertTrue(all(request["quantity"] == "1" for request in requests))
        voice = voice_ai_started_minute(source_event_id="h", occurred_at=NOW, duration_seconds="1")
        provider = pass_through(source_event_id="i", provider_event_id="fee-i",
                                provider_amount_minor="0", original_amount_minor="0")
        self.assertNotIn("quantity", voice)
        self.assertNotIn("quantity", provider)

    def test_provider_contracts_enforce_and_normalize_attribution(self):
        caller_metadata = {"provider": "wrong", "provider_event_id": "wrong", "provider_invoice_id": "INV-1"}
        meta = whatsapp_meta_marketing_message(source_event_id="meta", occurred_at=NOW,
                                                provider_event_id="wamid.real", metadata=caller_metadata)
        self.assertEqual(meta["metadata"]["provider"], "Meta")
        self.assertEqual(meta["metadata"]["provider_event_id"], "wamid.real")
        self.assertEqual(meta["metadata"]["provider_invoice_id"], "INV-1")
        self.assertEqual(caller_metadata["provider"], "wrong", "builder mutated the caller's metadata")

        telephony = telephony_voice_minute(source_event_id="call", occurred_at=NOW, provider=" Twilio ",
                                           provider_event_id=" CA123 ", billable_minutes="1.250")
        self.assertEqual(telephony["quantity"], "1.250")
        self.assertEqual(telephony["metadata"]["provider"], "Twilio")
        self.assertEqual(telephony["metadata"]["provider_event_id"], "CA123")

    def test_engine_owned_conversation_windows_and_ai_allowance_inputs(self):
        request = conversation(metadata={"conversation_id": "wrong", "channel": Channel.TIKTOK})
        self.assertEqual(request["quantity"], "1")
        self.assertEqual(request["metadata"]["conversation_id"], "conversation-1")
        self.assertEqual(request["metadata"]["channel"], Channel.WHATSAPP)
        self.assertEqual(ai_assist_action(source_event_id="assist", occurred_at=NOW),
                         ai_assist_action_over_allowance(source_event_id="assist", occurred_at=NOW))
        self.assertIs(mizan.AIAssistActionConsumptionRequest,
                      mizan.AIAssistActionOverAllowanceConsumptionRequest)

    def test_pass_through_requires_original_invoice_and_conditional_fx_evidence(self):
        sar = pass_through(original_currency="sar")
        self.assertEqual(sar["metadata"]["original_currency"], "SAR")
        usd = pass_through(provider_amount_minor="375", original_amount_minor="100",
                           original_currency="USD", fx_rule="USD-SAR-2026-08")
        self.assertEqual(usd["metadata"]["fx_rule"], "USD-SAR-2026-08")
        with self.assertRaises(ValueError):
            pass_through(original_currency="USD", original_amount_minor="100")
        with self.assertRaises(ValueError):
            pass_through(provider_amount_minor="338", original_amount_minor="337")

    def test_invalid_feature_inputs_fail_before_a_request_can_be_built(self):
        cases = {
            "empty source": lambda: conversation(source_event_id=""),
            "long source": lambda: conversation(source_event_id="x" * 129),
            "missing timezone": lambda: conversation(occurred_at="2026-08-04T00:00:00"),
            "invalid timestamp": lambda: conversation(occurred_at="not-a-date"),
            "missing conversation": lambda: conversation(conversation_id=""),
            "invalid conversation channel": lambda: conversation(channel="email"),
            "zero quantity": lambda: outbound_delivered_message(source_event_id="x", occurred_at=NOW, quantity="0"),
            "negative quantity": lambda: ai_reply_handling(source_event_id="x", occurred_at=NOW, quantity="-1"),
            "too precise": lambda: ai_assist_action_over_allowance(source_event_id="x", occurred_at=NOW, quantity="1.0001"),
            "fractional count": lambda: outbound_delivered_message(source_event_id="x", occurred_at=NOW, quantity="1.5"),
            "zero duration": lambda: voice_ai_started_minute(source_event_id="x", occurred_at=NOW, duration_seconds="0"),
            "fractional duration": lambda: voice_ai_started_minute(source_event_id="x", occurred_at=NOW,
                                                                    duration_seconds="1.5"),
            "missing meta event": lambda: whatsapp_meta_marketing_message(source_event_id="x", occurred_at=NOW,
                                                                            provider_event_id=""),
            "missing telephony provider": lambda: telephony_voice_minute(source_event_id="x", occurred_at=NOW,
                                                                           provider="", provider_event_id="call"),
            "missing inbound event": lambda: inbound_voice_minute(source_event_id="x", occurred_at=NOW,
                                                                    provider="Carrier", provider_event_id=""),
            "negative provider amount": lambda: pass_through(provider_amount_minor="-1"),
            "provider amount overflow": lambda: pass_through(provider_amount_minor="9223372036854775808"),
            "too many attributes": lambda: conversation(metadata={"attributes": {str(i): i for i in range(33)}}),
            "non-finite attribute": lambda: conversation(metadata={"attributes": {"score": math.inf}}),
            "unknown top-level metadata": lambda: conversation(metadata={"custom": "value"}),
        }
        for name, build in cases.items():
            with self.subTest(name=name), self.assertRaises(ValueError):
                build()

    def test_exact_boundaries_match_the_worker_contract(self):
        self.assertEqual(conversation(source_event_id="x" * 128)["quantity"], "1")
        self.assertEqual(telephony_voice_minute(source_event_id="minutes", occurred_at=NOW, provider="Carrier",
                                                provider_event_id="minutes", billable_minutes="0.001")["quantity"], "0.001")
        self.assertEqual(voice_ai_started_minute(source_event_id="duration", occurred_at=NOW,
                                                 duration_seconds="9223372036854775807")["duration_seconds"],
                         "9223372036854775807")
        self.assertEqual(pass_through(source_event_id="amount", provider_event_id="fee",
                                     provider_amount_minor="9223372036854775807",
                                     original_amount_minor="9223372036854775807")
                         ["provider_amount_minor"], "9223372036854775807")

    def test_readme_feature_matrix_names_real_exported_symbols(self):
        readme = (Path(__file__).parents[1] / "README.md").read_text(encoding="utf-8")
        contracts = {
            "conversation_24h": ("Conversation24HConsumptionRequest", "conversation_24h", "consume_conversation_24h"),
            "outbound_delivered_message": ("OutboundDeliveredMessageConsumptionRequest", "outbound_delivered_message", "consume_outbound_delivered_message"),
            "ai_assist_action_over_allowance": ("AIAssistActionConsumptionRequest", "ai_assist_action", "consume_ai_assist_action"),
            "voice_ai_started_minute": ("VoiceAIStartedMinuteConsumptionRequest", "voice_ai_started_minute", "consume_voice_ai_started_minute"),
            "ai_reply_handling": ("AIReplyHandlingConsumptionRequest", "ai_reply_handling", "consume_ai_reply_handling"),
            "whatsapp_meta_marketing_msg": ("WhatsAppMetaMarketingMessageConsumptionRequest", "whatsapp_meta_marketing_message", "consume_whatsapp_meta_marketing_message"),
            "telephony_voice_minute": ("TelephonyVoiceMinuteConsumptionRequest", "telephony_voice_minute", "consume_telephony_voice_minute"),
            "inbound_voice_minute": ("InboundVoiceMinuteConsumptionRequest", "inbound_voice_minute", "consume_inbound_voice_minute"),
            "other_provider_charge": ("OtherProviderChargeConsumptionRequest", "other_provider_charge", "consume_other_provider_charge"),
        }
        for feature_code, (contract, builder, method) in contracts.items():
            with self.subTest(feature_code=feature_code):
                self.assertIn(f"`{feature_code}`", readme)
                self.assertIn(f"`{contract}`", readme)
                self.assertIn(f"`{builder}`", readme)
                self.assertIn(f"`{method}`", readme)
                self.assertTrue(hasattr(mizan, contract))
                self.assertTrue(hasattr(mizan, builder))
                self.assertTrue(callable(getattr(MizanClient, method)))


class FeatureClientContractTests(unittest.TestCase):
    def test_every_canonical_client_method_serializes_its_exact_wire_contract(self):
        requests = []
        client = MizanClient("https://billing.test", "secret", transport=lambda request, timeout:
                             (requests.append(request) or (201, {}, b'{"data":{"accepted":true}}')))
        client.consume_conversation_24h("business-1", source_event_id="a", occurred_at=NOW,
                                        conversation_id="conversation-a", channel=Channel.WHATSAPP)
        client.consume_outbound_delivered_message("business-1", source_event_id="b", occurred_at=NOW)
        client.consume_ai_assist_action_over_allowance("business-1", source_event_id="c", occurred_at=NOW)
        client.consume_voice_ai_started_minute("business-1", source_event_id="d", occurred_at=NOW,
                                               duration_seconds="61")
        client.consume_ai_reply_handling("business-1", source_event_id="e", occurred_at=NOW)
        client.consume_whatsapp_meta_marketing_message("business-1", source_event_id="f", occurred_at=NOW,
                                                       provider_event_id="wamid.f")
        client.consume_telephony_voice_minute("business-1", source_event_id="g", occurred_at=NOW,
                                              provider="Twilio", provider_event_id="CA-g")
        client.consume_inbound_voice_minute("business-1", source_event_id="h", occurred_at=NOW,
                                            provider="Carrier", provider_event_id="IN-h",
                                            billable_minutes="2.5")
        client.consume_other_provider_charge("business-1", source_event_id="i", occurred_at=NOW,
                                             provider="Carrier", provider_event_id="INV-i",
                                             provider_amount_minor="337", provider_invoice_id="INV-i",
                                             original_amount_minor="337", original_currency="SAR",
                                             tariff_version="carrier-v1")
        bodies = [json.loads(request.data) for request in requests]
        self.assertEqual([body["feature_code"] for body in bodies], list(values(FeatureCode)))
        for index in (0, 1, 2, 4, 5, 6):
            self.assertEqual(bodies[index]["quantity"], "1")
        self.assertEqual(bodies[3]["duration_seconds"], "61")
        self.assertNotIn("quantity", bodies[3])
        self.assertEqual(bodies[7]["quantity"], "2.5")
        self.assertEqual(bodies[8]["provider_amount_minor"], "337")
        self.assertEqual(bodies[8]["metadata"]["provider_invoice_id"], "INV-i")
        self.assertNotIn("quantity", bodies[8])
        self.assertTrue(all(request.full_url.endswith("/v1/businesses/business-1/consumptions") for request in requests))

    def test_client_validation_prevents_network_calls(self):
        requests = []
        client = MizanClient("https://billing.test", "secret", transport=lambda request, timeout:
                             (requests.append(request) or (201, {}, b'{}')))
        with self.assertRaises(ValueError):
            client.consume_telephony_voice_minute("business-1", source_event_id="x", occurred_at=NOW,
                                                  provider="", provider_event_id="call")
        with self.assertRaises(ValueError):
            client.consume_voice_ai_started_minute("business-1", source_event_id="x", occurred_at=NOW,
                                                   duration_seconds="0")
        self.assertEqual(requests, [])


if __name__ == "__main__":
    unittest.main()
