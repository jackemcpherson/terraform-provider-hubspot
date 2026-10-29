# 05 Extend the Cumulative Demo

Type: task

Status: resolved

Blocked by: 03

## Acceptance

- Add a stable-keyed `contact-segment-definition` module that accepts a map of
  manual, dynamic, and snapshot definitions.
- Manage all three Northstar variants without Contact memberships or record
  values. Use existing Contact text and select property names.
- Exercise create, no-op, rename, dynamic drift and repair, snapshot
  replacement, refresh, active import, tombstoned import and restore, destroy,
  and terminal cleanup under OpenTofu and Terraform.
- Work in an isolated checkout and preserve the dirty local demo worktree.

## Comments

- 2026-10-29: Release train recovery adds a blocker: `release-train-recovery`
  ticket 07 (rebase the Contact segment slice and open its pull request). See
  `.scratch/release-train-recovery/map.md`.
- 2026-10-29: The cumulative demo change merged as demo PR #35
  (<https://github.com/jackemcpherson/terraform-hubspot-instance-demo/pull/35>)
  at squash commit `7b1531b9ac43883920291bcdcc19dd8c6b6ed1c0`. It was built in a
  fresh clone from demo `main` `c33e553`, not the pinned `2ef32f7`. The commits
  between those two change only CI and toolchain files. The original dirty demo
  worktree was not touched.
- 2026-10-29: The demo adds a stable-keyed `contact-segment-definition`
  module. It wires the manual `all_contacts`, dynamic `completed_onboarding`
  (`ns_onboarding_status` select), and snapshot `contactable_snapshot` (built-in
  `email` text) definitions into the cumulative root. The demo has no
  memberships or record values. It uses `depends_on` on the CRM schema module
  and pins the root to `= 0.8.0`. Demo CI validates against provider candidate
  `3d3812f` until ticket 07 repins it.
- 2026-10-29: The journey adds a `revise` phase after `repair`. It renames in
  place, replaces the snapshot, and reads the old snapshot tombstone by exact
  ID. `refresh` tombstones the dynamic definition and restores the same ID.
  `adopt` imports a tombstoned manual definition and restores the same ID.
  `destroy-apply` reads each exact tombstone before terminal cleanup. The
  Northstar helper gains `verify-contact-segment-tombstone`, which reads once
  and never deletes. Verification and drift now take the name label. The
  maintenance phase order is asserted exactly.
- 2026-10-29: Segment names carry a `<files prefix><engine>` label, and
  `revise` persists rather than reverts. As a result, no engine leg or
  replacement recreates a name that a retained tombstone may hold. Whether
  HubSpot reserves tombstoned names is unproven live. Separate weekly runs still
  share the `ns_` prefix unless maintenance sets a per-run
  `HUBSPOT_NORTHSTAR_FILES_PREFIX`. The ticket 07 protected run will reveal
  this first.
- 2026-10-29: Local gates passed:
  - Provider `make check` passed.
  - `make test-hermetic` passed 56 tests with 0 failures and 0 skips, with
    `HUBSPOT_DEMO_REPO` at the demo branch. OpenTofu 1.13.2 and Terraform 1.17.1
    ran the new `TestHermeticContactSegmentModuleLifecycle` and its Terraform
    parity test through the real module. That test covers create, no-op,
    dynamic drift and repair, rename, snapshot replacement, stable-key add and
    remove, tombstoned import with same-ID restore, and destroy.
  - Demo `make check` passed with its pinned OpenTofu 1.12.3 and Terraform
    1.15.8, including `demo_test` and module and root mock tests under both
    engines.
  - Demo PR CI passed.
- 2026-10-29: Live dual-engine execution of the full Northstar journey is
  expected in ticket 07's protected maintenance run, after ticket 06 or 07
  pins the merged demo commit. Release-train-recovery ticket 07 still reads
  open, but this work is built on its rebased branch and draft PR #119. Two
  local rough edges are known:
  - `drift-contact-segments` assumes the baseline snapshot value, so
    `drift` works only before `revise`.
  - A journey that fails after `revise` leaves its marker under `.demo/`
    until `destroy-apply`. Ephemeral runners are unaffected.
