"""Public request and response types for the Mizan API.

All monetary values are integer halala strings. Azeer Unit balances and charges are
integer milliunit strings. Keeping those values as strings prevents accidental loss
of precision in application and JSON layers.
"""

from __future__ import annotations

from typing import Any, Literal, TypeAlias, TypedDict

ExactAmount: TypeAlias = str
PlanId = Literal["start", "growth", "command"]
BillingTerm = Literal["monthly", "quarterly", "semi_annual", "annual"]
BudgetMetric = Literal["azeer_unit_millis", "money_minor", "quantity"]
BudgetAction = Literal["alert", "pause"]


class _RecurringAddonOptional(TypedDict, total=False):
    quantity: ExactAmount
    approved_quote_id: str
    approved_monthly_minor: ExactAmount


class RecurringAddon(_RecurringAddonOptional):
    code: str


class _ServiceLineOptional(TypedDict, total=False):
    quantity: ExactAmount
    taxable: bool


class ServiceLine(_ServiceLineOptional):
    code: str
    amount_minor: ExactAmount


class _ActivationOptional(TypedDict, total=False):
    timezone: str
    addons: list[RecurringAddon]
    services: list[ServiceLine]


class ActivationRequest(_ActivationOptional):
    catalog_version: str
    plan_id: PlanId
    term: BillingTerm
    seats: int
    payment_status: Literal["confirmed"]
    payment_event_id: str
    currency: Literal["SAR"]
    paid_total_minor: ExactAmount


class _SubscriptionChangeOptional(TypedDict, total=False):
    plan_id: PlanId
    term: BillingTerm
    seats: int
    addons: list[RecurringAddon]
    requested_by: str
    reason: str


class SubscriptionChangeRequest(_SubscriptionChangeOptional):
    catalog_version: str


class CancellationRequest(TypedDict, total=False):
    event_id: str
    reason: str


class _RenewalOptional(TypedDict, total=False):
    currency: Literal["SAR"]
    paid_total_minor: ExactAmount


class RenewalEventRequest(_RenewalOptional):
    payment_event_id: str
    payment_status: Literal["confirmed", "failed"]


class ConfirmedTopUp(TypedDict):
    amount_minor: ExactAmount
    payment_event_id: str
    payment_status: Literal["confirmed"]
    currency: Literal["SAR"]
    paid_total_minor: ExactAmount


class ProviderRefundRequest(TypedDict):
    amount_minor: ExactAmount
    payment_event_id: str
    refund_status: Literal["confirmed"]
    currency: Literal["SAR"]
    refunded_total_minor: ExactAmount
    reason: str


class _BudgetOptional(TypedDict, total=False):
    warning_bps: int
    sensitive: bool
    absolute_reserve: ExactAmount
    reserve_bps: int


class BudgetRequest(_BudgetOptional):
    metric: BudgetMetric
    period: Literal["subscription_month"]
    limit: ExactAmount
    action: BudgetAction


class UsageMetadata(TypedDict, total=False):
    actor: dict[str, str]
    channel: str
    channel_account_id: str
    provider: str
    provider_event_id: str
    conversation_id: str
    campaign_id: str
    raw_quantity: str
    billable_quantity: str
    attributes: dict[str, str | int | float | bool | None]


class ChargeInput(TypedDict, total=False):
    quantity: ExactAmount
    duration_seconds: ExactAmount
    provider_amount_minor: ExactAmount
    metadata: UsageMetadata


class ConsumptionComponent(ChargeInput):
    feature_code: str


class ConsumptionRequest(ChargeInput, total=False):
    source_event_id: str
    occurred_at: str
    feature_code: str
    components: list[ConsumptionComponent]


class EligibilityRequest(ChargeInput, total=False):
    components: list[ConsumptionComponent]


class BaseEnvelope(TypedDict):
    api_version: str
    catalog_version: str
    policy_version: str


class Balance(TypedDict):
    azeer_unit_millis: ExactAmount
    provider_balance_minor: ExactAmount


class InvoiceLine(TypedDict, total=False):
    code: str
    quantity: ExactAmount
    net_minor: ExactAmount
    discount_minor: ExactAmount
    after_discount_minor: ExactAmount
    vat_minor: ExactAmount
    total_minor: ExactAmount
    discountable: bool
    taxable: bool
    pricing_mode: str
    approved_quote_id: str | None


class Invoice(TypedDict):
    id: str
    currency: str
    eligible_gross_minor: ExactAmount
    discount_minor: ExactAmount
    subtotal_minor: ExactAmount
    vat_minor: ExactAmount
    total_minor: ExactAmount
    lines: list[InvoiceLine]


class ActivationResult(TypedDict):
    subscription_id: str
    cycle_id: str
    status: Literal["active"]
    current_period_start: str
    current_period_end: str
    included_unit_millis_granted: ExactAmount
    invoice: Invoice
    ledger_entry_id: str
    balances: Balance


class ActivationResponse(BaseEnvelope):
    data: ActivationResult


class ChangeResult(TypedDict):
    change_id: str
    status: Literal["pending"]
    effective_at: str
    ledger_entry_id: str


class ChangeResponse(BaseEnvelope):
    data: ChangeResult


class CancellationResult(TypedDict):
    status: Literal["cancel_at_period_end"]
    effective_at: str
    ledger_entry_id: str


class CancellationResponse(BaseEnvelope):
    data: CancellationResult


class RenewalResult(TypedDict, total=False):
    status: Literal["active", "past_due"]
    subscription_id: str
    cycle_id: str
    current_period_start: str
    current_period_end: str
    invoice: Invoice
    ledger_entry_id: str


class RenewalResponse(BaseEnvelope):
    data: RenewalResult


class TopUpResult(TypedDict, total=False):
    lot_id: str
    unit_millis_granted: ExactAmount
    expires_at: str
    balance_unit_millis: ExactAmount
    balance_before_minor: ExactAmount
    balance_after_minor: ExactAmount
    ledger_entry_id: str
    payment: dict[str, ExactAmount]


class TopUpResponse(BaseEnvelope):
    data: TopUpResult


class RefundResult(TypedDict):
    refunded_minor: ExactAmount
    balance_after_minor: ExactAmount
    ledger_entry_id: str


class RefundResponse(BaseEnvelope):
    data: RefundResult


class BudgetResult(TypedDict):
    feature_code: str
    metric: BudgetMetric
    period: Literal["subscription_month"]
    limit: ExactAmount
    action: BudgetAction
    ledger_entry_id: str


class BudgetResponse(BaseEnvelope):
    data: BudgetResult


class EligibilityResult(TypedDict, total=False):
    eligible: bool
    reason: str
    details: dict[str, Any]
    charges: list[dict[str, Any]]


class EligibilityResponse(BaseEnvelope):
    data: EligibilityResult


class EntitlementResult(TypedDict, total=False):
    capability: str
    enabled: bool
    plan_id: str
    fair_use: dict[str, int | None]


class EntitlementResponse(BaseEnvelope):
    data: EntitlementResult


class ConsumptionResult(TypedDict, total=False):
    accepted: bool
    code: str
    source_event_id: str
    ledger_entry_id: str
    business_sequence: int
    charges: list[dict[str, Any]]
    allocations_by_feature: list[dict[str, Any]]
    totals: dict[str, ExactAmount]
    balances: Balance
    details: dict[str, Any]


class ConsumptionResponse(BaseEnvelope):
    data: ConsumptionResult


class BillingSummaryResult(TypedDict, total=False):
    business_id: str
    account: dict[str, Any]
    subscription: dict[str, Any] | None
    balances: Balance
    credit_lots: list[dict[str, Any]]
    budgets: list[dict[str, Any]]
    pauses: list[dict[str, Any]]
    replication: dict[str, Any]


class BillingSummaryResponse(BaseEnvelope):
    data: BillingSummaryResult


class LedgerResult(TypedDict):
    entries: list[dict[str, Any]]
    next_after_sequence: int | None


class LedgerResponse(BaseEnvelope):
    data: LedgerResult


class CatalogResponse(TypedDict):
    catalog_version: str
    policy_version: str
    plans: dict[str, dict[str, Any]]
    terms: dict[str, dict[str, int]]
    recurring_addons: dict[str, dict[str, Any]]
    feature_prices: dict[str, dict[str, Any]]
    unit_topups: dict[str, ExactAmount]


ApiResponse: TypeAlias = dict[str, Any]
