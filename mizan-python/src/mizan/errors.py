"""Typed API exceptions raised for Mizan error codes."""
from .client import (
    AccountInactiveError, DependencyTemporarilyUnavailableError, DuplicatePaymentEventError,
    DuplicateProviderEventError, DuplicateSourceEventError, EarlyRenewalEventError, FeatureDisabledError,
    FeaturePausedBudgetError, FeaturePausedManualError, ForbiddenError, IdempotencyKeyReusedError,
    InsufficientAzeerUnitsError, InsufficientProviderBalanceError, InternalRetryableError,
    InvalidQuantityError, InvalidRequestError, InvariantViolationError, MisconfiguredError, NotFoundError,
    PaymentAmountMismatchError, QuoteRequiredError, QuoteVerificationUnavailableError,
    RequestTimestampOutOfRangeError, SensitiveReserveReachedError, StalePlanVersionError,
    SubscriptionChangePendingError, SubscriptionInactiveError, UnauthorizedError,
)

__all__ = [name for name in globals() if name.endswith("Error") and name != "MizanAPIError"]
