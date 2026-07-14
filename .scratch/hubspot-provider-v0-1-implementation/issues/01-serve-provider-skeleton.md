# Serve a deterministic OpenTofu-first provider skeleton

Type: task
Status: resolved
Blocked by: none

## Outcome

A contributor can build, serve, validate, test, and document a protocol-6
`jackemcpherson/hubspot` provider from a clean checkout using exact pinned tools.
The repository has a secure deterministic pull-request gate before remote HubSpot
behavior is added.

## Scope

- Establish the MPL-2.0 Go module, provider entry point, version injection, and
  conventional Plugin Framework package boundaries.
- Pin Go language `1.25.0`, toolchain `go1.26.5`, Framework `v1.19.0`, protocol 6,
  `tfplugindocs v0.25.0`, GoReleaser `v2.17.0`, and exact supporting tools.
- Serve the provider schema and protocol manifest under both full registry source
  identities without adding a second binary.
- Implement `make tools` and deterministic offline non-mutating `make check` with
  wrong-version refusal.
- Gate formatting, vet, static analysis, module tidiness/verification, unit and
  Framework smoke tests, race tests, bounded fuzz seeds, generated docs, HCL,
  engine smoke tests, and workflow analysis.
- Add fork-safe CI/security workflow foundations with immutable action SHAs,
  `permissions: {}`, job-local permissions, hosted runners, and no secrets.
- Add generated provider documentation, contributing setup, changelog, security
  policy, and ownership foundations.

## Non-goals

- No HubSpot network request or managed CRM configuration.
- No live acceptance, release publication, OAuth, or CRM record handling.
- Do not prescribe a runner operating system name or version.

## Acceptance

- A clean checkout with the declared tools builds a CGO-disabled provider and
  serves protocol 6 with versioned user-agent metadata.
- Wrong/missing tool versions fail before checks mutate or download anything.
- The exact offline aggregate passes locally and in pull-request CI.
- Minimum/current OpenTofu and Terraform smoke configurations load the provider.
- Generated documentation is committed and a regeneration has no diff.
- Workflow analysis rejects mutable actions, broad permissions, unsafe triggers,
  untrusted shell interpolation, inherited secrets, and self-hosted runners.
- Repository scans find no credentials or generated platform/account identifiers.

## Resolution

Implemented in the current branch. The provider now serves protocol 6 with a
canonical Terraform Framework address, has typed validated configuration with
secret-safe runtime data, carries the registry manifest and dual-address inventory,
and includes generated docs, examples, deterministic Go/OpenTofu/Terraform smoke,
CI/security foundations, reproducible-build configuration, and repository security
and contribution policy.

Verification passed:

- `make check`;
- `make docs` followed by clean generated-doc validation;
- Go unit, race, vet, staticcheck, module tidy/verify, and fuzz-seed checks;
- OpenTofu 1.12.3 and Terraform 1.15.8 provider schema validation;
- CGO-disabled trim-path provider build;
- staged diff whitespace validation.

## Authorities

- [Normative specification](../spec.md)
- [Provider foundations](../../hubspot-provider-rewrite/research/03-opentofu-provider-foundations.md)
- [CI/security/release decisions](../../hubspot-provider-rewrite/research/10-ci-security-release-operations.md)
