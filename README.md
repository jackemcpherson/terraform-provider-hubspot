# HubSpot provider

This provider manages HubSpot CRM property configuration with OpenTofu or
Terraform. `v0.1.0-alpha.1` is an opt-in Free-tier alpha and does not promise
compatibility with later prereleases or v1.

Managed resources:

- property groups
- ordinary non-sensitive scalar and enumeration property definitions

The two property-definition data sources inspect schema metadata. The provider
does not read CRM records or record values.

## Configure

Declare the OpenTofu registry source and keep the static app token outside HCL:

```hcl
terraform {
  required_providers {
    hubspot = {
      source  = "registry.opentofu.org/jackemcpherson/hubspot"
      version = "= 0.1.0-alpha.1"
    }
  }
}

provider "hubspot" {}
```

Pin the alpha exactly because registries do not select prereleases by default.

```sh
export HUBSPOT_ACCESS_TOKEN='...'
tofu init
tofu plan
```

Terraform users can change the source to
`registry.terraform.io/jackemcpherson/hubspot` and replace `tofu` with
`terraform`. Both registry identities publish the same provider artifacts.

## Read before applying

- [Authentication and aliases](docs/authentication-and-aliases.md)
- [Permissions, account tiers, and exclusions](docs/permissions-and-limits.md)
- [Imports and drift](docs/imports-and-drift.md)
- [Property lifecycle](docs/property-lifecycle.md)
- [State portability](docs/state-portability.md)
- [Troubleshooting](docs/troubleshooting.md)
- [Maintainer release operations](docs/release-operations.md)

Generated field references are under [docs/resources](docs/resources) and
[docs/data-sources](docs/data-sources). Reviewed configurations are under
[examples](examples). Pipeline and custom-schema examples are retained as marked
development-only fixtures; they do not describe the alpha's public schema.

## Exclusions

The Free alpha does not register pipelines or custom object schemas and rejects
advanced, sensitive, calculated, currency, external-option, unique-value, and
referenced-object property fields. It also does not manage CRM records, record
values, association labels, OAuth, HubSpot-defined properties, or arbitrary
HTTP/JSON payloads. It does not migrate state from third-party HubSpot providers.

## Development

The local gate uses the exact Go, OpenTofu, and Terraform versions in `Makefile`.

```sh
make tools
make check
```

`make check` formats and tests the Go code, checks generated references, and
validates each reviewed example with both CLIs. Live HubSpot acceptance runs in
protected workflows against disposable accounts.
