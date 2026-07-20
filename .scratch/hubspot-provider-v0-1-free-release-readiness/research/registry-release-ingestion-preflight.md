# Terraform and OpenTofu provider Registry release pre-flight

Research date: 2026-07-20 (Australia/Melbourne)

## Answer

The release failures are deterministic packaging-contract failures, not Registry
flakiness:

1. `v0.1.0` put per-archive SPDX SBOM filenames into the Registry checksum
   contract. Terraform Registry's provider upload accepts the provider archives
   and the named manifest from that checksum set, so it rejected the SBOM entries
   as missing request-body files.
2. `v0.1.1` excluded the SBOMs from the checksum, but uploaded
   `terraform-registry-manifest.json` instead of the required
   `terraform-provider-hubspot_0.1.1_manifest.json`.
3. `v0.1.2` fixed the release filename, but the manifest still says
   `"format_version": 1`. The supported field is numeric `"version": 1`.
   With `version` absent, Terraform Registry reports the zero value as
   `unknown manifest version 0`.

The correct recovery is a new immutable patch release. Do not edit the existing
tags or assets. Before publishing it, run a local GoReleaser build and a strict
Registry-contract checker over the resulting directory; run that same checker in
ordinary CI and again over the downloaded GitHub draft. HashiCorp does not expose
a supported Terraform public-Registry validator or ingestion dry-run. GoReleaser
supplies the build/configuration primitives, but the repository must supply the
small contract checker.

One Terraform-compatible release bundle also satisfies OpenTofu. OpenTofu has a
separate provider-registration and GPG-key-registration workflow, but it consumes
the same conventionally named ZIP, checksum, signature, and manifest assets.

## Evidence from the three releases

| Release | What changed | Why Terraform Registry rejects it |
| --- | --- | --- |
| `v0.1.0` | The GoReleaser checksum had no `ids` filter, and `SHA256SUMS` lists every `*.zip.spdx.sbom` alongside the archives. | HashiCorp's contract defines the checksum set as provider ZIPs plus the renamed manifest, not SBOMs. The Registry error lists those exact SBOM names as missing request-body files. [tagged GoReleaser configuration](https://github.com/jackemcpherson/terraform-provider-hubspot/blob/v0.1.0/.goreleaser.yml), [published checksum](https://github.com/jackemcpherson/terraform-provider-hubspot/releases/download/v0.1.0/terraform-provider-hubspot_0.1.0_SHA256SUMS), [release](https://github.com/jackemcpherson/terraform-provider-hubspot/releases/tag/v0.1.0) |
| `v0.1.1` | `checksum.ids: [provider]` removed SBOMs from `SHA256SUMS`, but `release.extra_files` still had no `name_template`. | The release contains `terraform-registry-manifest.json`; Terraform requires `terraform-provider-hubspot_0.1.1_manifest.json`. [tagged configuration](https://github.com/jackemcpherson/terraform-provider-hubspot/blob/v0.1.1/.goreleaser.yml), [release](https://github.com/jackemcpherson/terraform-provider-hubspot/releases/tag/v0.1.1) |
| `v0.1.2` | Both checksum and release `extra_files` rename the manifest correctly. | The uploaded JSON uses `format_version`; Terraform's only supported manifest-format field is numeric `version`, set to `1`. [tagged configuration](https://github.com/jackemcpherson/terraform-provider-hubspot/blob/v0.1.2/.goreleaser.yml), [uploaded manifest](https://github.com/jackemcpherson/terraform-provider-hubspot/releases/download/v0.1.2/terraform-provider-hubspot_0.1.2_manifest.json), [release](https://github.com/jackemcpherson/terraform-provider-hubspot/releases/tag/v0.1.2) |

These observations reproduce the UI's three errors exactly. They also show why
the current checks passed: they establish file integrity and local installability,
but not the Registry's exact asset namespace or manifest schema.

## Canonical Terraform public-Registry contract

HashiCorp's current publishing specification requires a public, lower-case GitHub
repository named `terraform-provider-{NAME}`, a finalized GitHub release, a
v-prefixed SemVer tag, and no branch with the same name as the tag. Existing
versions must not be replaced; corrections are new versions. The provider release
contract is as follows. [HashiCorp provider publishing specification](https://developer.hashicorp.com/terraform/registry/providers/publishing)

For an example `v0.1.3` release of this provider:

| Kind | Required release name or content |
| --- | --- |
| Platform archive | One or more `terraform-provider-hubspot_0.1.3_{OS}_{ARCH}.zip` files |
| Binary inside each archive | `terraform-provider-hubspot_v0.1.3` (with the platform's executable convention where applicable) |
| Registry manifest | `terraform-provider-hubspot_0.1.3_manifest.json` |
| Checksums | `terraform-provider-hubspot_0.1.3_SHA256SUMS` |
| Signature | `terraform-provider-hubspot_0.1.3_SHA256SUMS.sig` |

The manifest source can remain named `terraform-registry-manifest.json` in the
repository, but the release asset must be renamed. For a Plugin Framework provider
serving protocol 6, its contents should be exactly:

```json
{
  "version": 1,
  "metadata": {
    "protocol_versions": ["6.0"]
  }
}
```

There is no supported `manifest_version` or `format_version` field. `version` is
the numeric manifest-format version, not the provider version, and the documented
value is `1`. Plugin SDK v2 normally advertises `5.0`; Plugin Framework normally
advertises `6.0` unless explicitly forced to protocol 5. [Manifest schema](https://developer.hashicorp.com/terraform/registry/providers/publishing#terraform-registry-manifest-file), [official protocol-6 manifest](https://github.com/hashicorp/terraform-provider-scaffolding-framework/blob/2594dc2513439e3f81a9ebeec09eb9c36de94e6c/terraform-registry-manifest.json)

`SHA256SUMS` must cover every provider ZIP and the release-named manifest. The
binary detached GPG signature covers that checksum file. The Registry validates
it with the public key registered for the namespace; HashiCorp currently accepts
RSA and DSA keys, not a default ECC key. The `.sig` must be a binary detached
signature, not ASCII-armored. [Checksum and signing requirements](https://developer.hashicorp.com/terraform/registry/providers/publishing#manually-preparing-a-release), [signing-key requirements](https://developer.hashicorp.com/terraform/registry/providers/publishing#preparing-and-adding-a-signing-key)

HashiCorp's current provider scaffold is the executable reference configuration:
it applies the same manifest `name_template` in both `checksum.extra_files` and
`release.extra_files`, names the checksum predictably, and GPG-signs the checksum.
It does not add SBOMs to the provider-release configuration. [Official GoReleaser configuration](https://github.com/hashicorp/terraform-provider-scaffolding-framework/blob/2594dc2513439e3f81a9ebeec09eb9c36de94e6c/.goreleaser.yml)

### Platform matrix detail

HashiCorp recommends Darwin AMD64/ARM64, Linux AMD64/ARM64/ARMv6, and Windows
AMD64; Linux AMD64 is required for HCP Terraform. It additionally recommends
Linux 386, Windows 386, and FreeBSD 386/AMD64. The Linux AMD64 build must have
CGO disabled and no external executable dependency. [Recommended platforms](https://developer.hashicorp.com/terraform/registry/providers/os-arch)

The Registry filename grammar has only `{OS}_{ARCH}`. The official scaffold emits
the ARMv6 target as `linux_arm.zip`; it does not encode GOARM as `armv6` or build
both GOARM 6 and 7 under separate archive names. This matters to OpenTofu too:
its current ingestion source probes only `386`, `amd64`, `arm`, and `arm64`
suffixes, so this repository's `*_armv6.zip` and `*_armv7.zip` extras are ignored
there. The shared matrix should therefore use the standard `arm` filename for the
recommended ARMv6 build rather than two nonstandard names. [HashiCorp scaffold](https://github.com/hashicorp/terraform-provider-scaffolding-framework/blob/2594dc2513439e3f81a9ebeec09eb9c36de94e6c/.goreleaser.yml), [OpenTofu target discovery source](https://github.com/opentofu/registry/blob/e0286f1c9679b8166d18da62d4fbb9bfb0f1964c/src/internal/provider/version.go#L12-L100)

## SBOMs, Cosign, and GPG

GoReleaser can ask Syft to emit an SBOM per archive. Those documents become
GoReleaser artifacts, and `checksum.ids` controls which artifact IDs enter the
checksum file. [GoReleaser SBOM pipeline](https://www.goreleaser.com/customization/sbom/), [GoReleaser checksum filtering](https://goreleaser.com/customization/package/checksum/)

The Terraform provider-publishing contract does not define SBOMs as Registry
inputs. For this repository, the safest boundary is:

- keep the public Registry asset bundle to ZIPs, the manifest, `SHA256SUMS`, and
  its `.sig`;
- generate the SPDX document separately for GitHub's SBOM attestation or retain it
  as workflow metadata;
- if standalone SBOM release attachments are retained, never include their names
  in the Registry checksum file;
- assert that the checksum filename set is exactly `ZIPs + named manifest`.

This preserves supply-chain metadata without making it part of the Terraform
Registry multipart-ingestion contract. Syft can scan the archives and emit SPDX
JSON, but it is an SBOM generator/parser, not a Terraform Registry release
validator. [Syft project and formats](https://github.com/anchore/syft)

GoReleaser can also sign blobs with Cosign and verify a Sigstore bundle locally.
That can be additive provenance, but it cannot replace the Registry signature:
Terraform explicitly requires the binary detached GPG signature and a registered
GPG public key. [GoReleaser Cosign support](https://goreleaser.com/customization/sign/sign/#signing-with-cosign), [Terraform GPG requirement](https://developer.hashicorp.com/terraform/registry/providers/publishing#preparing-and-adding-a-signing-key)

## OpenTofu Registry: overlap and differences

OpenTofu registration is separate from Terraform registration. A provider is
submitted through the OpenTofu Registry's GitHub issue-form UI using the public
repository path `NAMESPACE/terraform-provider-NAME`; direct PRs, API-created
issues, and CLI-created issues are explicitly not accepted because the automation
depends on structured issue-form data. A signing key is submitted through a
second issue form, either for the namespace or one provider. Organization key
submitters must have public organization membership. [OpenTofu Registry README](https://github.com/opentofu/registry/blob/e0286f1c9679b8166d18da62d4fbb9bfb0f1964c/README.md), [provider issue form](https://github.com/opentofu/registry/blob/e0286f1c9679b8166d18da62d4fbb9bfb0f1964c/.github/ISSUE_TEMPLATE/provider.yml), [key issue form](https://github.com/opentofu/registry/blob/e0286f1c9679b8166d18da62d4fbb9bfb0f1964c/.github/ISSUE_TEMPLATE/provider_key.yml)

Once accepted, OpenTofu discovers later SemVer tags automatically. Its generator
looks for the same `{project}_{version}_SHA256SUMS`, `.sig`,
`{project}_{version}_{os}_{arch}.zip`, and
`{project}_{version}_manifest.json` URL pattern. Extra checksum entries such as
SBOMs are ignored because only known platform filenames are selected. A missing or
invalid manifest falls back to protocol `5.0`. [OpenTofu ingestion source](https://github.com/opentofu/registry/blob/e0286f1c9679b8166d18da62d4fbb9bfb0f1964c/src/internal/provider/version.go), [checksum parser](https://github.com/opentofu/registry/blob/e0286f1c9679b8166d18da62d4fbb9bfb0f1964c/src/internal/provider/shasum.go), [manifest parser](https://github.com/opentofu/registry/blob/e0286f1c9679b8166d18da62d4fbb9bfb0f1964c/src/internal/provider/protocols.go)

OpenTofu's manifest parser currently reads only `metadata.protocol_versions`; it
does not validate the top-level Terraform manifest-format `version`. Therefore
`v0.1.2` is acceptable to OpenTofu as protocol 6 but unacceptable to Terraform.
The dual-registry contract must use Terraform's stricter `"version": 1` schema so
one immutable bundle works for both.

This was verified on 2026-07-20 by running the OpenTofu Registry repository's own
`cmd/add-provider` at commit
`e0286f1c9679b8166d18da62d4fbb9bfb0f1964c` against the published releases. It
detected `v0.1.2` as protocol 6, defaulted the wrongly named manifests in `v0.1.0`
and `v0.1.1` to protocol 5, ignored `armv6`/`armv7`, and successfully generated
provider metadata for the other targets. The command is useful evidence after
assets are public, but it is internal Registry tooling that fetches GitHub URLs;
it is not a pre-publication local-artifact validator or a stable distributed CLI.
[OpenTofu add-provider command](https://github.com/opentofu/registry/blob/e0286f1c9679b8166d18da62d4fbb9bfb0f1964c/src/cmd/add-provider/main.go)

OpenTofu's public guidance says to add the provider first and then its ASCII-armored
public GPG key. The same release GPG key can and should be registered independently
with both registries. OpenTofu CLI verifies Registry provider signatures; while a
temporary default-Registry compatibility mode can skip validation when no key is
available, a first-class release should not rely on that fallback. [Adding a provider and key](https://search.opentofu.org/docs/providers/adding), [OpenTofu plugin signing](https://opentofu.org/docs/cli/plugins/signing/)

The existing OpenTofu submission issue remains the correct registration vehicle.
After a correct patch release exists, editing the issue retriggers its automation.
Do not retrigger it as a substitute for the pre-flight. [OpenTofu submission #4699](https://github.com/opentofu/registry/issues/4699)

## What can be tested locally

| Tool/check | What it proves | What it does not prove |
| --- | --- | --- |
| `goreleaser check` | GoReleaser configuration parses and uses supported fields. | Registry filename/schema semantics. |
| `goreleaser healthcheck` | Commands required by the configured pipeline are installed. | Artifact correctness. |
| `goreleaser build` | Configured build targets compile. | Archives, manifest, checksums, signatures, or full release pipeline. |
| `goreleaser release --snapshot --clean` | Full local artifact pipeline writes to `dist` without uploading. Suitable for ordinary CI. | Public-Registry ingestion; snapshot naming is not a real version. |
| Real-version local tag plus `goreleaser release --clean --skip=announce,publish,sign` | Exact production filenames and unsigned bytes without publishing or exposing the production key. | Signature validity and Registry ingestion. |
| `dist/artifacts.json` plus a repository checker | Exact artifact types, names, paths, and platform inventory. | Runtime behavior unless archives are also smoke-installed. |
| `shasum -a 256 -c ...` / `sha256sum --check ...` | Every checksum entry exists and matches. | Whether the checksum contains only Registry inputs. |
| `gpg --verify SUMS.sig SUMS` | Detached signature is valid for the imported key. | Whether that public key was registered in each Registry. |
| Terraform/OpenTofu filesystem-mirror init and schema smoke | An archive for the runner's platform launches under both CLIs and source identities. | Public Registry ingestion or the other platform archives. |
| OpenTofu Registry `cmd/add-provider` | OpenTofu's current public-source generator can ingest already-public release URLs. | Pre-publication validation; Terraform Registry behavior; a stable supported CLI contract. |

GoReleaser documents `check`, `healthcheck`, build-only mode, local snapshots, and
`--skip=publish` as the available dry-run techniques. It also documents
`artifacts.json` as the machine-readable catalog of generated artifacts. A
snapshot is local artifact generation, not Registry validation. [GoReleaser quick start and dry runs](https://www.goreleaser.com/getting-started/quick-start/), [snapshots](https://www.goreleaser.com/customization/publish/snapshots/), [artifact catalog](https://goreleaser.com/customization/general/artifacts/)

No current HashiCorp documentation or public HashiCorp tool exposes a Terraform
public-Registry release-ingestion validator or API dry-run. The documented
`Resync` action is a real ingestion retry and webhook repair, not a dry-run. The
Registry Doc Preview Tool validates documentation rendering only. [Publishing and Resync](https://developer.hashicorp.com/terraform/registry/providers/publishing#webhooks), [documentation preview scope](https://developer.hashicorp.com/terraform/registry/providers/docs)

HashiCorp offers an official reusable community-provider release workflow, but it
still delegates artifact construction to GoReleaser and requires the publishing
spec to be followed; it is release automation, not a Registry validator. Its
configuration is useful as a maintained reference, though this repository's
protected, reproducible, dual-registry workflow has additional controls worth
keeping. [HashiCorp provider release workflow](https://github.com/hashicorp/ghaction-terraform-provider-release)

## Required pre-flight contract

Implement one read-only checker that accepts an asset directory, provider name,
and unprefixed version. Run it after every snapshot build in CI, after every exact
release-candidate build, and after downloading the GitHub draft. It should fail
unless all of the following hold:

1. The asset set contains the expected platform ZIPs, exactly one correctly named
   manifest, exactly one checksum, and (where signing has run) exactly one `.sig`.
2. Every ZIP name matches
   `terraform-provider-hubspot_{version}_{os}_{arch}.zip`; no release platform is
   represented only by a nonstandard `armv6`/`armv7` name.
3. Every ZIP contains exactly the expected provider binary and the binary is
   executable on Unix platforms. No source manifest needs to be duplicated inside
   each archive; it is a release metadata asset.
4. JSON parsing succeeds and an exact structural assertion confirms numeric
   `.version == 1` and `.metadata.protocol_versions == ["6.0"]`. Grep is not a
   schema check.
5. The checksum entries' filename set equals `all ZIP basenames + the named
   manifest basename`. There are no SBOM, provenance, checksum, signature, or
   unrenamed-manifest entries.
6. `shasum -a 256 -c` (or `sha256sum --check` in Linux CI) succeeds from the asset
   directory.
7. After signing, `gpg --verify "$checksum.sig" "$checksum"` succeeds with the
   exported public key and the signature is binary detached data.
8. The current-platform ZIP smoke-installs through filesystem mirrors with both
   Terraform and OpenTofu, under their full Registry identities.
9. Optional SBOM generation/attestation runs on a separate metadata path and
   cannot change the Registry checksum domain.

The exact manifest assertion can be expressed with `jq -e`; the checksum-domain
assertion should compare sorted filename inventories rather than merely executing
`shasum -c`. GoReleaser's `artifacts.json` should be retained and queried as a
second inventory source so missing or unexpected artifact types are visible.

## Gaps in the current repository checks

- `Makefile` and `scripts/verify-release-assets.sh` grep only for
  `protocol_versions`; they never assert `.version == 1`, so `format_version`
  passed.
- `scripts/verify-release-assets.sh` checks that an unversioned
  `terraform-registry-manifest.json` exists, which encodes the `v0.1.1` failure as
  success. It should require the versioned release name.
- `shasum -c` proves integrity but accepts any extra checksum entry, including all
  `v0.1.0` SBOMs. There is no checksum-domain equality check.
- The workflow manually copies the source manifest into `dist`, which can create a
  second, unrenamed asset outside GoReleaser's artifact catalog. The reference
  configuration should have one owner for the renamed release manifest.
- `scripts/smoke-release-archive.sh` proves only the runner platform through local
  filesystem mirrors. It cannot model public Registry multipart parsing, signing
  key registration, or the full platform filename inventory.
- `make release-snapshot` is useful, but it does not currently chain a strict
  Registry-contract verifier, `goreleaser check`, or `goreleaser healthcheck`.

## Recommended recovery sequence

1. Correct the source manifest to use numeric `"version": 1` and retain protocol
   `6.0`.
2. Align GoReleaser with the official HashiCorp manifest rename in both checksum
   and release `extra_files`; make checksum IDs explicit and separate SBOM
   metadata from Registry assets.
3. Normalize the platform matrix to standard `{OS}_{ARCH}` filenames, including
   `linux_arm.zip` for the recommended ARMv6 target if retained.
4. Add the strict checker and make the local target, ordinary CI, release build,
   rebuild, and downloaded-draft verification call the same implementation.
5. Run `goreleaser check`, `goreleaser healthcheck`, the snapshot build/checker,
   and the real-version unsigned build/checker locally or in an isolated CI
   worktree.
6. In the protected release path, sign the checksum, verify it with the public
   key, create the GitHub draft, re-download it, and run the same checker before
   publication.
7. Publish a new patch version; do not modify `v0.1.0` through `v0.1.2`.
8. Resync Terraform Registry, retrigger OpenTofu issue #4699, submit the same GPG
   public key to OpenTofu, and poll both version APIs before running the existing
   real-Registry install/digest/state-migration verification.

This sequence follows the repository's standing release principles: one
Git-authored immutable artifact, identical local and CI gates, protected signing,
and independent verification before external publication.
