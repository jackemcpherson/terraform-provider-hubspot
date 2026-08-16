# Product Definition v0.7.0 Map

This map tracks the append-only work needed to publish v0.7.0.

## Frontier

- None. Every implementation ticket is resolved. Immutable publication runs
  from the final reviewed main commit after its Required and protected
  maintenance checks pass.

## Decisions So Far

- The frozen contract is in [the v0.7.0 specification](spec.md).
- Official research is in
  [the 2026-03 Products contract](research/2026-03-products-contract.md).
- The [live Product contract](issues/01-validate-live-contract.md) proves that
  omitted `hs_folder` creates at the account-independent root.
- The [typed Product client](issues/02-add-typed-product-client.md) owns exact
  `2026-03` HTTP identity and recovery behavior.
- The [Product resource](issues/03-add-product-resource.md) owns the frozen
  Terraform state contract and semantic decimal handling.
- The [lifecycle and cleanup work](issues/04-extend-lifecycle-and-cleanup.md)
  proves both engines, guarded archival, and cumulative helpers.
- The [cumulative demo](issues/05-extend-cumulative-demo.md) composes the
  stable-keyed module without touching the existing dirty worktree.
- The [documentation ticket](issues/06-publish-documentation.md) publishes the
  resource, operational, portal, and v0.7.0 release surfaces and pins the exact
  merged demo.
- The [release ticket](issues/07-review-merge-release.md) records the fixed-point
  reviews, hosted checks, live lifecycle evidence, and immutable publication
  decision.
