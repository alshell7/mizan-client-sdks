"""Run a complete, replay-safe customer billing lifecycle against Mizan.

All configuration is server-side. Exact payment totals must come from a
verified payment-provider event; never accept them from an untrusted client.
See ``examples/README.md`` for the environment variables and run command.
"""

from __future__ import annotations

import os
import sys
from typing import NoReturn

from mizan import (
    ActivationRequest,
    BillingTerm,
    Capability,
    Channel,
    Currency,
    FeatureCode,
    MizanAPIError,
    MizanClient,
    MizanProtocolError,
    MizanTransportError,
    PaymentStatus,
    PlanId,
    UsageMetadata,
    confirmed_top_up,
)


def main() -> None:
    base_url = required_env("MIZAN_BASE_URL")
    token = required_env("MIZAN_API_TOKEN")
    business_id = required_env("MIZAN_BUSINESS_ID")
    paid_total = required_env("MIZAN_ACTIVATION_PAID_TOTAL_MINOR")
    run_id = os.getenv("MIZAN_EXAMPLE_RUN_ID", "checkout-001")

    # Runtime bearer credentials are business-scoped. Binding the client makes
    # an accidental route mismatch fail locally before a request is sent.
    client = MizanClient(base_url, token, business_id=business_id, timeout=10, max_attempts=3)

    try:
        # 1. Fetch and persist this version with checkout state before taking
        # payment. This example proceeds immediately with the current version.
        catalog = client.get_catalog()
        catalog_version = catalog["catalog_version"]
        print(f"catalog_version={catalog_version}")

        # 2. Activate from one trusted, confirmed provider payment. A stable run
        # identity yields a stable payment event and HTTP idempotency key.
        payment_event_id = f"example-activation:{run_id}"
        activation_request: ActivationRequest = {
            "catalog_version": catalog_version,
            "plan_id": PlanId.START,
            "term": BillingTerm.MONTHLY,
            "seats": 1,
            "timezone": os.getenv("MIZAN_BUSINESS_TIMEZONE", "Asia/Riyadh"),
            "payment_status": PaymentStatus.CONFIRMED,
            "payment_event_id": payment_event_id,
            "currency": Currency.SAR,
            "paid_total_minor": paid_total,
        }
        activation_key = f"activate:{business_id}:{run_id}"
        activation = client.activate_subscription(
            business_id,
            activation_request,
            idempotency_key=activation_key,
        )["data"]
        print(
            "subscription={} period={}..{} included_millis={}".format(
                activation["subscription_id"],
                activation["current_period_start"],
                activation["current_period_end"],
                activation["included_unit_millis_granted"],
            )
        )

        # 3. Provider funding is optional because it requires a second trusted
        # payment event. Set both exact amount variables to exercise this rail.
        maybe_fund_provider(client, business_id, run_id)

        # 4. Entitlement is a subscription capability decision. Eligibility is
        # a short-lived advisory and reserves no balance.
        entitlement = client.get_entitlement(
            business_id, Capability.BASIC_ANALYTICS
        )["data"]
        print(
            f"entitlement capability={entitlement.get('capability')} "
            f"enabled={entitlement.get('enabled')}"
        )

        metadata: UsageMetadata = {
            "channel": Channel.WHATSAPP,
            "conversation_id": f"example-conversation:{run_id}",
        }
        eligibility = client.check_eligibility(
            business_id,
            FeatureCode.OUTBOUND_DELIVERED_MESSAGE,
            {"quantity": "1", "metadata": metadata},
        )["data"]
        print(
            f"eligibility eligible={eligibility.get('eligible')} "
            f"reason={eligibility.get('reason', '')}"
        )
        if not eligibility.get("eligible", False):
            raise RuntimeError(
                f"advisory eligibility rejected usage: {eligibility.get('reason')} "
                f"{eligibility.get('details', {})}"
            )

        # 5. Consumption repeats every authoritative check. A real outbox event
        # supplies occurred_at. It must be in the currently open subscription
        # month and no later than now. The server-authored period start used here
        # is timezone-safe and cannot accidentally predate activation.
        source_event_id = f"example-message:{run_id}"
        consume_key = f"consume:{business_id}:{run_id}"
        decision = client.consume_outbound_delivered_message(
            business_id,
            source_event_id=source_event_id,
            occurred_at=activation["current_period_start"],
            quantity="1",
            metadata=metadata,
            idempotency_key=consume_key,
        )["data"]
        print(
            "consumption accepted={} ledger={} sequence={} balance_millis={}".format(
                decision.get("accepted"),
                decision.get("ledger_entry_id"),
                decision.get("business_sequence"),
                decision.get("balances", {}).get("azeer_unit_millis"),
            )
        )

        # Replaying the exact body and key must return the original ledger
        # identity rather than create a second debit.
        replay = client.consume_outbound_delivered_message(
            business_id,
            source_event_id=source_event_id,
            occurred_at=activation["current_period_start"],
            quantity="1",
            metadata=metadata,
            idempotency_key=consume_key,
        )["data"]
        if (
            replay.get("ledger_entry_id") != decision.get("ledger_entry_id")
            or replay.get("business_sequence") != decision.get("business_sequence")
        ):
            raise RuntimeError("idempotent replay returned a different ledger identity")
        print("idempotency_replay=original_result")

        # 6. Use the summary as the current billing view and the immutable
        # ledger for restartable export/reconciliation.
        summary = client.get_billing_summary(business_id)["data"]
        balances = summary["balances"]
        print(
            f"summary azeer_unit_millis={balances['azeer_unit_millis']} "
            f"provider_balance_minor={balances['provider_balance_minor']}"
        )
        count, last_sequence = export_ledger(client, business_id)
        print(f"ledger entries={count} last_sequence={last_sequence}")
    except (MizanAPIError, MizanTransportError, MizanProtocolError) as error:
        fail_with_mizan_error(error)


def maybe_fund_provider(client: MizanClient, business_id: str, run_id: str) -> None:
    amount = os.getenv("MIZAN_PROVIDER_TOP_UP_MINOR")
    paid_total = os.getenv("MIZAN_PROVIDER_TOP_UP_PAID_TOTAL_MINOR")
    if amount is None and paid_total is None:
        print(
            "provider_funding=skipped "
            "(set both provider top-up environment variables to exercise it)"
        )
        return
    if not amount or not paid_total:
        raise RuntimeError(
            "MIZAN_PROVIDER_TOP_UP_MINOR and "
            "MIZAN_PROVIDER_TOP_UP_PAID_TOTAL_MINOR must be set together"
        )
    payment_event_id = f"example-provider-topup:{run_id}"
    top_up = confirmed_top_up(
        amount_minor=amount,
        payment_event_id=payment_event_id,
        paid_total_minor=paid_total,
    )
    funded = client.top_up_provider_balance(
        business_id,
        top_up,
        idempotency_key=f"provider-topup:{business_id}:{run_id}",
    )["data"]
    print(f"provider_funding={funded}")


def export_ledger(client: MizanClient, business_id: str) -> tuple[int, int]:
    after = 0
    count = 0
    while True:
        page = client.get_ledger(
            business_id, after_sequence=after, limit=100
        )["data"]
        count += len(page["entries"])
        next_after = page.get("next_after_sequence")
        if next_after is None or next_after <= after:
            return count, after
        after = next_after


def fail_with_mizan_error(error: MizanAPIError | MizanTransportError | MizanProtocolError) -> NoReturn:
    if isinstance(error, MizanAPIError) and error.code == "INSUFFICIENT_AZEER_UNITS":
        raise SystemExit(
            "Azeer Units are insufficient. Fund a supported catalog package, then "
            "retry the same source event with its original body and key."
        )
    if isinstance(error, MizanAPIError) and error.code == "INSUFFICIENT_PROVIDER_BALANCE":
        raise SystemExit(
            "Provider balance is insufficient. Stop provider work and complete a "
            "trusted provider-balance top-up."
        )
    if isinstance(error, MizanAPIError):
        raise SystemExit(
            f"Mizan rejection code={error.code} retryable={error.retryable} "
            f"request_id={error.request_id} idempotency_key={error.idempotency_key} "
            f"details={error.details}"
        )
    if isinstance(error, MizanTransportError):
        raise SystemExit(
            "Mutation outcome is unknown. Retry only the identical body and "
            f"key={error.idempotency_key}; request_id={error.request_id}."
        )
    raise SystemExit(
        "Mizan returned an invalid response. Preserve "
        f"key={error.idempotency_key} and investigate request_id={error.request_id}."
    )


def required_env(name: str) -> str:
    value = os.getenv(name)
    if not value:
        raise SystemExit(f"{name} is required")
    return value


if __name__ == "__main__":
    main()
