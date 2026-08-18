# 01 Validate the Live Contract

Type: research

Status: claimed

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
- 2026-08-18: Protected run 32095515566 proved that `IS_UNKNOWN` works and a
  text/presence `DYNAMIC` definition round-trips. `SNAPSHOT` creation then
  rejected `MULTISTRING` for the `lifecyclestage` select property with
  `ListError.INVALID_OPERATION_FOR_PROPERTY_TYPE`. Deferred cleanup verified
  both created tombstones. A final matrix tests whether `STRING` or
  `ENUMERATION` is universal across text and select without a schema read.
- 2026-08-18: Protected run 32096106495 completed the operation matrix:
  `MULTISTRING` and `STRING` accepted text but rejected select, while
  `ENUMERATION` rejected text but accepted select. Every accepted candidate
  was deleted and verified through the same exact-ID tombstone. The run stopped
  before resource implementation and demo execution as required.

## Findings

The agreed three-field filter cannot choose the required dated wire operation
for both text and select properties using only the Lists API. A frozen contract
now requires one user decision: expose property type in each value filter,
require contact-property schema read permission and infer it, narrow v0.8.0 to
one property kind, or defer the release. Until then this ticket remains claimed
and all implementation tickets remain blocked.
