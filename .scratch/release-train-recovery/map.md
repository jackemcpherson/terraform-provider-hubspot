# Release Train Recovery Map

This map tracks the work in `spec.md` to restore the release train, finish
v0.8.0, and amend the release order.

## Frontier

Tickets 01, 02, 05, and 06 have no blockers. Ticket 01 is next.

## Graph

- 01 Patch the Go toolchain and x/net: no blockers.
- 02 Move the current engine pins: no blockers.
- 03 Re-establish the protected baseline: blocked by 01, 02.
- 04 Decide and ship any v0.7.x patch: blocked by 03.
- 05 Discover the repository root on case-insensitive filesystems: no
  blockers.
- 06 Amend the release order: no blockers.
- 07 Rebase the Contact segment slice and open its pull request: blocked by
  03, 05.
- Contact segment 05 Extend the cumulative demo: blocked by 07 here.
- Contact segment 06 Publish documentation: blocked by contact segment 05 and
  06 here.
- Contact segment 07 Review, merge, and release: blocked by every ticket above.

## Execution Shape

A gated sequence of pull requests, not one integration branch. Pause at every
hosted or human gate.

## Decisions So Far
