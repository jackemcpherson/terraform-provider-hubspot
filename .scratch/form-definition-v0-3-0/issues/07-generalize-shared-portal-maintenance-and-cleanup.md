# 07 — Generalize shared-portal maintenance and cleanup

**What to build:** Give operators one safe HubSpot-configuration maintenance surface that reports and archives stale owned Forms fixtures without weakening capability-specific credentials or racing other portal mutations.

**Blocked by:** 02 — Manage one typed Form definition end to end.

**Status:** resolved

- [x] A `form_definitions` capability contract declares the exact `forms` scope family and contains no credentials, portal IDs, form IDs, or user content.
- [x] Forms acceptance uses its own protected environment and least-privilege token rather than the cumulative union-scoped Northstar credential.
- [x] Property-schema, Forms, Northstar, scheduled maintenance, and manual cleanup mutations share one account-wide non-cancelling concurrency group and portal lock identity.
- [x] Scheduled maintenance reports stale active prefix-owned Form definitions and retained tombstone counts without mutating either view.
- [x] The manual operator entry is generalized from CRM-only wording to HubSpot configuration and routes each shard through a statically defined protected job and credential boundary.
- [x] Forms cleanup requires an exact owned prefix ending in `_`, a Forms-specific literal confirmation, and exact-ID archival; it never archives by name match alone.
- [x] Cleanup proves zero active prefix-owned Form definitions and reports verified Archived form definitions as retained terminal cleanup rather than deletion.
- [x] Wrong shards, malformed prefixes, wrong confirmations, portal-guard failures, unowned definitions, and unverifiable archives fail closed.
- [x] Workflow permissions remain least privilege, actions and runners remain immutably pinned, and cleanup/maintenance policy tests cover every new routing branch.

## Answer

Added the exact `form_definitions` capability contract, a protected Forms source-acceptance job, read-only active/Archived maintenance reporting, and exact-ID terminal archival behind a Forms-specific confirmation. Replaced the CRM-only manual workflow with static property and Forms jobs so operator input cannot choose an Environment or credential dynamically. All portal consumers now use `hubspot-account-free-configuration` and the `free-configuration` local lock identity, including the pinned Northstar demo.

The Forms client follows list cursors and rejects missing identities or repeated cursors. Cleanup selects only an exact owned prefix, rejects unsupported or unverifiable definitions, archives by generated ID, proves active absence and the exact Archived tombstone, then proves zero active prefix-owned Forms. Fake-backed tests cover owned/unowned routing and ambiguous failure, shell tests cover every shard/confirmation/prefix/guard/lock branch, and workflow policy tests enforce the static environments, shared lock, least permissions, immutable pins, and identifier-free manifest. `make check`, `make test-hermetic`, the tagged janitor tests, actionlint, zizmor, and the sibling demo's `make check` pass.
