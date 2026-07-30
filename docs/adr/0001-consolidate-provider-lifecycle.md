# ADR 0001: Consolidate the Provider Lifecycle at Trust Boundaries

This decision record defines the provider validation and release workflow shape.

- Status: Accepted.
- Date: 2026-07-20.

---

## Context

The provider had seven workflows for validation, release, and cleanup. Separate
workflow dispatches passed candidate reports and repeated release build logic.
This design made the operator interface difficult to review. It also let local
and CI validation differ.

Neither public registry provides a complete pre-publication test. The closest
test must build and install the real release shape locally. It must also validate
the manifest, checksums, archives, and software bills of materials (SBOMs).

## Decision

Keep three workflows at separate trust boundaries:

1. `validate-provider.yml` checks pull requests, `main`, and scheduled security
   analysis. It preserves the required status named `Required`.
2. `run-provider-lifecycle.yml` checks live source health and publishes a
   selected version. It binds each release to the checked commit on `main`.
3. `archive-crm-configuration.yml` provides manual cleanup for the supported
   capability shard. It requires an owned prefix and an exact confirmation.

The `scripts/build-release-bundle.sh` script builds every release bundle. Local
checks and both CI builds use this script. The pipeline compares two independent
builds and promotes the first verified bundle.

Release jobs use narrow permissions. Only the protected signing step receives
the private GPG key. Only attestation receives OpenID Connect write permission.
Only publication jobs receive repository content write permission.

Post-release checks wait for both registries. They compare registry downloads
with GitHub assets. They then test both provider lifecycles and state migration
in one disposable portal window.

## Consequences

The operator starts one `Provider lifecycle` workflow with a version. A rerun of
that version resumes a verified draft or registry check.

Mismatched tags, commits, releases, or assets stop the workflow. Maintainers must
investigate the mismatch instead of changing an immutable release.

Policy tests enforce the workflow count, permission boundaries, engine matrix,
shared builder, and registry checks. Dependabot continues to manage action and
Go dependency updates.
