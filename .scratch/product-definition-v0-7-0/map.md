# Product Definition v0.7.0 Map

This map tracks the append-only work needed to publish v0.7.0.

## Frontier

- None. The remaining live-contract, documentation-pin, and release tickets are
  claimed or blocked.

## Decisions So Far

- The frozen contract is in [the v0.7.0 specification](spec.md).
- Official research is in
  [the 2026-03 Products contract](research/2026-03-products-contract.md).
- Root placement uses omission, subject to the guarded live publication probe.
- The [typed Product client](issues/02-add-typed-product-client.md) owns exact
  `2026-03` HTTP identity and recovery behavior.
- The [Product resource](issues/03-add-product-resource.md) owns the frozen
  Terraform state contract and semantic decimal handling.
- The [lifecycle and cleanup work](issues/04-extend-lifecycle-and-cleanup.md)
  proves both engines, guarded archival, and cumulative helpers.
- The [cumulative demo](issues/05-extend-cumulative-demo.md) composes the
  stable-keyed module without touching the existing dirty worktree.
