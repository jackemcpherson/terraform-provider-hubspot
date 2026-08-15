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
- 2026-08-15: Provider PR 84 merged the staged-file convergence guard and demo
  pin as `f2636e0add3b5044087013ecb001d74d07d65db1`. Exact-main Required run
  31876363264 passed. Protected maintenance run 31876456472 crossed the prior
  combined-repair failure and completed the targeted parent-folder rename. The
  Downloads child-folder path had not propagated within the existing 32-second
  verification window. Staged verification therefore failed closed before the
  private file was moved home. The follow-up retains the two-apply sequence and
  extends only bounded descendant-path convergence beyond the observed
  32-second delay.
- 2026-08-15: Provider PR 85 merged the bounded descendant-path convergence as
  `bd3631d9587ae0fc2daa4ac24c2858367b24c6ed`. Exact-main Required run
  31877336609 passed. Protected maintenance run 31877434853 passed guarded
  cleanup, Product creation and drift repair, Product external archival, and
  OpenTofu refresh-only state removal. The targeted parent-folder repair then
  completed, but the Downloads child-folder path remained stale for the full
  87-second bound. HubSpot documents that the asynchronous folder-update API
  updates all children. The provider follow-up therefore uses that endpoint for
  an ancestor rename with direct child folders, waits for terminal task
  completion, and retains exact descendant-path verification.
- 2026-08-16: Provider PR 86 merged the asynchronous hierarchy update as
  `7429126b83edd79030b732b8f9b3b17476f9666f`, and exact-main Required run
  31878843313 passed. Protected maintenance run 31878944027 passed the Product
  preflight and the earlier OpenTofu lifecycle seams, but the exact descendant
  path read remained stale for the full 87-second post-task window. The next
  follow-up changes no resource semantics: it extends only the exact descendant
  GET convergence boundary beyond the live-observed window and gives the two
  affected helper actions enough enclosing time to complete or fail closed.
