# Release Train Recovery Map

This map tracks the work in `spec.md` to restore the release train, finish
v0.8.0, and amend the release order.

## Frontier

Tickets 01, 02, 03, 04, 05, and 06 are resolved. Ticket 07 is the frontier.

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

- 01: Go toolchain 1.26.10 and `golang.org/x/net` v0.62.0 cleared every
  reported finding, plus the later GO-2026-6860 (PR 115). Every patched x/net
  release declares `go 1.26.0`, so the module language version moved from
  1.25.0 to 1.26.0. The maintainer approved that deviation from the spec on
  2026-10-29. Ten findings reached the provider binary. See
  [ticket 01](issues/01-patch-go-toolchain-and-x-net.md).
- 02: Current engine pins are OpenTofu 1.13.2 and Terraform 1.17.1, and the
  workflow check rejects disagreement with the Makefile (PR 114). See
  [ticket 02](issues/02-move-current-engine-pins.md).
- 05: The acceptance harness resolves the on-disk working-directory spelling
  before finding the module root (PR 116). See
  [ticket 05](issues/05-case-insensitive-repository-root.md).
- 06: ADR 0005 defers association definition and renumbers deal and ticket
  pipeline configuration to v0.9.0 and v0.10.0 (PR 113). See
  [ticket 06](issues/06-amend-release-order.md).
- 03: Protected runs 40811520501 and 40812498133 passed on provider `18224aa`
  and demo `2ef32f7`, re-establishing the baseline. See
  [ticket 03](issues/03-re-establish-protected-baseline.md).
- 04: Ten findings reached the provider binary, so v0.7.1 ships the patch
  from the qualified `main` commit. See
  [ticket 04](issues/04-decide-v0-7-x-patch.md).
