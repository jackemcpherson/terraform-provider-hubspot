# 07 Review, Merge, and Release

Type: task

Status: claimed

Blocked by: 01, 02, 03, 04, 05, 06

## Acceptance

- Pass all named local and hosted gates.
- Clear standards, specification, test-quality, Actions, and release reviews.
- Merge demo before provider and publish one immutable signed v0.7.0 release.
- Record Terraform and OpenTofu Registry ingestion separately.

## Comments

- 2026-08-15: Local required, hermetic, race, fuzz, security, release-contract,
  and complete demo gates passed. Reviews, hosted gates, merges, live preflight,
  and immutable publication remain.
- 2026-08-15: Demo PR 24 and provider PRs 79, 80, and 81 are merged. Exact-main
  Product preflight passed in protected maintenance run 31871546257. Cumulative
  Northstar maintenance and publication remain blocked until the protected
  `HUBSPOT_NORTHSTAR_MEMBERSHIP_EMAIL` variable is configured.
