# 01 Validate the Live Contract

Type: research

Status: in progress

Blocked by: None

## Acceptance

- Record current official Lists routes, processing types, filter structure,
  task states, versions, tombstones, restore behavior, scopes, and limits.
- Add a protected Free-portal probe for manual, dynamic, and snapshot Contact
  segment definitions with text and select predicates.
- Prove dynamic updates, snapshot immutability, exact delete/read/restore,
  repeated cleanup, and minimum scopes without logging identities or secrets.
- Stop if all three variants are not Free-compatible, tombstones cannot be
  read and restored by exact ID, or dated filters cannot round-trip.

## Comments

- 2026-08-18: Official primary-source research is in progress. The protected
  environment exposes the expected token and portal-identity variable names.
  Token scope proof remains a live-probe obligation because secret values are
  inaccessible by design.
- 2026-08-18: Official research confirmed the dated route and processing-type
  surface but found that deleted exact-ID reads and the canonical
  text/select/presence filter wire shape are not documented consistently.
  A probe-only protected maintenance gate must resolve both stop conditions
  before the provider resource contract is frozen.
- 2026-08-18: Protected run 32095048270 proved `MANUAL` creation, then the
  first `DYNAMIC` request failed with `ListError.ENUM_CONVERSION`. Deferred
  cleanup verified the created manual definition's tombstone. The rejected
  request used the filter guide's `IS_NOT_KNOWN`; the next probe uses the exact
  dated schema's `IS_UNKNOWN` while retaining public `is_not_known` semantics.
