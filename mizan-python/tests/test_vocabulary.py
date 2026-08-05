import unittest

from mizan import (BudgetAction, BudgetMetric, Channel, FeatureCode, confirmed_refund,
                   confirmed_top_up, feature_budget, outbound_delivered_message,
                   telephony_voice_minute, voice_ai_started_minute,
                   whatsapp_meta_marketing_message, values)


class VocabularyTests(unittest.TestCase):
    def test_authoritative_enum_values_and_builders(self):
        self.assertIn("outbound_delivered_message", values(FeatureCode))
        self.assertEqual(Channel.WHATSAPP.value, "whatsapp")
        self.assertEqual(confirmed_top_up(amount_minor="100", payment_event_id="pay_1", paid_total_minor="115")["currency"], "SAR")
        refund = confirmed_refund(amount_minor="100", refunded_total_minor="115",
                                  payment_event_id="refund_1", reason="Unused funds")
        self.assertEqual(refund["refunded_total_minor"], "115")
        self.assertEqual(refund["payment_event_id"], "refund_1")
        self.assertEqual(feature_budget(metric=BudgetMetric.QUANTITY, limit="10", action=BudgetAction.PAUSE)["period"], "subscription_month")
        with self.assertRaises(ValueError):
            confirmed_top_up(amount_minor="100", payment_event_id="p" * 129, paid_total_minor="115")
        with self.assertRaises(ValueError):
            confirmed_refund(amount_minor="100", refunded_total_minor="115", payment_event_id="refund_2", reason=" ")

    def test_feature_specific_usage_builders_encode_only_relevant_fields(self):
        outbound = outbound_delivered_message(source_event_id="msg-1", occurred_at="2026-08-04T00:00:00Z")
        self.assertEqual(outbound["quantity"], "1")
        self.assertNotIn("duration_seconds", outbound)
        voice_ai = voice_ai_started_minute(source_event_id="call-1", occurred_at="2026-08-04T00:00:00Z",
                                           duration_seconds="61")
        self.assertEqual(voice_ai["duration_seconds"], "61")
        self.assertNotIn("quantity", voice_ai)
        telephony = telephony_voice_minute(source_event_id="call-2", occurred_at="2026-08-04T00:00:00Z",
                                           provider="Twilio", provider_event_id="CA123")
        self.assertEqual(telephony["quantity"], "1")
        self.assertEqual(telephony["metadata"]["provider_event_id"], "CA123")
        meta = whatsapp_meta_marketing_message(source_event_id="wamid-1", occurred_at="2026-08-04T00:00:00Z",
                                                provider_event_id="wamid.123")
        self.assertEqual(meta["metadata"]["provider"], "Meta")


if __name__ == "__main__":
    unittest.main()
