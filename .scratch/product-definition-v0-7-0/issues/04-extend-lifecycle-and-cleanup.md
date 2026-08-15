# 04 Extend Lifecycle and Cleanup

Type: task

Status: resolved

Blocked by: 03

## Acceptance

- Extend the behavioural fake and both engine journeys.
- Add guarded live Product lifecycle and ownership-safe cleanup.
- Add the manual `product_definitions` archival shard.

## Comments

- 2026-08-15: Added the behavioural fake, hermetic and guarded live lifecycles,
  cumulative Northstar helpers, ownership-safe janitor, and manual archival
  shard. The full hermetic suite passed.

## Answer

The behavioral fake and both real CLIs cover normalization, drift, recovery,
non-404 retention, absence, and archive verification. Guarded live and manual
cleanup paths act only on exact generated IDs after prefix ownership checks.
