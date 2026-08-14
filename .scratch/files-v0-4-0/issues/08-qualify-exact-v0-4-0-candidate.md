# 08 - Prepare v0.4.0 for Publication

**What to build:** Prepare the additive v0.4.0 Files configuration release for
the small v0.x publication transaction defined by ADR 0003.

**Type:** task

**Blocked by:** 01, 02, 03, 04, 05, 06, 07.

**Status:** resolved

- [x] The provider registers the two Files resources and preserves all earlier
  public resources, data sources, and state upgrades.
- [x] The changelog, release notes, generated references, examples, permissions,
  and lifecycle guides describe the v0.4.0 contract and exclusions.
- [x] The local and pull-request `Required` gate passes with current
  OpenTofu and Terraform versions.
- [x] Vulnerability, workflow security, release configuration, artifact naming,
  checksum, signature, and Registry manifest checks pass.
- [x] Exact clean-checkout preflight validates the dated changelog and matching
  release notes before publication can create a signed tag.
- [x] An unpublished GoReleaser build produces all 13 supported platform
  archives and the checksum inventory.
- [x] v0.4.0 has no existing tag or GitHub release.
- [x] Live acceptance, historical engine matrices, candidate bundles, registry
  polling, and released-provider journeys remain outside merge and publication
  under ADR 0003.

## Answer

ADR 0003 replaced the original exhaustive candidate transaction with one fast
required gate and separate weekly maintenance. The completed Files resources,
typed clients, behavioral fake, current-engine lifecycle checks, module,
documentation, and release metadata remain in the release.

The required gate, security checks, release checks, exact-commit preflight, and
full unpublished platform build pass. Release preflight now rejects missing,
empty, or mismatched version notes before the publication job creates a signed
tag. v0.4.0 remains unpublished and ready for the manual immutable release after
this final pull request merges with a successful `Required` check.
