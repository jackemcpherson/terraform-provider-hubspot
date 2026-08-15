# 05 Extend the Cumulative Demo

Type: task

Status: resolved

Blocked by: 03

## Acceptance

- Add a stable-keyed `product-definition` module.
- Exercise create, no-op, drift, repair, refresh, import, and destroy.
- Preserve the existing dirty demo worktree.

## Comments

- 2026-08-15: Built the stable-keyed module and cumulative lifecycle in a clean
  temporary worktree. The original dirty demo worktree was not modified. Both
  engine tests and the complete demo check passed.
