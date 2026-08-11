"""Authoritative closed vocabularies used by the Mizan v1 contract."""
from enum import Enum

class StrEnum(str, Enum):
    """Python 3.10-compatible string enum whose wire value is returned by ``str()``."""
    def __str__(self) -> str: return str(self.value)

class PlanId(StrEnum):
    """Immutable public catalog plan template."""
    START = "start"
    GROWTH = "growth"
    COMMAND = "command"


class BillingTerm(StrEnum):
    """Paid subscription period and catalog discount selection."""
    MONTHLY = "monthly"          # One anchored subscription month; no term discount.
    QUARTERLY = "quarterly"      # Three months with monthly included-unit grants.
    SEMI_ANNUAL = "semi_annual"  # Six months with monthly included-unit grants.
    ANNUAL = "annual"            # Twelve months; all included units are granted up front.


class Currency(StrEnum):
    """ISO settlement currency accepted by the Mizan v1 contract."""
    SAR = "SAR"


class PaymentStatus(StrEnum):
    """Trusted outcome of a uniquely identified payment event."""
    CONFIRMED = "confirmed"  # Exact paid totals are required and reconciled.
    FAILED = "failed"        # Renewal failure moves the subscription to past due.


class RefundStatus(StrEnum):
    """Trusted outcome of a uniquely identified refund event."""
    CONFIRMED = "confirmed"


class BudgetMetric(StrEnum):
    """Exact value accumulated by a feature budget."""
    AZEER_UNIT_MILLIS = "azeer_unit_millis"  # Platform-credit spend in milliunits.
    MONEY_MINOR = "money_minor"              # Provider-wallet spend in halala.
    QUANTITY = "quantity"                    # Normalized event quantity in thousandths.


class BudgetPeriod(StrEnum):
    """Lifecycle window in which a feature budget resets."""
    SUBSCRIPTION_MONTH = "subscription_month"  # Anchored to activation, not calendar month.


class BudgetAction(StrEnum):
    """Whether crossing a feature limit alerts or rejects usage."""
    ALERT = "alert"  # Commit crossing usage and emit warning/breach notifications.
    PAUSE = "pause"  # Reject the crossing request and keep the feature paused.


class Channel(StrEnum):
    """Application channel attribution stored with a usage decision."""
    WHATSAPP = "whatsapp"
    INSTAGRAM = "instagram"
    FACEBOOK = "facebook"
    TIKTOK = "tiktok"
    TELEPHONY = "telephony"
    WEBCHAT = "webchat"


class Capability(StrEnum):
    """Entitlement key frozen into a subscription snapshot."""
    # Capability values answer entitlement questions; they are not metering feature codes.
    WHATSAPP_NUMBER_1="whatsapp_number_1"; INSTAGRAM_ACCOUNT_1="instagram_account_1"; WEBSITE_WIDGET="website_widget"
    VOIP="voip"; IVR="ivr"; CALL_RECORDING_30D="call_recording_30d"; CALL_MONITORING="call_monitoring"
    AI_CALL_SUMMARY="ai_call_summary"; UNIFIED_INBOX="unified_inbox"; MOBILE_APP="mobile_app"; AI_AGENT_BUILDER="ai_agent_builder"
    BASIC_ANALYTICS="basic_analytics"; BASIC_BROADCASTS="basic_broadcasts"; BASIC_CONTACTS="basic_contacts"
    QUICK_REPLIES="quick_replies"; WELCOME_WORKING_HOURS="welcome_working_hours"; MENTIONS="mentions"; PRIVATE_NOTES="private_notes"
    ECOMMERCE_STORE_1="ecommerce_store_1"; ECOMMERCE_APP="ecommerce_app"; WEBHOOKS_API="webhooks_api"; NOTIFICATION_LOG="notification_log"
    AZEER_SUPPORT="azeer_support"; SECURITY_BASELINE="security_baseline"; TWO_FACTOR_AUTHENTICATION="two_factor_authentication"
    BYOL_SIP="byol_sip"; CUSTOM_ATTRIBUTES="custom_attributes"; ADVANCED_CONTACTS="advanced_contacts"; CUSTOM_INBOX="custom_inbox"
    MEDIA_LIBRARY="media_library"; CLOSING_REASONS="closing_reasons"; BULK_HISTORY="bulk_history"; CSAT="csat"
    ADVANCED_ANALYTICS="advanced_analytics"; WHATSAPP_FLOW="whatsapp_flow"; SEGMENTED_CAMPAIGNS="segmented_campaigns"
    SEGMENTED_ROUTING="segmented_routing"; WHATSAPP_CATALOG="whatsapp_catalog"; MULTI_STORE="multi_store"
    ADVANCED_ECOMMERCE="advanced_ecommerce"; MASKING="masking"; ADVANCED_AI_INTEGRATIONS="advanced_ai_integrations"
    WHATSAPP_FLOW_API="whatsapp_flow_api"; CUSTOM_BROADCAST_LOGIC="custom_broadcast_logic"; EXTERNAL_ATTRIBUTE_SYNC="external_attribute_sync"
    ADVANCED_ATTRIBUTE_TYPES="advanced_attribute_types"; CUSTOM_AUTOMATION_RULES="custom_automation_rules"; RBAC_AUDIT="rbac_audit"
    WHITE_LABEL="white_label"; LOCAL_HOSTING="local_hosting"
class FeatureCode(StrEnum):
    """Versioned feature metering and pricing contract."""
    CONVERSATION_24H = "conversation_24h"  # Fixed conversation windows priced in Azeer Units.
    OUTBOUND_DELIVERED_MESSAGE = "outbound_delivered_message"  # Product delivery only; provider fee is separate.
    AI_ASSIST_ACTION_OVER_ALLOWANCE = "ai_assist_action_over_allowance"  # Legacy wire name; report every action and let Mizan apply allowance.
    VOICE_AI_STARTED_MINUTE = "voice_ai_started_minute"  # Raw seconds rounded up by Mizan.
    AI_REPLY_HANDLING = "ai_reply_handling"  # Included audit/fair-use event in the default catalog.
    WHATSAPP_META_MARKETING_MSG = "whatsapp_meta_marketing_msg"  # Meta provider tariff and event ID required.
    TELEPHONY_VOICE_MINUTE = "telephony_voice_minute"  # Provider-normalized outbound minutes.
    INBOUND_VOICE_MINUTE = "inbound_voice_minute"  # Attributed inbound minutes; default is zero-rated.
    OTHER_PROVIDER_CHARGE = "other_provider_charge"  # Exact pass-through provider amount in halala.


class RecurringAddonCode(StrEnum):
    """Catalog-backed recurring subscription add-on."""
    # Quote-priced members still require approved quote evidence in the request.
    WHATSAPP_011_LANDLINE="whatsapp_011_landline"; WHATSAPP_05_MOBILE="whatsapp_05_mobile"
    CONCURRENT_CALLS_5="concurrent_calls_5"; CONCURRENT_CALLS_10="concurrent_calls_10"; CONCURRENT_CALLS_20="concurrent_calls_20"
    AUTO_DIALER="auto_dialer"; CSAT_START="csat_start"; INSTAGRAM_ADDITIONAL_ACCOUNTS="instagram_additional_accounts"
    WHATSAPP_9200="whatsapp_9200"; TOLL_FREE_800="toll_free_800"; INTERNATIONAL_NUMBER="international_number"
    OUTBOUND_MINUTE_BUNDLE_500="outbound_minute_bundle_500"; OUTBOUND_MINUTE_BUNDLE_1000="outbound_minute_bundle_1000"
    VOICE_BROADCAST="voice_broadcast"; RECORDING_RETENTION_EXTENDED="recording_retention_extended"
class ErrorCode(StrEnum):
    """Stable machine-readable API rejection or decision code."""
    # Callers should branch on these values, never on human-readable error messages.
    ACCOUNT_INACTIVE="ACCOUNT_INACTIVE"; DEPENDENCY_TEMPORARILY_UNAVAILABLE="DEPENDENCY_TEMPORARILY_UNAVAILABLE"
    DUPLICATE_PAYMENT_EVENT="DUPLICATE_PAYMENT_EVENT"; DUPLICATE_PROVIDER_EVENT="DUPLICATE_PROVIDER_EVENT"; DUPLICATE_SOURCE_EVENT="DUPLICATE_SOURCE_EVENT"
    EARLY_RENEWAL_EVENT="EARLY_RENEWAL_EVENT"; FEATURE_DISABLED="FEATURE_DISABLED"; FEATURE_PAUSED_BUDGET="FEATURE_PAUSED_BUDGET"
    FEATURE_PAUSED_MANUAL="FEATURE_PAUSED_MANUAL"; FORBIDDEN="FORBIDDEN"; IDEMPOTENCY_KEY_REUSED="IDEMPOTENCY_KEY_REUSED"
    INSUFFICIENT_AZEER_UNITS="INSUFFICIENT_AZEER_UNITS"; INSUFFICIENT_PROVIDER_BALANCE="INSUFFICIENT_PROVIDER_BALANCE"
    INTERNAL_RETRYABLE="INTERNAL_RETRYABLE"; INVALID_QUANTITY="INVALID_QUANTITY"; INVALID_REQUEST="INVALID_REQUEST"
    INVARIANT_VIOLATION="INVARIANT_VIOLATION"; MISCONFIGURED="MISCONFIGURED"; NOT_FOUND="NOT_FOUND"
    PAYMENT_AMOUNT_MISMATCH="PAYMENT_AMOUNT_MISMATCH"; QUOTE_REQUIRED="QUOTE_REQUIRED"; QUOTE_VERIFICATION_UNAVAILABLE="QUOTE_VERIFICATION_UNAVAILABLE"
    REQUEST_TIMESTAMP_OUT_OF_RANGE="REQUEST_TIMESTAMP_OUT_OF_RANGE"; SENSITIVE_RESERVE_REACHED="SENSITIVE_RESERVE_REACHED"
    STALE_PLAN_VERSION="STALE_PLAN_VERSION"; SUBSCRIPTION_CHANGE_PENDING="SUBSCRIPTION_CHANGE_PENDING"
    SUBSCRIPTION_INACTIVE="SUBSCRIPTION_INACTIVE"; UNAUTHORIZED="UNAUTHORIZED"

def values(enum_type: type[StrEnum]) -> tuple[str, ...]:
    """Return wire values for UI controls or validation without hand-maintained strings."""
    return tuple(str(item.value) for item in enum_type)
