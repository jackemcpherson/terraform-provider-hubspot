# Product Definition v0.7.0 Map

This map tracks the append-only work needed to publish v0.7.0.

## Frontier

- None. The release ticket is claimed while protected Northstar configuration
  and publication remain.

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
