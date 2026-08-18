# 04 Extend Lifecycle and Cleanup

Type: task

Status: open

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
