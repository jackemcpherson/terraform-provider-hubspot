# 07 Rebase the Contact Segment Slice and Open Its Pull Request

Type: task

Status: resolved

Label: ready-for-agent

Blocked by: 03, 05

## What to Build

Reviewers can review the Contact segment typed client, resource, and lifecycle
work as one pull request qualified against the patched toolchain and current
runner.

## Acceptance

- Rebase, not merge, commit `150c8c0` on
  `release/v0.8.0-contact-segment-client` onto the repaired `main`.
- Keep the frozen v0.8.0 specification, ADR 0003, and ADR 0004 authoritative.
- Rerun `make check`, `make test-hermetic`, `make test-race`,
  `make fuzz-seeds`, `make check-security`, and `make check-release` green
  after the rebase.
- Push the branch and open one pull request containing the typed client,
  resource, and lifecycle work.
- Unblock contact segment tickets 05, 06, and 07, which remain the units of
  v0.8.0 completion.

## Comments

## Answer

The Contact segment slice is rebased onto repaired `main` and open for review
as draft PR 119.

- `150c8c0` was rebased, without merges, onto `9d21c62` (v0.7.1 preparation)
  and is now `6526fd6` on `release/v0.8.0-contact-segment-client`. The branch
  also carries the docs commit that orders the slice after this effort, and a
  release-note correction that follows ADR 0005.
- The append-only Contact segment map kept every line from both sides. The
  changelog keeps the slice entries under `[Unreleased]` and leaves `[0.7.1]`
  identical to `main`.
- `make check`, `make test-hermetic`, `make test-race`, `make fuzz-seeds`,
  `make check-security`, and `make check-release` passed on the rebased tip
  under Go 1.26.10, OpenTofu 1.13.2, and Terraform 1.17.1, with a clean demo
  clone at `2ef32f7`. `Required` passed on PR 119.
- Contact segment tickets 05, 06, and 07 are unblocked by this effort, in
  that order.
