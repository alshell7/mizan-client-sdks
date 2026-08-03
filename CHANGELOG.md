# Changelog

## 1.3.0 - 2026-08-04

- Added typed contract vocabularies for plans, terms, features, add-ons, currency, statuses, budgets, channels, capabilities, and error codes.
- Added safe request builders and typed Python API exceptions; Go API errors support `errors.Is` against domain sentinels.
- Catalog responses now expose the authoritative `contract_values` collection.
- Activation and renewal-change request types now support immutable per-business `plan_configuration_id` values.

## 1.2.0 — 2026-08-04

- Move public ownership to `alshell7` and use the monorepo-safe Go module path.
- Add typed Python response envelopes and complete mutation request models.
- Add Go plan/term enums, result structs, and `DecodeData[T]`.
- Expose idempotency and request IDs on unknown transport/protocol outcomes.
- Validate base URLs, bound responses to 2 MiB, and reject malformed response envelopes.
- Add multi-version CI, package build validation, trusted PyPI publishing, and expanded tests.

## 1.1.0 — 2026-08-03

- Add catalog/entitlement calls, recurring add-ons, bounded mutation retries, and initial typed requests.
