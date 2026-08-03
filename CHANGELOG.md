# Changelog

## 1.2.0 — 2026-08-04

- Move public ownership to `alshell7` and use the monorepo-safe Go module path.
- Add typed Python response envelopes and complete mutation request models.
- Add Go plan/term enums, result structs, and `DecodeData[T]`.
- Expose idempotency and request IDs on unknown transport/protocol outcomes.
- Validate base URLs, bound responses to 2 MiB, and reject malformed response envelopes.
- Add multi-version CI, package build validation, trusted PyPI publishing, and expanded tests.

## 1.1.0 — 2026-08-03

- Add catalog/entitlement calls, recurring add-ons, bounded mutation retries, and initial typed requests.
