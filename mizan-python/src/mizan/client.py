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

from ._version import __version__
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

    def set_feature_budget(self, business_id: str, feature_code: str, request: BudgetRequest, *, idempotency_key: str | None = None) -> BudgetResponse:
        path = self._business_path(business_id, f"features/{quote(feature_code, safe='')}/budget")
        return cast(BudgetResponse, self._request("PUT", path, request, business_id, idempotency_key))

    def check_eligibility(self, business_id: str, feature_code: str, request: EligibilityRequest) -> EligibilityResponse:
        path = self._business_path(business_id, f"features/{quote(feature_code, safe='')}/eligibility")
        return cast(EligibilityResponse, self._request("POST", path, request, business_id, None, mutation=False))

    def get_entitlement(self, business_id: str, capability: str) -> EntitlementResponse:
        path = self._business_path(business_id, f"entitlements/{quote(capability, safe='')}")
        return cast(EntitlementResponse, self._request("GET", path, None, business_id, None, mutation=False))

    def get_catalog(self) -> CatalogResponse:
        return cast(CatalogResponse, self._request("GET", "/v1/catalog", None, "", None, mutation=False))

    def consume(self, business_id: str, request: ConsumptionRequest, *, idempotency_key: str | None = None) -> ConsumptionResponse:
        """Atomically authorize, charge, record, and sequence one source event."""
        return cast(ConsumptionResponse, self._request("POST", self._business_path(business_id, "consumptions"), request, business_id, idempotency_key))

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
                api_error = MizanAPIError(
                    status=status,
                    code=error.get("code", "HTTP_ERROR"),
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
