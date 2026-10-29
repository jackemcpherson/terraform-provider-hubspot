# 04 Decide and Ship Any v0.7.x Patch

Type: task

Status: resolved

Label: ready-for-agent

Blocked by: 03

## What to Build

Registry users get the patched standard library without waiting for v0.8.0,
but only when the repair changes shipped provider binaries.

## Acceptance

- Record whether any reachable finding from ticket 01 reaches the provider
  binary itself. Findings that reach only the docs-portal server or provenance
  tools do not require a patch.
- If a patch is required, publish one signed immutable v0.7.x release from the
  qualified `main` commit, verify its complete asset set, and record Terraform
  Registry and OpenTofu Registry ingestion separately.
- If no patch is required, record the reasoning and close the ticket.

## Comments

## Answer

Verdict: a patch is required. v0.7.1 ships it.

- Ticket 01 recorded 10 findings reachable from the shipped provider binary
  itself: GO-2026-6603, 6605, 6607, 6608, 6610, 6611, 6612, 6613, 6617, and
  6809. The spec requires a patch for any finding that reaches the provider
  binary. The docs-portal and provenance-only findings do not change that.
- `main` since v0.7.0 changes the provider binary in three ways. PR 115 is
  the Go 1.26.10 and `golang.org/x/net` v0.62.0 repair. PR 107 updates gRPC
  to v1.83.2. PR 98 fixes the File folder stale-task result. PR 98 is a
  bounded bug fix with hermetic coverage. Provider `18224aa` contains it, and
  that commit passed both protected cumulative baseline runs in ticket 03, so
  PR 98 is ready to ship. Every other change since v0.7.0 is tests, CI,
  scripts, documentation, or the acceptance harness, which the provider binary
  does not import.
- The module language version moved to 1.26.0 in ticket 01, and the
  maintainer approved it. The changelog records it under Changed.
- Publication uses the existing `main`-only `Release` workflow. It needs no
  release branch or candidate workflow, so it stays within ADR 0003.

Run and release URLs are recorded outside Git, in a comment on the pull
request that resolves this ticket, as release-slices section 7 requires.
