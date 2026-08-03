"""Safe request builders that populate fixed protocol values automatically."""
from .enums import BudgetAction, BudgetMetric, BudgetPeriod, Currency, PaymentStatus, RefundStatus
from .models import BudgetRequest, ConfirmedTopUp, ProviderRefundRequest

def confirmed_top_up(*, amount_minor: str, payment_event_id: str, paid_total_minor: str) -> ConfirmedTopUp:
    return {"amount_minor": amount_minor, "payment_event_id": payment_event_id,
            "payment_status": PaymentStatus.CONFIRMED, "currency": Currency.SAR,
            "paid_total_minor": paid_total_minor}

def confirmed_refund(*, amount_minor: str, payment_event_id: str, reason: str) -> ProviderRefundRequest:
    return {"amount_minor": amount_minor, "payment_event_id": payment_event_id, "reason": reason,
            "refund_status": RefundStatus.CONFIRMED, "currency": Currency.SAR,
            "refunded_total_minor": amount_minor}

def feature_budget(*, metric: BudgetMetric, limit: str, action: BudgetAction,
                   warning_bps: int = 8000, sensitive: bool = False) -> BudgetRequest:
    return {"metric": metric, "period": BudgetPeriod.SUBSCRIPTION_MONTH.value,
            "limit": limit, "warning_bps": warning_bps, "action": action, "sensitive": sensitive}
