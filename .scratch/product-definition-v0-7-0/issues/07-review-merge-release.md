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
- 2026-08-15: The protected variable now uses the approved reserved fixture.
  Maintenance run 31872888242 passed the Product probe and reached the live
  OpenTofu refresh phase. The run correctly archived the Product, then stopped
  because the root demo output indexed the temporarily empty Product ID map.
  Demo PR 25 fixed and tested the nullable refresh output. The provider recovery
  change adds guarded pre-run cleanup for exact owned residuals before the gate
  is rerun.
- 2026-08-15: Provider PR 83 merged the guarded recovery as
  `280c5249174add2f145b02212ffa1c960f670b06`, and exact-main Required run
  31874754037 passed. Protected maintenance run 31874843419 then proved cleanup,
  Product recreation, and nullable refresh handling before HubSpot rejected a
  combined ancestor-folder rename and direct-file move. Demo PR 26 now stages
  the exact file, repairs and verifies the empty parent separately, and then
  moves the file home. The provider follow-up adds bounded exact-ID and complete
  parent-search convergence proof before folder drift.
