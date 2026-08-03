# 10 — Verify released Forms through one migrated identity

**What to build:** Extend released-provider verification so both public registry packages operate the same live Form definition and preserve its generated identity across both engines before one final archive.

**Blocked by:** 09 — Add Forms to the cumulative Northstar lifecycle.

**Status:** claimed

- [ ] Released verification installs the requested version from Terraform Registry and OpenTofu Registry and binds each selected archive digest to the immutable GitHub release assets.
- [ ] The journey creates one uniquely prefixed Form definition once and records its generated identity safely for serialized reuse.
- [ ] The same state and remote ID move from Terraform's provider source to OpenTofu's source and back, with both engines reading, planning, updating, drifting, repairing, and reaching an empty plan.
- [ ] No migration step creates a second active form, matches by name, or archives the shared fixture early.
- [ ] The shared form is archived exactly once after every source/engine phase and its active absence plus exact terminal identity are verified.
- [ ] The released cumulative Northstar lifecycle runs under both engines after package/digest verification and includes the Form definition surface.
- [ ] Sanitized evidence records version, exact provider/demo commits, archive digest, engines, registry sources, timestamps, state-migration result, and cleanup result without raw remote identifiers.
- [ ] Script contract tests cover ordering, early failure, identity preservation, cleanup traps, registry mismatch, wrong version, and incomplete evidence without requiring a live portal.
- [ ] The released journey can be prepared and tested before publication but cannot report completion until real v0.3.0 registry packages and protected live credentials are available.
