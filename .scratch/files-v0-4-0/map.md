# Files v0.4.0 delivery map

**Status:** resolved

## Decisions so far

- [06 - Prove and maintain the protected Files shard](issues/06-prove-and-maintain-protected-files-shard.md):
  Added protected exact-scope, exact-candidate lifecycle proof for both engines,
  sanitized evidence, read-only scheduled reporting, and fail-closed exact-prefix
  cleanup.
- [07 - Integrate cumulative and released Files journeys](issues/07-integrate-cumulative-and-released-journeys.md):
  Integrated exact-ID Files configuration into cumulative dual-engine Northstar.
  Added the registry-installed, source-migrated released journey with
  checksum-bound packages and sanitized cleanup evidence.
- [08 - Prepare v0.4.0 for publication](issues/08-qualify-exact-v0-4-0-candidate.md):
  ADR 0003 reduced v0.x qualification to the fast required gate, separate weekly
  maintenance, and fail-fast immutable publication preflight. All local release
  checks and the unpublished platform build pass.
