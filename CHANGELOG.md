# Changelog

## 1.7.0 - 2026-08-06

- Add a read-only balance impact preview for consumption, Azeer Unit and provider funding, provider refunds, promotional grants, and feature budgets.
- Add attributed global add-on rollout governance for catalog contents, availability, rollout stages, notes, and documentation links.
- Add paginated admin helpers for the business directory, usage decisions, and business audit history.

## 1.6.0 - 2026-08-06

- Require whole positive quantities for count-priced events while preserving exact milli-precision for provider-normalized telephony minutes.
- Require VAT-inclusive `refunded_total_minor` in the Python refund builder and model refund responses as principal, VAT, and total reversals.
- Align SDK metadata validation with the server's reserved top-level keys, bounded actor identity, and exact-value limits.
- Document the hardened current-period event-time, funding, refund, and retry contracts.
- Stop sending caller-selected admin roles; the server derives role from the role-specific credential.
- Remove caller-defined activation service lines and bound payment-event identifiers and refund reasons.

## 1.5.0 - 2026-08-05

- Add framework-neutral, typed webhook receivers for ledger and notification delivery in Go and Python.
- Add Go Fiber v2 middleware and an optional FastAPI endpoint adapter that can be mounted on any application route.
- Validate bearer authentication, exact amounts, event identity, supported types, and balanced ledger postings before dispatch.
- Return `X-Mizan-Ack-Sequence` only after ledger application processing succeeds.
- Document all ledger entry types and the outbox ID, event ID, source event ID, business sequence, deduplication, ordering, retry, and dead-letter contracts.

## 1.4.0 - 2026-08-04

- Added nine exported feature-specific Python request contracts and nine distinct Go usage structs, one for every catalog feature code.
- Added validated builders and canonical client methods; quantity-based contracts default to one while started-minute and provider-amount contracts require their own exact inputs.
- Fixed Meta attribution to `Meta`, required provider/event attribution for provider-priced features, and added provider-normalized telephony/inbound-minute contracts.
- Added compile-checked examples, exact-boundary tests, pre-network validation tests, exhaustive wire serialization tests, and README-to-symbol drift tests.
- Added typed provider attribution and documented response/delivery fields.
- Added admin clients for global and per-business delivery endpoint configuration.

## 1.3.0 - 2026-08-04

- Added typed contract vocabularies for plans, terms, features, add-ons, currency, statuses, budgets, channels, capabilities, and error codes.
- Added safe request builders and typed Python API exceptions; Go API errors support `errors.Is` against domain sentinels.
- Catalog responses now expose the authoritative `contract_values` collection.
- Activation and renewal-change request types now support immutable per-business `plan_configuration_id` values.

## 1.2.0 — 2026-08-04

- Add typed Python response envelopes and complete mutation request models.
- Add Go plan/term enums, result structs, and `DecodeData[T]`.
- Expose idempotency and request IDs on unknown transport/protocol outcomes.
- Validate base URLs, bound responses to 2 MiB, and reject malformed response envelopes.
- Add multi-version CI, package build validation, trusted PyPI publishing, and expanded tests.

## 1.1.0 — 2026-08-03

- Add catalog/entitlement calls, recurring add-ons, bounded mutation retries, and initial typed requests.
