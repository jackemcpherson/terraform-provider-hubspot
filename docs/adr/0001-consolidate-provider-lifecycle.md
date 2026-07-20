# ADR 0001: Consolidate the provider lifecycle at trust boundaries

- Status: Accepted
- Date: 2026-07-20

## Context

The provider had seven GitHub Actions workflows for quality, security,
acceptance, candidate qualification, release, release verification, and manual
cleanup. The release path passed a candidate report between separately dispatched
workflows and repeated tool setup and release-build logic. That made the operator
interface difficult to understand and allowed local and CI release validation to
drift. The provider must publish one immutable artifact set that works through
both the Terraform and OpenTofu registries.

Neither public registry offers a complete pre-publication dry run. The closest
reliable pre-flight is therefore to construct the real release shape locally,
validate its manifest, checksum closure, archives, and SBOMs, and install those
archives under both registry identities through filesystem mirrors.

## Decision

Keep three workflows, aligned to distinct trust boundaries:

1. `validate-provider.yml` handles pull requests, pushes to `main`, and scheduled security
   analysis. It runs the repository's local aggregate and release pre-flight,
   the complete supported Terraform/OpenTofu engine matrix, vulnerability and
   workflow scans, CodeQL, scheduled Scorecard analysis, and preserves the single
   branch-protection context named `Required`.
2. `run-provider-lifecycle.yml` handles scheduled live source health and an explicitly
   dispatched release version. A release is bound to the workflow commit at the
   head of `main` and its successful `Required` check. New releases qualify live
   source, build the same real-version asset set twice, compare it, wait for the
   protected `release` environment before exposing signing credentials, attest,
   and publish. The same version may be rerun: verified drafts resume publication
   and verified published releases resume registry and live verification.
3. `archive-crm-configuration.yml` is a manual break-glass operation for the only
   supported capability shard. It requires an exact owned prefix and the literal
   confirmation `archive-prefixed-crm-configuration`.

The local `scripts/build-release-bundle.sh` is the sole release-bundle builder for
developer pre-flight and both CI builds. The release assets are independently
reproduced and compared. The pipeline builds one bundle for publication;
downstream signing and publishing promote that artifact rather than rebuilding it.
The independent second bundle is comparison evidence only; its volatile attestation
metadata is not promoted.

Release permissions are job-scoped. The private GPG key exists only in the
protected signing step, OIDC write permission exists only in attestation, and
repository content write permission exists only in publication jobs. All hosted
runners and third-party actions are immutably pinned.

Released-provider verification waits for both registries, verifies registry
downloads against GitHub assets, then performs the Terraform lifecycle, OpenTofu
lifecycle, and bidirectional state migration inside one serialized disposable
portal teardown/restoration window.

## Consequences

The normal operator interface is one workflow and one input: run `Provider
lifecycle` from `main` with a v-prefixed SemVer. Rerunning that version is the
recovery and OpenTofu-bootstrap path. Mismatched tags, releases, commits, or assets
fail closed and require investigation rather than mutation of an immutable release.

The workflow policy test treats the three-file surface, permission boundaries,
engine matrix, local builder, and both registry verification paths as executable
architecture constraints. Dependabot continues to track GitHub Actions and Go
dependencies centrally.
