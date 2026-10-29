# 07 Rebase the Contact Segment Slice and Open Its Pull Request

Type: task

Status: open

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
