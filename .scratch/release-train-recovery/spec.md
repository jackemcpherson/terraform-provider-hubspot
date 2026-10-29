# Release Train Recovery Specification

Label: ready-for-agent

Status: open

This specification covers the work needed to bring the release train back to
health, finish the v0.8.0 Contact segment definition slice, amend the
configuration-surface release order after v0.8.0, and clear two pieces of
local tooling drift.

## Problem Statement

The maintainer returned after about ten weeks away and found the release train
stalled.

Scheduled protected maintenance on `main` has failed every week since
2026-10-12. `govulncheck` reports 13 reachable vulnerabilities in the pinned Go
1.26.6 standard library and in `golang.org/x/net` v0.58.0. The two-run green
baseline recorded on 2026-08-18 no longer describes `main`, which has since
taken dependency, runner, and toolchain changes. Release-slice discipline
treats an inherited baseline failure as maintenance work that blocks the
active slice.

The v0.8.0 Contact segment definition slice has its typed client, resource,
and lifecycle proof complete in one unpushed commit on a branch that is five
commits behind `main`. The cumulative demo module, final documentation and demo
pin, fixed-point reviews, exact-commit qualification, and signed publication
remain.

The next planned surface, association definition at v0.9.0, needs paid-tier
qualification. That conflicts with the Free-portal delivery contract, so the
release order no longer describes work that can ship.

Local verification is also fragile. The Makefile, validation workflow, and
maintenance workflow pin OpenTofu 1.12.3 and Terraform 1.15.8 as "current",
although newer engines are in use. The acceptance harness derives the
repository root from a case-variant working directory, which breaks the
provider build on a case-insensitive filesystem.

## Solution

Work proceeds in four phases. Each phase gates the next where noted.

1. **Baseline repair.** Raise the pinned Go toolchain to a patched release
   and `golang.org/x/net` to a patched version. Prove `make check-security`
   is clean, merge through `main`, and re-establish two consecutive green
   protected maintenance runs on unchanged provider and demo commits.
2. **v0.8.0 completion.** Rebase the Contact segment branch onto the repaired
   `main`. Open its pull request, land the cumulative demo module, pin the
   merged demo commit, freeze documentation, pass every local and hosted
   gate, and publish one signed immutable v0.8.0 release.
3. **Release-order amendment.** Defer association definition indefinitely.
   Deal pipeline configuration becomes v0.9.0 and ticket pipeline
   configuration becomes v0.10.0. Record this through the explicit amendment
   path that the release rules require.
4. **Tooling hygiene.** Move the "current" engine pins to the current
   OpenTofu and Terraform releases, and make repository-root discovery
   independent of working-directory spelling.

Phase 4's engine-pin change affects maintenance gates, so it lands in phase 1,
before the baseline runs. The repository-root fix may land at any point
before v0.8.0 source freeze.

## User Stories

### Baseline repair

1. As the maintainer, I want the pinned Go toolchain to include the fixes for
   every reported standard-library vulnerability, so that weekly maintenance
   stops failing on `govulncheck`.
2. As the maintainer, I want `golang.org/x/net` raised to a version that fixes
   the reported vulnerabilities, so that the provider and its tools stop
   reaching vulnerable HTTP/2 code.
3. As the maintainer, I want one Go version source to drive local checks, CI,
   and release builds, so that a toolchain bump cannot leave one path on a
   vulnerable release.
4. As the maintainer, I want `make check-security` to pass locally before I
   open the repair pull request, so that hosted runs are not used to discover
   local failures.
5. As the maintainer, I want two consecutive green protected maintenance runs
   on the same provider and demo commits after the repair, so that v0.8.0
   qualifies against a proven baseline.
6. As the maintainer, I want each failed scheduled run recorded in a failure
   ledger with phase, run, commits, cause, and fix, so that release history
   explains the gap.
7. As the maintainer, I want the baseline repair to land as its own pull
   request, separate from the Contact segment slice, so that a cumulative fix
   is never hidden in a surface release.
8. As the maintainer, I want the repair released as a patch on the v0.7.x
   line only if it changes shipped provider binaries, so that registry users
   get the patched standard library without waiting for v0.8.0.

### v0.8.0 completion

9. As the maintainer, I want the Contact segment branch rebased onto the
   repaired `main`, so that it is qualified with the patched toolchain and the
   current runner.
10. As the maintainer, I want the full local suite rerun after the rebase, so
    that rebasing introduces no regression.
11. As a reviewer, I want one pull request containing the typed client,
    resource, and lifecycle work, so that I can review the slice as one unit.
12. As a Terraform author, I want the Northstar demo to show manual, dynamic,
    and snapshot Contact segment definitions, so that I can copy a working
    pattern.
13. As the maintainer, I want the demo module keyed by stable map keys, so
    that adding or removing one definition never re-addresses the others.
14. As the maintainer, I want the demo to reference existing Contact text and
    select property names, so that it creates no Contact memberships or record
    values.
15. As the maintainer, I want the cumulative journey to exercise create,
    no-op, rename, dynamic drift and repair, snapshot replacement, refresh,
    active import, tombstoned import and restore, destroy, and terminal
    cleanup, so that every frozen contract obligation has live proof.
16. As the maintainer, I want that journey to pass under both OpenTofu and
    Terraform, so that either engine is supported.
17. As the maintainer, I want the demo change built in an isolated checkout,
    so that the existing dirty demo worktree is preserved untouched.
18. As the maintainer, I want the demo change merged first and its exact
    merge commit pinned in provider maintenance, so that provider
    qualification never runs against a moving demo reference.
19. As a Terraform author, I want generated resource reference, an executable
    example, and surface, permissions, import, drift, destroy, and
    troubleshooting guidance, so that I can adopt the resource without reading
    source.
20. As a Terraform author, I want a dated changelog entry and v0.8.0 release
    notes, so that I know what changed and how to upgrade.
21. As the maintainer, I want generated documentation and the portal source
    digest reproduced from the frozen source, so that published docs match the
    released binary.
22. As the maintainer, I want standards, specification, test-quality, Actions,
    and release-readiness reviews cleared at one fixed point, so that the
    release candidate is reviewed as frozen.
23. As the maintainer, I want green `Required` and protected maintenance on
    the exact merged `main` commit, so that the release is qualified exactly
    as ADR 0003 requires.
24. As the maintainer, I want one signed immutable v0.8.0 release with
    verified assets, so that users get reproducible, authenticated binaries.
25. As the maintainer, I want Terraform Registry and OpenTofu Registry
    ingestion recorded separately, so that asynchronous registry lag is not
    mistaken for a failed release.
26. As the maintainer, I want post-freeze run URLs recorded outside Git, so
    that recording evidence does not move the qualified commit.

### Release-order amendment

27. As the maintainer, I want association definition deferred indefinitely,
    so that the release order contains only surfaces a Free portal can
    qualify.
28. As the maintainer, I want deal pipeline configuration renumbered to
    v0.9.0, so that the next slice can start immediately after v0.8.0.
29. As the maintainer, I want ticket pipeline configuration renumbered to
    v0.10.0, so that it still follows and reuses the deal pipeline
    implementation.
30. As the maintainer, I want the amendment recorded through the explicit
    rescheduling path, keeping one surface per release and the dependency
    graph intact, so that the change follows the release rules.
31. As the maintainer, I want the conflict between "requires paid-tier
    qualification" and the Free-only contract resolved in the written record,
    so that a future session does not plan association work on a Free portal.
32. As a future agent, I want the conditions for restoring association
    definition stated, so that I know what evidence would reopen it.

### Tooling hygiene

33. As the maintainer, I want the "current" OpenTofu and Terraform pins to
    name the current stable releases, so that "current engine" qualification
    means what it says.
34. As the maintainer, I want every engine pin in local checks and workflows
    to come from consistent values, so that local and hosted runs cannot test
    different engines.
35. As the maintainer, I want the minimum-supported engine versions kept
    separate from the current pins, so that a current-pin bump never removes
    compatibility proof.
36. As the maintainer, I want acceptance tests to find the repository root
    whatever the case of the working directory path, so that local runs work
    on a case-insensitive filesystem.
37. As the maintainer, I want a regression test for that root discovery, so
    that the failure cannot return silently.

## Implementation Decisions

### Baseline repair

- The Go toolchain pin moves to the newest patched Go 1.26 point release that
  is at least 1.26.10. That clears GO-2026-6809 (`crypto/x509`, fixed in
  1.26.10) and the `net/http`, `net/textproto`, `crypto/tls`, and
  `html/template` findings fixed in 1.26.9. The language version in the module
  directive does not change.
- `golang.org/x/net` moves to at least v0.60.0. That clears GO-2026-6617,
  6612, 6611, 6610, and 6603. Other module bumps are limited to what tidiness
  requires.
- The Makefile exact-version guard, the module toolchain line, and the
  workflows' Go setup must agree. Provider PR 109 established one toolchain
  source; the bump edits that source, not parallel copies.
- The repair is a provider-only change. The demo pin stays at `2ef32f7` for
  the baseline runs.
- A failure ledger entry covers the 2026-10-12, 2026-10-19, and 2026-10-26
  scheduled failures. It belongs to a maintenance ticket outside the
  Contact segment slice.
- Whether to cut a v0.7.x patch is decided after the repair merges. A patch
  is required only if the patched standard library changes shipped provider
  binaries, which it does for any reachable finding in the provider binary
  itself. Findings that reach only the docs-portal server or provenance tools
  do not require a patch.

### v0.8.0 completion

- The Contact segment branch is rebased, not merged, onto repaired `main`.
  The trial merge from 2026-10-29 shows no conflicts.
- The frozen v0.8.0 specification, ADR 0004 (author-declared
  `property_kind`), and ADR 0003 (qualify exact merged `main` commits only)
  stay authoritative. No candidate workflow is added; the protected
  candidate prerequisite in release-slice discipline stays unimplemented.
- The demo module accepts a map of definitions keyed by stable names. Each
  value selects a processing type and, for dynamic and snapshot, OR groups of
  AND predicates using existing Northstar Contact text and select properties.
- The demo journey includes a tombstoned import and same-ID restore. Destroy
  leaves a restorable tombstone, and terminal cleanup verifies it through
  exact-ID reads.
- Tickets 05, 06, and 07 of the Contact segment slice remain the units of
  work. This specification does not renumber or replace them; it orders them
  after baseline repair.

### Release-order amendment

- Association definition is removed from the numbered release order and
  listed as deferred. Restoring it requires a paid-tier qualification portal,
  a protected environment for it, and an ADR that admits paid-tier evidence
  into the delivery contract.
- Deal pipeline configuration becomes v0.9.0 and ticket pipeline
  configuration becomes v0.10.0. Their scope, ordering rationale, and shared
  internal pipeline implementation are unchanged.
- The amendment is recorded as an ADR. It names the release-order decision it
  amends, keeps the one-surface-per-release rule, and confirms that neither
  pipeline surface depends on association definition.
- The v0.8.0 map decision that says association definitions are "deferred to
  v0.9.0" gets an appended correction pointing at the ADR. It is not edited in
  place, because the map is append-only.

### Tooling hygiene

- "Current" engine pins move to the current stable OpenTofu and Terraform
  releases at implementation time, verified against each project's release
  page. Minimum-supported engine versions and recorded state fixtures stay as
  they are.
- The current pins appear in the Makefile, the validation workflow, and the
  maintenance workflow. All three change together, and the workflow checks
  should reject disagreement where they can.
- Repository-root discovery in the acceptance harness resolves the working
  directory to its canonical on-disk form before walking up to the module
  root. It must never return a path that the Go command treats as outside the
  main module.

## Testing Decisions

- A good test drives external behavior through an existing boundary and
  asserts observable outcomes: plan and apply results, state, remote fake
  state, request counts, and command exit status. Tests do not assert on
  private helpers or internal call order.
- Vulnerability repair is proven by `make check-security` (`govulncheck`)
  locally, then by two consecutive green protected maintenance runs. No new
  test is written for a version bump.
- Engine-pin consistency is proven by `make check`, its engine smoke test, and
  the existing workflow checks. The workflow-check script is the place to add
  a guard if pins can disagree.
- Contact segment behavior stays proven through the existing behavioral
  HubSpot fake and the dual-engine acceptance harness. Prior art: the hermetic
  Contact segment lifecycle and recovery tests, and their Terraform parity
  variants, which passed on 2026-10-29.
- The demo module is proven through the existing Northstar lifecycle command
  and the cumulative journey in protected maintenance. Prior art: earlier
  surfaces' Northstar lifecycle phases and their unit tests.
- Repository-root discovery gets one regression test at the harness entry
  point. It simulates a case-variant working directory, then asserts that the
  resolved root builds the provider. Prior art: the existing harness and
  property harness tests, which fail today under that condition.
- The release gate is the full local suite on the frozen source: `make
  check`, `make test-hermetic`, `make test-race`, `make fuzz-seeds`, `make
  check-security`, and `make check-release`. Hosted gates follow on the exact
  merged `main` commit.

## Out of Scope

- Implementing deal or ticket pipeline configuration. This specification only
  renumbers them; each gets its own slice under release-slice discipline.
- Any association definition work, including probes on a paid portal.
- A protected release-candidate workflow or an ADR superseding ADR 0003.
- Contact segment features beyond the frozen v0.8.0 contract, such as
  multi-value, numeric, datetime, event, association, or historical
  predicates.
- Contact memberships, record values, or derived segment size.
- Changes to the existing dirty demo worktree.
- A Go language-version bump or a move to Go 1.27.

## Further Notes

- Recommended order: baseline repair with the engine pins, then
  the repository-root fix, the v0.8.0 rebase, the PR, the demo module, docs,
  reviews, and release. The release-order amendment can land any time before
  v0.9.0 work starts, but it should land before v0.8.0 release notes mention
  what comes next.
- The Contact segment work is commit `150c8c0` on branch
  `release/v0.8.0-contact-segment-client`, which is not yet pushed.
- The protected environment variables and scopes must be confirmed without
  reading secret values before the baseline runs, as release-slice discipline
  step 1 requires.
- Hosted runners are free for these public repositories, so the run budget is
  bounded by the slice discipline table, not by cost.
