# HubSpot provider

OpenTofu-first provider for declarative HubSpot CRM configuration.

## Development

Requirements are pinned in `go.mod` and `Makefile`: Go 1.26.5 and OpenTofu
1.12.3 for the current local gate.

```sh
make tools
make check
```

`make check` is deterministic, offline, and non-mutating after tools are
installed. It runs formatting, vet, static analysis, tests, race tests, module
verification, documentation checks, and workflow policy checks.

## Provider addresses

OpenTofu: `registry.opentofu.org/jackemcpherson/hubspot`

Terraform: `registry.terraform.io/jackemcpherson/hubspot`

The same release artifacts serve both identities. State migration between full
addresses uses the documented `state replace-provider` workflow; it is not
assumed to be implicit.

The provider binary uses the canonical Terraform Framework address. OpenTofu
registry selection is represented by the separate published registry identity,
not by a second binary.

## Authentication

Set `HUBSPOT_ACCESS_TOKEN` in the execution environment. Never commit or log the
token. Pull-request workflows do not receive HubSpot credentials.

## Scope

This first scaffold serves protocol 6 and provider configuration. CRM
configuration resources are added through the accepted vertical implementation
tickets under `.scratch/hubspot-provider-v0-1-implementation/`.
