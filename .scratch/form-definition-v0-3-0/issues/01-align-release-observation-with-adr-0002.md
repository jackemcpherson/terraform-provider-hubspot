# 01 — Align release observation with ADR 0002

**What to build:** Make post-publication observation report the release evidence the accepted minimal-release architecture actually produces, so a healthy v0.3.0 release cannot fail because provenance attestations were deliberately removed.

**Blocked by:** None — can start immediately.

**Status:** resolved

- [x] Release observation no longer invokes or requires provenance attestation verification.
- [x] Observation verifies the signed tag, registered signing identity, checksum signature, versioned Registry manifest, exact archive/checksum closure, and asset digests required by ADR 0002.
- [x] Observer contract tests fail for a bad signature, manifest, closure, digest, tag, or signing identity and pass without any attestation artifact.
- [x] Workflow policy continues to forbid attestation work in the minimal publication transaction.
- [x] Existing immutable v0.2.0 tags and release assets are read only and remain untouched.
- [x] Repository release-operation guidance describes the corrected observation contract consistently.

## Answer

Release observation now uses the evidence produced by ADR 0002's minimal
publication transaction. Existing releases are checked through an isolated GPG
keyring: the annotated tag and checksum inventory must both validate against the
full registered fingerprint. The observer validates every GitHub asset's
immutable SHA-256 digest and exact downloaded file set before applying the
existing versioned-manifest, standard-platform, checksum-closure, archive-name,
and archive-smoke checks. It does not request provenance attestations.

The observer contract suite covers unsigned or invalid tags, an unregistered
signer, invalid checksum signatures, malformed manifests, incomplete closure,
digest mismatches, tag/release conflicts, failed required checks, and commits
outside `main`. Workflow policy rejects attestation or provenance work in both
the minimal publication job and the observer. Operator guidance now documents
the corrected read-only observation command separately from the released live
completion journey.

Verification passed with ShellCheck, `make check-workflows`, and the complete
`make check` aggregate gate. No Git tag, GitHub release, Registry entry, or
v0.2.0 asset was mutated.
