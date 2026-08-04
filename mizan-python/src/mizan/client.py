from __future__ import annotations

import json
import random
import time
import uuid
from datetime import datetime, timezone
from typing import Any, Callable, Mapping, cast
from urllib.error import HTTPError, URLError
from urllib.parse import quote, urlencode, urlsplit
from urllib.request import Request, urlopen

from . import builders as usage_builders
from ._version import __version__
from .enums import Capability, FeatureCode
from .models import (
    ActivationRequest,
    ActivationResponse,
    ApiResponse,
    BillingSummaryResponse,
    BudgetRequest,
    BudgetResponse,
    CancellationRequest,
    CancellationResponse,
    CatalogResponse,
    ChangeResponse,
    ConfirmedTopUp,
    ConsumptionRequest,
    ConsumptionResponse,
    DeliveryConfigurationResponse,
    DeliveryEndpointInput,
    EligibilityRequest,
    EligibilityResponse,
    EntitlementResponse,
    LedgerResponse,
    ProviderRefundRequest,
    RefundResponse,
    RenewalEventRequest,
    RenewalResponse,
    SubscriptionChangeRequest,
    TopUpResponse,
    UsageMetadata,
)

Transport = Callable[[Request, float], tuple[int, Mapping[str, str], bytes]]
Logger = Callable[[str, Mapping[str, Any]], None]


class MizanError(Exception):
    """Base SDK error."""


class MizanAPIError(MizanError):
    """A structured error returned by Mizan.

    ``retryable`` is authoritative. For mutations, any SDK retry uses the exact
    original body and ``idempotency_key``.
    """

    def __init__(
        self,
        *,
        status: int,
        code: str,
        message: str,
        retryable: bool = False,
        details: Mapping[str, Any] | None = None,
        request_id: str | None = None,
        idempotency_key: str | None = None,
    ) -> None:
        super().__init__(message)
        self.status = status
        self.code = code
        self.retryable = retryable
        self.details = dict(details or {})
        self.request_id = request_id
        self.idempotency_key = idempotency_key

class AccountInactiveError(MizanAPIError): pass
class DependencyTemporarilyUnavailableError(MizanAPIError): pass
class DuplicatePaymentEventError(MizanAPIError): pass
class DuplicateProviderEventError(MizanAPIError): pass
class DuplicateSourceEventError(MizanAPIError): pass
class EarlyRenewalEventError(MizanAPIError): pass
class FeatureDisabledError(MizanAPIError): pass
class FeaturePausedBudgetError(MizanAPIError): pass
class FeaturePausedManualError(MizanAPIError): pass
class ForbiddenError(MizanAPIError): pass
class IdempotencyKeyReusedError(MizanAPIError): pass
class InsufficientAzeerUnitsError(MizanAPIError): pass
class InsufficientProviderBalanceError(MizanAPIError): pass
class InternalRetryableError(MizanAPIError): pass
class InvalidQuantityError(MizanAPIError): pass
class InvalidRequestError(MizanAPIError): pass
class InvariantViolationError(MizanAPIError): pass
class MisconfiguredError(MizanAPIError): pass
class NotFoundError(MizanAPIError): pass
class PaymentAmountMismatchError(MizanAPIError): pass
class QuoteRequiredError(MizanAPIError): pass
class QuoteVerificationUnavailableError(MizanAPIError): pass
class RequestTimestampOutOfRangeError(MizanAPIError): pass
class SensitiveReserveReachedError(MizanAPIError): pass
class StalePlanVersionError(MizanAPIError): pass
class SubscriptionChangePendingError(MizanAPIError): pass
class SubscriptionInactiveError(MizanAPIError): pass
class UnauthorizedError(MizanAPIError): pass

_API_ERROR_TYPES = {
    "ACCOUNT_INACTIVE": AccountInactiveError, "DEPENDENCY_TEMPORARILY_UNAVAILABLE": DependencyTemporarilyUnavailableError,
    "DUPLICATE_PAYMENT_EVENT": DuplicatePaymentEventError, "DUPLICATE_PROVIDER_EVENT": DuplicateProviderEventError,
    "DUPLICATE_SOURCE_EVENT": DuplicateSourceEventError, "EARLY_RENEWAL_EVENT": EarlyRenewalEventError,
    "FEATURE_DISABLED": FeatureDisabledError, "FEATURE_PAUSED_BUDGET": FeaturePausedBudgetError,
    "FEATURE_PAUSED_MANUAL": FeaturePausedManualError, "FORBIDDEN": ForbiddenError,
    "IDEMPOTENCY_KEY_REUSED": IdempotencyKeyReusedError, "INSUFFICIENT_AZEER_UNITS": InsufficientAzeerUnitsError,
    "INSUFFICIENT_PROVIDER_BALANCE": InsufficientProviderBalanceError, "INTERNAL_RETRYABLE": InternalRetryableError,
    "INVALID_QUANTITY": InvalidQuantityError, "INVALID_REQUEST": InvalidRequestError,
    "INVARIANT_VIOLATION": InvariantViolationError, "MISCONFIGURED": MisconfiguredError, "NOT_FOUND": NotFoundError,
    "PAYMENT_AMOUNT_MISMATCH": PaymentAmountMismatchError, "QUOTE_REQUIRED": QuoteRequiredError,
    "QUOTE_VERIFICATION_UNAVAILABLE": QuoteVerificationUnavailableError,
    "REQUEST_TIMESTAMP_OUT_OF_RANGE": RequestTimestampOutOfRangeError, "SENSITIVE_RESERVE_REACHED": SensitiveReserveReachedError,
    "STALE_PLAN_VERSION": StalePlanVersionError, "SUBSCRIPTION_CHANGE_PENDING": SubscriptionChangePendingError,
    "SUBSCRIPTION_INACTIVE": SubscriptionInactiveError, "UNAUTHORIZED": UnauthorizedError,
}


class MizanTransportError(MizanError):
    """The request outcome is unknown after transport retries.

    For a mutation, retry with ``idempotency_key`` and the identical request body.
    """

    def __init__(self, message: str, *, request_id: str, idempotency_key: str | None) -> None:
        super().__init__(message)
        self.request_id = request_id
        self.idempotency_key = idempotency_key


class MizanProtocolError(MizanError):
    """Mizan returned an invalid or excessively large JSON response."""

    def __init__(self, message: str, *, request_id: str, idempotency_key: str | None) -> None:
        super().__init__(message)
        self.request_id = request_id
        self.idempotency_key = idempotency_key


def _default_transport(request: Request, timeout: float) -> tuple[int, Mapping[str, str], bytes]:
    try:
        with urlopen(request, timeout=timeout) as response:
            return response.status, dict(response.headers), response.read(2_097_153)
    except HTTPError as exc:
        return exc.code, dict(exc.headers), exc.read(2_097_153)


def _header(headers: Mapping[str, str], name: str) -> str | None:
    expected = name.lower()
    return next((value for key, value in headers.items() if key.lower() == expected), None)


class MizanClient:
    """Thread-safe, calculation-free client for the authoritative Mizan API."""

    def __init__(
        self,
        base_url: str,
        token: str,
        *,
        timeout: float = 10.0,
        max_attempts: int = 3,
        transport: Transport | None = None,
        logger: Logger | None = None,
    ) -> None:
        if not base_url or not token:
            raise ValueError("base_url and token are required")
        parsed = urlsplit(base_url)
        if parsed.scheme not in {"http", "https"} or not parsed.netloc or parsed.query or parsed.fragment:
            raise ValueError("base_url must be an absolute HTTP(S) URL without a query or fragment")
        if timeout <= 0 or max_attempts < 1:
            raise ValueError("timeout must be positive and max_attempts must be at least one")
        self.base_url = base_url.rstrip("/")
        self.token = token
        self.timeout = timeout
        self.max_attempts = max_attempts
        self._transport = transport or _default_transport
        self._logger = logger

    def activate_subscription(self, business_id: str, request: ActivationRequest, *, idempotency_key: str | None = None) -> ActivationResponse:
        """Activate and pay for the first subscription period."""
        return cast(ActivationResponse, self._request("POST", self._business_path(business_id, "subscriptions/activate"), request, business_id, idempotency_key))

    def change_subscription(self, business_id: str, request: SubscriptionChangeRequest, *, idempotency_key: str | None = None) -> ChangeResponse:
        """Schedule one catalog-backed change for the next renewal boundary."""
        return cast(ChangeResponse, self._request("POST", self._business_path(business_id, "subscriptions/change"), request, business_id, idempotency_key))

    def cancel_subscription(self, business_id: str, request: CancellationRequest, *, idempotency_key: str | None = None) -> CancellationResponse:
        """Schedule cancellation at the paid period end."""
        return cast(CancellationResponse, self._request("POST", self._business_path(business_id, "subscriptions/cancel"), request, business_id, idempotency_key))

    def apply_renewal_event(self, business_id: str, request: RenewalEventRequest, *, idempotency_key: str | None = None) -> RenewalResponse:
        """Apply a uniquely identified confirmed or failed renewal payment event."""
        return cast(RenewalResponse, self._request("POST", self._business_path(business_id, "subscriptions/renewal-events"), request, business_id, idempotency_key))

    def top_up_azeer_units(self, business_id: str, request: ConfirmedTopUp, *, idempotency_key: str | None = None) -> TopUpResponse:
        """Purchase expiring Azeer Units from a confirmed payment."""
        return cast(TopUpResponse, self._request("POST", self._business_path(business_id, "azeer-units/top-ups"), request, business_id, idempotency_key))

    def top_up_provider_balance(self, business_id: str, request: ConfirmedTopUp, *, idempotency_key: str | None = None) -> TopUpResponse:
        """Fund the Provider Fees Balance from a confirmed payment."""
        return cast(TopUpResponse, self._request("POST", self._business_path(business_id, "provider-balance/top-ups"), request, business_id, idempotency_key))

    def refund_provider_balance(self, business_id: str, request: ProviderRefundRequest, *, idempotency_key: str | None = None) -> RefundResponse:
        """Apply a confirmed provider-balance refund."""
        return cast(RefundResponse, self._request("POST", self._business_path(business_id, "provider-balance/refunds"), request, business_id, idempotency_key))

    def set_feature_budget(self, business_id: str, feature_code: FeatureCode, request: BudgetRequest, *, idempotency_key: str | None = None) -> BudgetResponse:
        path = self._business_path(business_id, f"features/{quote(feature_code, safe='')}/budget")
        return cast(BudgetResponse, self._request("PUT", path, request, business_id, idempotency_key))

    def check_eligibility(self, business_id: str, feature_code: FeatureCode, request: EligibilityRequest) -> EligibilityResponse:
        path = self._business_path(business_id, f"features/{quote(feature_code, safe='')}/eligibility")
        return cast(EligibilityResponse, self._request("POST", path, request, business_id, None, mutation=False))

    def get_entitlement(self, business_id: str, capability: Capability) -> EntitlementResponse:
        path = self._business_path(business_id, f"entitlements/{quote(capability, safe='')}")
        return cast(EntitlementResponse, self._request("GET", path, None, business_id, None, mutation=False))

    def get_catalog(self) -> CatalogResponse:
        return cast(CatalogResponse, self._request("GET", "/v1/catalog", None, "", None, mutation=False))

    def consume(self, business_id: str, request: ConsumptionRequest, *, idempotency_key: str | None = None) -> ConsumptionResponse:
        """Atomically authorize, charge, record, and sequence one source event."""
        return cast(ConsumptionResponse, self._request("POST", self._business_path(business_id, "consumptions"), request, business_id, idempotency_key))

    def consume_conversation_24h(self, business_id: str, *, source_event_id: str, occurred_at: str,
                                 quantity: str = "1", metadata: UsageMetadata | None = None,
                                 idempotency_key: str | None = None) -> ConsumptionResponse:
        """Charge one or more fixed 24-hour conversation windows."""
        return self.consume(business_id, usage_builders.conversation_24h(
            source_event_id=source_event_id, occurred_at=occurred_at, quantity=quantity, metadata=metadata),
            idempotency_key=idempotency_key)

    def consume_outbound_delivered_message(self, business_id: str, *, source_event_id: str, occurred_at: str,
                                           quantity: str = "1", metadata: UsageMetadata | None = None,
                                           idempotency_key: str | None = None) -> ConsumptionResponse:
        """Charge delivered outbound messages; provider fees remain a separate event/component."""
        return self.consume(business_id, usage_builders.outbound_delivered_message(
            source_event_id=source_event_id, occurred_at=occurred_at, quantity=quantity, metadata=metadata),
            idempotency_key=idempotency_key)

    def consume_ai_assist_action_over_allowance(self, business_id: str, *, source_event_id: str, occurred_at: str,
                                                quantity: str = "1", metadata: UsageMetadata | None = None,
                                                idempotency_key: str | None = None) -> ConsumptionResponse:
        """Charge AI-assist actions only after the calling service establishes allowance exhaustion."""
        return self.consume(business_id, usage_builders.ai_assist_action_over_allowance(
            source_event_id=source_event_id, occurred_at=occurred_at, quantity=quantity, metadata=metadata),
            idempotency_key=idempotency_key)

    def consume_ai_reply_handling(self, business_id: str, *, source_event_id: str, occurred_at: str,
                                  quantity: str = "1", metadata: UsageMetadata | None = None,
                                  idempotency_key: str | None = None) -> ConsumptionResponse:
        """Record included AI reply handling without an extra charge."""
        return self.consume(business_id, usage_builders.ai_reply_handling(
            source_event_id=source_event_id, occurred_at=occurred_at, quantity=quantity, metadata=metadata),
            idempotency_key=idempotency_key)

    def consume_voice_ai_started_minute(self, business_id: str, *, source_event_id: str, occurred_at: str,
                                        duration_seconds: str, metadata: UsageMetadata | None = None,
                                        idempotency_key: str | None = None) -> ConsumptionResponse:
        """Charge Voice AI by started minute from required raw duration seconds."""
        return self.consume(business_id, usage_builders.voice_ai_started_minute(
            source_event_id=source_event_id, occurred_at=occurred_at,
            duration_seconds=duration_seconds, metadata=metadata), idempotency_key=idempotency_key)

    def consume_whatsapp_meta_marketing_message(self, business_id: str, *, source_event_id: str,
                                                occurred_at: str, provider_event_id: str,
                                                quantity: str = "1",
                                                metadata: UsageMetadata | None = None,
                                                idempotency_key: str | None = None) -> ConsumptionResponse:
        """Charge Meta marketing messages with mandatory provider-event deduplication."""
        return self.consume(business_id, usage_builders.whatsapp_meta_marketing_message(
            source_event_id=source_event_id, occurred_at=occurred_at, provider_event_id=provider_event_id,
            quantity=quantity, metadata=metadata), idempotency_key=idempotency_key)

    def consume_telephony_voice_minute(self, business_id: str, *, source_event_id: str, occurred_at: str,
                                       provider: str, provider_event_id: str, billable_minutes: str = "1",
                                       metadata: UsageMetadata | None = None,
                                       idempotency_key: str | None = None) -> ConsumptionResponse:
        """Charge provider-normalized billable minutes, not raw call duration."""
        return self.consume(business_id, usage_builders.telephony_voice_minute(
            source_event_id=source_event_id, occurred_at=occurred_at, provider=provider,
            provider_event_id=provider_event_id, billable_minutes=billable_minutes, metadata=metadata),
            idempotency_key=idempotency_key)

    def consume_inbound_voice_minute(self, business_id: str, *, source_event_id: str, occurred_at: str,
                                     provider: str, provider_event_id: str, billable_minutes: str = "1",
                                     metadata: UsageMetadata | None = None,
                                     idempotency_key: str | None = None) -> ConsumptionResponse:
        """Record provider-normalized inbound minutes; catalog tariff may be zero or overridden."""
        return self.consume(business_id, usage_builders.inbound_voice_minute(
            source_event_id=source_event_id, occurred_at=occurred_at, provider=provider,
            provider_event_id=provider_event_id, billable_minutes=billable_minutes, metadata=metadata),
            idempotency_key=idempotency_key)

    def consume_other_provider_charge(self, business_id: str, *, source_event_id: str, occurred_at: str,
                                      provider: str, provider_event_id: str, provider_amount_minor: str,
                                      metadata: UsageMetadata | None = None,
                                      idempotency_key: str | None = None) -> ConsumptionResponse:
        """Debit an exact pass-through provider amount in settlement-currency halala."""
        return self.consume(business_id, usage_builders.other_provider_charge(
            source_event_id=source_event_id, occurred_at=occurred_at, provider=provider,
            provider_event_id=provider_event_id, provider_amount_minor=provider_amount_minor,
            metadata=metadata), idempotency_key=idempotency_key)

    # Compatibility aliases retain complete signatures; canonical names above mirror feature codes.
    def consume_ai_assist_over_allowance(self, business_id: str, *, source_event_id: str, occurred_at: str,
                                         quantity: str = "1", metadata: UsageMetadata | None = None,
                                         idempotency_key: str | None = None) -> ConsumptionResponse:
        return self.consume_ai_assist_action_over_allowance(
            business_id, source_event_id=source_event_id, occurred_at=occurred_at,
            quantity=quantity, metadata=metadata, idempotency_key=idempotency_key)

    def consume_voice_ai(self, business_id: str, *, source_event_id: str, occurred_at: str,
                         duration_seconds: str, metadata: UsageMetadata | None = None,
                         idempotency_key: str | None = None) -> ConsumptionResponse:
        return self.consume_voice_ai_started_minute(
            business_id, source_event_id=source_event_id, occurred_at=occurred_at,
            duration_seconds=duration_seconds, metadata=metadata, idempotency_key=idempotency_key)

    def consume_telephony_voice(self, business_id: str, *, source_event_id: str, occurred_at: str,
                                provider: str, provider_event_id: str, billable_minutes: str = "1",
                                metadata: UsageMetadata | None = None,
                                idempotency_key: str | None = None) -> ConsumptionResponse:
        return self.consume_telephony_voice_minute(
            business_id, source_event_id=source_event_id, occurred_at=occurred_at,
            provider=provider, provider_event_id=provider_event_id, billable_minutes=billable_minutes,
            metadata=metadata, idempotency_key=idempotency_key)

    def consume_inbound_voice(self, business_id: str, *, source_event_id: str, occurred_at: str,
                              provider: str, provider_event_id: str, billable_minutes: str = "1",
                              metadata: UsageMetadata | None = None,
                              idempotency_key: str | None = None) -> ConsumptionResponse:
        return self.consume_inbound_voice_minute(
            business_id, source_event_id=source_event_id, occurred_at=occurred_at,
            provider=provider, provider_event_id=provider_event_id, billable_minutes=billable_minutes,
            metadata=metadata, idempotency_key=idempotency_key)

    def get_billing_summary(self, business_id: str) -> BillingSummaryResponse:
        return cast(BillingSummaryResponse, self._request("GET", self._business_path(business_id, "billing-summary"), None, business_id, None, mutation=False))

    def get_ledger(self, business_id: str, *, after_sequence: int = 0, limit: int = 50) -> LedgerResponse:
        if after_sequence < 0 or not 1 <= limit <= 100:
            raise ValueError("after_sequence must be non-negative and limit must be 1..100")
        query = urlencode({"after_sequence": after_sequence, "limit": limit})
        return cast(LedgerResponse, self._request("GET", f"{self._business_path(business_id, 'ledger')}?{query}", None, business_id, None, mutation=False))

    def _business_path(self, business_id: str, suffix: str) -> str:
        if not business_id:
            raise ValueError("business_id is required")
        return f"/v1/businesses/{quote(business_id, safe='')}/{suffix}"

    def _request(
        self,
        method: str,
        path: str,
        body: Mapping[str, Any] | None,
        business_id: str,
        idempotency_key: str | None,
        *,
        mutation: bool = True,
        extra_headers: Mapping[str, str] | None = None,
    ) -> ApiResponse:
        key = idempotency_key or (str(uuid.uuid4()) if mutation else None)
        correlation_id = str(uuid.uuid4())
        encoded = json.dumps(body, separators=(",", ":"), ensure_ascii=False).encode() if body is not None else None
        last_error: BaseException | None = None
        for attempt in range(1, self.max_attempts + 1):
            headers = {
                "Authorization": f"Bearer {self.token}",
                "Accept": "application/json",
                "User-Agent": f"mizan-python/{__version__}",
                "X-Business-Id": business_id,
                "X-Request-ID": correlation_id,
                "X-Request-Timestamp": datetime.now(timezone.utc).isoformat(),
            }
            if encoded is not None:
                headers["Content-Type"] = "application/json"
            if key:
                headers["Idempotency-Key"] = key
            if extra_headers:
                headers.update(extra_headers)
            request = Request(self.base_url + path, data=encoded, headers=headers, method=method)
            try:
                status, response_headers, raw = self._transport(request, self.timeout)
                if len(raw) > 2_097_152:
                    raise MizanProtocolError("Mizan response exceeded the 2 MiB safety limit", request_id=correlation_id, idempotency_key=key)
                try:
                    payload = json.loads(raw.decode("utf-8")) if raw else {}
                except (UnicodeDecodeError, json.JSONDecodeError) as exc:
                    raise MizanProtocolError("Mizan returned invalid JSON", request_id=correlation_id, idempotency_key=key) from exc
                if not isinstance(payload, dict):
                    raise MizanProtocolError("Mizan returned a non-object JSON response", request_id=correlation_id, idempotency_key=key)
                if 200 <= status < 300:
                    self._log("request_complete", {"status": status, "attempt": attempt, "request_id": correlation_id})
                    return payload
                error = payload.get("error", {})
                if not isinstance(error, dict):
                    raise MizanProtocolError("Mizan returned an invalid error envelope", request_id=correlation_id, idempotency_key=key)
                error_code = error.get("code", "HTTP_ERROR")
                api_error = _API_ERROR_TYPES.get(error_code, MizanAPIError)(
                    status=status,
                    code=error_code,
                    message=error.get("message", f"Mizan returned HTTP {status}"),
                    retryable=bool(error.get("retryable", False)),
                    details=error.get("details", {}),
                    request_id=error.get("request_id") or _header(response_headers, "x-request-id"),
                    idempotency_key=key,
                )
                if not mutation or not api_error.retryable or attempt == self.max_attempts:
                    raise api_error
                last_error = api_error
            except (URLError, TimeoutError, OSError) as exc:
                last_error = exc
                if not mutation or attempt == self.max_attempts:
                    raise MizanTransportError(
                        f"Request outcome is unknown after {attempt} attempt(s)",
                        request_id=correlation_id,
                        idempotency_key=key,
                    ) from exc
            self._log("request_retry", {"attempt": attempt, "request_id": correlation_id, "idempotency_key": key})
            time.sleep(random.uniform(0, min(2.0, 0.1 * (2 ** (attempt - 1)))))
        raise MizanTransportError("Request outcome is unknown", request_id=correlation_id, idempotency_key=key) from last_error

    def _log(self, event: str, fields: Mapping[str, Any]) -> None:
        if self._logger:
            self._logger(event, fields)


class MizanAdminClient(MizanClient):
    """Admin-scoped client for global and per-business delivery configuration.

    Use a dedicated Admin Worker token. Every mutation is attributed to ``actor``
    and ``role`` and must include a human-readable ``reason`` in its request body.
    """

    def __init__(self, base_url: str, token: str, *, actor: str,
                 role: str = "billing_admin", **kwargs: Any) -> None:
        super().__init__(base_url, token, **kwargs)
        if not actor:
            raise ValueError("actor is required")
        if role not in {"billing_admin", "finance_admin", "support_admin"}:
            raise ValueError("role must be billing_admin, finance_admin, or support_admin")
        self.actor = actor
        self.role = role

    def _admin_headers(self) -> Mapping[str, str]:
        return {"X-Admin-Actor": self.actor, "X-Admin-Role": self.role}

    def get_global_delivery_endpoints(self) -> DeliveryConfigurationResponse:
        """Read masked fallbacks used only when a business has no endpoint record."""
        return cast(DeliveryConfigurationResponse, self._request(
            "GET", "/admin/api/delivery-endpoints", None, "", None, mutation=False,
            extra_headers=self._admin_headers()))

    def configure_global_delivery_endpoint(self, kind: str, request: DeliveryEndpointInput,
                                           *, idempotency_key: str | None = None) -> DeliveryConfigurationResponse:
        """Create, rotate, enable, or disable one global fallback endpoint."""
        if kind not in {"ledger", "notification"}:
            raise ValueError("kind must be ledger or notification")
        return cast(DeliveryConfigurationResponse, self._request(
            "PUT", f"/admin/api/delivery-endpoints/{kind}", request, "", idempotency_key,
            extra_headers=self._admin_headers()))

    def get_business_delivery_endpoints(self, business_id: str) -> DeliveryConfigurationResponse:
        """Read effective endpoints and the source of each resolved endpoint."""
        path = f"/admin/api/businesses/{quote(business_id, safe='')}/delivery-endpoints"
        return cast(DeliveryConfigurationResponse, self._request(
            "GET", path, None, business_id, None, mutation=False,
            extra_headers=self._admin_headers()))

    def configure_business_delivery_endpoint(self, business_id: str, kind: str,
                                             request: DeliveryEndpointInput,
                                             *, idempotency_key: str | None = None) -> DeliveryConfigurationResponse:
        """Set an explicit business endpoint; an explicit disabled row suppresses fallback."""
        if kind not in {"ledger", "notification"}:
            raise ValueError("kind must be ledger or notification")
        path = f"/admin/api/businesses/{quote(business_id, safe='')}/delivery-endpoints/{kind}"
        return cast(DeliveryConfigurationResponse, self._request(
            "PUT", path, request, business_id, idempotency_key,
            extra_headers=self._admin_headers()))
