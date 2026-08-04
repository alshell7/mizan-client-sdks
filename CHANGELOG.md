# Changelog

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
