"""Public request and response types for the Mizan API.

All monetary values are integer halala strings. Azeer Unit balances and charges are
integer milliunit strings. Keeping those values as strings prevents accidental loss
of precision in application and JSON layers.
"""

from __future__ import annotations

from typing import Any, Literal, TypeAlias, TypedDict
from .enums import BillingTerm, BudgetAction, BudgetMetric, Capability, Channel, Currency, FeatureCode, PaymentStatus, PlanId, RecurringAddonCode, RefundStatus

ExactAmount: TypeAlias = str


class _RecurringAddonOptional(TypedDict, total=False):
    quantity: ExactAmount
    approved_quote_id: str
    approved_monthly_minor: ExactAmount


class RecurringAddon(_RecurringAddonOptional):
    code: RecurringAddonCode


class _ServiceLineOptional(TypedDict, total=False):
    quantity: ExactAmount
    taxable: bool


class ServiceLine(_ServiceLineOptional):
    code: str
    amount_minor: ExactAmount


class _ActivationOptional(TypedDict, total=False):
    plan_id: PlanId
    plan_configuration_id: str
    timezone: str
    addons: list[RecurringAddon]
    services: list[ServiceLine]


class ActivationRequest(_ActivationOptional):
    catalog_version: str
    term: BillingTerm
    seats: int
    payment_status: PaymentStatus
    payment_event_id: str
    currency: Currency
    paid_total_minor: ExactAmount


class _SubscriptionChangeOptional(TypedDict, total=False):
    plan_id: PlanId
    plan_configuration_id: str
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
    currency: Currency
    paid_total_minor: ExactAmount


class RenewalEventRequest(_RenewalOptional):
    payment_event_id: str
    payment_status: PaymentStatus


class ConfirmedTopUp(TypedDict):
    amount_minor: ExactAmount
    payment_event_id: str
    payment_status: PaymentStatus
    currency: Currency
    paid_total_minor: ExactAmount


class ProviderRefundRequest(TypedDict):
    amount_minor: ExactAmount
    payment_event_id: str
    refund_status: RefundStatus
    currency: Currency
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


class _CommonUsageMetadata(TypedDict, total=False):
    actor: dict[str, str]
    channel: Channel
    channel_account_id: str
    conversation_id: str
    campaign_id: str
    raw_quantity: str
    billable_quantity: str
    provider_invoice_id: str
    original_amount_minor: ExactAmount
    original_currency: str
    fx_rule: str
    tariff_version: str
    attributes: dict[str, str | int | float | bool | None]


class _OptionalProviderAttribution(TypedDict, total=False):
    provider: str
    provider_event_id: str


class _RequiredProviderAttribution(TypedDict):
    provider: str
    provider_event_id: str


class UsageMetadata(_CommonUsageMetadata, _OptionalProviderAttribution):
    """Optional application attribution persisted with a usage decision.

    ``attributes`` accepts at most 32 small scalar facts. Do not include secrets
    or unrestricted provider payloads.
    """


class ProviderUsageMetadata(_CommonUsageMetadata, _RequiredProviderAttribution):
    """Attribution required for every provider-balance feature.

    ``provider_event_id`` is the provider-side deduplication boundary and
    ``provider`` identifies the party whose tariff or invoice caused the debit.
    """


class ChargeInput(TypedDict, total=False):
    quantity: ExactAmount
    duration_seconds: ExactAmount
    provider_amount_minor: ExactAmount
    metadata: UsageMetadata


class ConsumptionComponent(ChargeInput):
    feature_code: FeatureCode


class _UsageEventRequired(TypedDict):
    source_event_id: str
    occurred_at: str


class _CountConsumptionOptional(TypedDict, total=False):
    quantity: ExactAmount
    metadata: UsageMetadata


class Conversation24HConsumptionRequest(_UsageEventRequired, _CountConsumptionOptional):
    """One or more fixed 24-hour conversation windows; quantity defaults to one."""
    feature_code: Literal["conversation_24h"]


class OutboundDeliveredMessageConsumptionRequest(_UsageEventRequired, _CountConsumptionOptional):
    """Delivered outbound product messages; quantity defaults to one."""
    feature_code: Literal["outbound_delivered_message"]


class AIAssistActionOverAllowanceConsumptionRequest(_UsageEventRequired, _CountConsumptionOptional):
    """AI-assist actions after the caller establishes allowance exhaustion."""
    feature_code: Literal["ai_assist_action_over_allowance"]


class AIReplyHandlingConsumptionRequest(_UsageEventRequired, _CountConsumptionOptional):
    """Included AI reply handling recorded for audit; quantity defaults to one."""
    feature_code: Literal["ai_reply_handling"]


class _StartedMinuteOptional(TypedDict, total=False):
    metadata: UsageMetadata


class VoiceAIStartedMinuteConsumptionRequest(_UsageEventRequired, _StartedMinuteOptional):
    """Voice AI duration; Mizan charges ``ceil(duration_seconds / 60)`` started minutes."""
    feature_code: Literal["voice_ai_started_minute"]
    duration_seconds: ExactAmount


class _ProviderQuantityOptional(TypedDict, total=False):
    quantity: ExactAmount


class WhatsAppMetaMarketingMessageConsumptionRequest(_UsageEventRequired, _ProviderQuantityOptional):
    """Meta marketing-message tariff quantity; quantity defaults to one."""
    feature_code: Literal["whatsapp_meta_marketing_msg"]
    metadata: ProviderUsageMetadata


class TelephonyVoiceMinuteConsumptionRequest(_UsageEventRequired, _ProviderQuantityOptional):
    """Provider-normalized outbound billable minutes; quantity defaults to one."""
    feature_code: Literal["telephony_voice_minute"]
    metadata: ProviderUsageMetadata


class InboundVoiceMinuteConsumptionRequest(_UsageEventRequired, _ProviderQuantityOptional):
    """Provider-normalized inbound minutes; default catalog treatment is zero-rated."""
    feature_code: Literal["inbound_voice_minute"]
    metadata: ProviderUsageMetadata


class OtherProviderChargeConsumptionRequest(_UsageEventRequired):
    """Pass-through provider amount in settlement-currency halala."""
    feature_code: Literal["other_provider_charge"]
    provider_amount_minor: ExactAmount
    metadata: ProviderUsageMetadata


class _MultiFeatureOptional(TypedDict, total=False):
    metadata: UsageMetadata


class MultiFeatureConsumptionRequest(_UsageEventRequired, _MultiFeatureOptional):
    """One source event with one to ten components committed or rejected atomically."""
    components: list[ConsumptionComponent]


ConsumptionRequest: TypeAlias = (
    Conversation24HConsumptionRequest
    | OutboundDeliveredMessageConsumptionRequest
    | AIAssistActionOverAllowanceConsumptionRequest
    | AIReplyHandlingConsumptionRequest
    | VoiceAIStartedMinuteConsumptionRequest
    | WhatsAppMetaMarketingMessageConsumptionRequest
    | TelephonyVoiceMinuteConsumptionRequest
    | InboundVoiceMinuteConsumptionRequest
    | OtherProviderChargeConsumptionRequest
    | MultiFeatureConsumptionRequest
)


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
    capability: Capability
    enabled: bool
    plan_id: str
    fair_use: dict[str, int | None]


class EntitlementResponse(BaseEnvelope):
    data: EntitlementResult


class ConsumptionResult(TypedDict, total=False):
    """Committed decision. Exact values remain decimal strings."""
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


class DeliveryEndpointInput(TypedDict, total=False):
    """Admin delivery endpoint mutation. Omit ``auth_secret`` to keep the current secret."""
    endpoint_url: str
    auth_type: Literal["none", "bearer"]
    auth_secret: str
    clear_auth_secret: bool
    enabled: bool
    reason: str


class DeliveryEndpoint(TypedDict, total=False):
    """Masked delivery endpoint. ``source`` reports business override or global fallback."""
    kind: Literal["ledger", "notification"]
    scope: Literal["business", "global"]
    source: Literal["business", "global"]
    endpoint_url: str
    auth_type: Literal["none", "bearer"]
    auth_secret_configured: bool
    enabled: bool
    revision: int
    updated_by: str
    reason: str
    updated_at: str


class DeliveryConfigurationResult(TypedDict):
    scope: Literal["business", "global"]
    ready: bool
    endpoints: list[DeliveryEndpoint | None]


class DeliveryConfigurationResponse(BaseEnvelope):
    data: DeliveryConfigurationResult


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
    contract_values: dict[str, list[str]]


ApiResponse: TypeAlias = dict[str, Any]
