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
- 2026-08-16: Provider PR 87 merged the 142-second exact descendant GET window
  as `0b364529b5eb330bae0f841898c6876e974d8c27`. Exact-main Required run
  31915598967 passed. Protected maintenance run 31915696685 crossed the prior
  87-second boundary and again received terminal completion for the documented
  asynchronous hierarchy update, but the exact child-folder GET remained stale
  through the full 142-second repair verification. Further delay is no longer
  justified. The follow-up keeps generated ID as the sole identity and adds a
  parent-scoped folder search as an independent read-back surface, accepting
  only the result whose generated ID exactly matches the known child ID.
- 2026-08-16: Provider PR 88 merged exact-ID parent-scoped Search read-back as
  `bd84f5baf13ac3e6831df06fe0ed651702aed5b6`. Exact-main Required run
  31916371791 passed. Protected maintenance run 31916459611 proved Search
  retained the stale child path for 142 seconds. HubSpot had reported the parent
  update task complete. Demo PR 27 added an explicit guarded child-path repair
  after the targeted parent apply. The PR merged as
  `1dac3295f66ebe35d3aeeee32ef6843366989016`.
  The provider follow-up pins that exact demo merge and implements the helper
  action with exact parent, child, placement, and owned-name preconditions.
- 2026-08-16: Provider PR 89 merged child-update repair as
  `52af98d9c23fc24d4079eaae109ec691e78a7878`. Exact-main Required run
  31917402328 passed. Protected run 31917496856 showed a terminal same-value
  child update did not recompute its path. The follow-up moves only two exact
  owned files, renames the empty child away and back, restores both files, and
  verifies every identity.
- 2026-08-16: Demo PR 28 passed hosted run 31918561660 and merged as
  `c3921ae7a86bdaabdd2524127212a2c58d640f4d`. Provider maintenance pins that
  exact merge. Fixed-point specification, standards, test-quality, and Actions
  reviews found no remaining hard issues.
- 2026-08-16: Provider PR 90 merged the guarded rename cycle as
  `769c25b178635d45451c84d24b82e7cda712db1b`. Exact-main Required run
  31918918820 passed. Protected run 31919037400 then failed earlier during
  OpenTofu drift repair. A local removal-guard update returned unconfigured
  computed membership names as unknown. The follow-up preserves observed state
  and changes only the planned local guard. Real OpenTofu and Terraform
  regression tests cover the fix.
- 2026-08-16: Provider PR 91 merged the local membership-state fix as
  `01518e6830fdc142ab386b47757e7c1c3da62d40`; exact-main Required run
  31919639422 passed. Protected maintenance run 31919990382 then completed the
  Product and cumulative lifecycle teardown, but terminal verification rejected
  the exact CRM profile `404` caused by separately removing its account
  membership. The follow-up keeps CRM-profile destroy write-free and accepts
  projection absence only after exact membership absence and aggregate CRM
  collection absence are both proven.
- 2026-08-16: Provider PR 92 merged the guarded profile-terminal verifier as
  `81ddb2b09d816cacfef6d245a44b4025cecc3aaf`; exact-main Required run
  31920665393 passed. Protected maintenance run 31920769235 crossed the prior
  profile `404` failure and emitted the new exact-absence record. The pinned
  demo still required only the retained-values record and stopped before later
  terminal assertions. Demo PR 29 now accepts only those two enumerated
  terminal outcomes, tests both, passed hosted run 31921173435, and merged as
  `793256402c5e1e811cd374f0815a83c66f092616`.
- 2026-08-16: Provider PR 93 pinned that exact demo merge and merged as
  `1c73fcc0bf494ce5fba4d2d0a344084e79e89d47`; exact-main Required run
  31921499705 passed. Protected maintenance run 31921593549 completed the
  OpenTofu journey and reached Terraform refresh repair. HubSpot reported the
  asynchronous root-folder restore complete, but exact-ID GET returned the
  preceding name for the provider's full bounded read-back window. The
  follow-up types and verifies the documented terminal task `result`, accepts
  it only for the same generated ID and complete planned state, and treats a
  later GET as stale only when its `updatedAt` is older than that verified
  revision. Dual-engine hermetic tests reproduce the stale read window.
