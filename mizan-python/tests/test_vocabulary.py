from mizan import (BudgetAction, BudgetMetric, Channel, FeatureCode, confirmed_refund,
                   confirmed_top_up, feature_budget, values)


def test_authoritative_enum_values_and_builders():
    assert "outbound_delivered_message" in values(FeatureCode)
    assert Channel.WHATSAPP.value == "whatsapp"
    assert confirmed_top_up(amount_minor="100", payment_event_id="pay_1", paid_total_minor="115")["currency"] == "SAR"
    refund = confirmed_refund(amount_minor="100", payment_event_id="refund_1", reason="Unused funds")
    assert refund["refunded_total_minor"] == "100"
    assert refund["payment_event_id"] == "refund_1"
    assert feature_budget(metric=BudgetMetric.QUANTITY, limit="10", action=BudgetAction.PAUSE)["period"] == "subscription_month"
