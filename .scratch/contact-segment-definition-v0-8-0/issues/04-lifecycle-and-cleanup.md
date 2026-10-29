# 04 Extend Lifecycle and Cleanup

Type: task

Status: resolved

Blocked by: 03

## Acceptance

- Extend the behavioral fake for pending, complete, failed, stale-version,
  timeout, rejected, and ambiguous outcomes.
- Prove create, drift, dynamic update, snapshot replacement, active import,
  tombstoned import and restore, purge, destroy, and request counts under both
  engines.
- Preserve state plus a private tombstone marker after restorable deletion,
  and restore the same exact ID on apply.
- Extend protected maintenance, ownership-safe janitor reporting, repeated
  exact cleanup, and manual archival selection.

## Comments

- The provider now retains readable tombstones through private state, plans
  same-ID restoration, polls current list versions, and handles permanent
  absence and idempotent destroy.
- The behavioral fake, dual-engine lifecycle source, bounded polling tests,
  exact-ID Northstar helper, and manual janitor selection are implemented.
  Actual dual-engine execution remains pending outside the local listener
  sandbox.
- The first fixed-point review found that authoritative 4xx rejections were
  read back instead of failing immediately and that unsafe 5xx outcomes lacked
  ambiguous-write classification. Both paths now fail or recover according to
  the frozen contract, with exact-ID import guidance for an unknown create.
- The fake now models delayed completion, stale versions, permanent pending,
  failure, rejection, and ambiguous outcomes. Recovery and exact unsafe-write
  counts are defined for both engines, with an acceptance-build-only timing
  override that leaves the shipped two-second/five-minute bounds unchanged.
- Local dual-engine execution passed on 2026-10-29 with `make check` and
  `make test-hermetic` under Go 1.26.6, OpenTofu 1.13.2, and Terraform 1.17.0.
  Contact segment lifecycle, recovery, and Terraform parity shards all passed.

## Answer

The behavioral fake models pending, complete, failed, stale-version, timeout,
rejected, and ambiguous Lists outcomes. Dual-engine hermetic suites prove
create, drift, dynamic update, snapshot replacement, active import, tombstoned
import and restore, purge, destroy, and exact request counts. Readable
tombstones stay in state with a private marker, and apply restores the same
ID. Protected maintenance, ownership-safe janitor reporting, repeated exact
cleanup, and manual archival selection are in place. Local `make check` and
`make test-hermetic` passed on 2026-10-29 under OpenTofu 1.13.2 and Terraform
1.17.0.
