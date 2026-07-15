# HubSpot provider

This provider manages HubSpot CRM configuration with OpenTofu or Terraform. The
v0.1 release is a public beta and does not promise compatibility with v1.

Managed resources:

- property groups and property definitions
- pipelines with an exclusively owned stage map
- custom object schemas with continuously owned bootstrap properties

The two property-definition data sources inspect schema metadata. The provider
does not read CRM records or record values.

## Configure

Declare the OpenTofu registry source and keep the static app token outside HCL:

```hcl
terraform {
  required_providers {
    hubspot = {
      source = "registry.opentofu.org/jackemcpherson/hubspot"
    }
  }
}

provider "hubspot" {}
```

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
- [Pipeline lifecycle](docs/pipeline-lifecycle.md)
- [Custom schema ownership and teardown](docs/custom-schema-lifecycle.md)
- [State portability](docs/state-portability.md)
- [Troubleshooting](docs/troubleshooting.md)
- [Maintainer release operations](docs/release-operations.md)

Generated field references are under [docs/resources](docs/resources) and
[docs/data-sources](docs/data-sources). Reviewed configurations are under
[examples](examples).

## Exclusions

v0.1 does not manage CRM records, record values, association labels, OAuth,
HubSpot-defined properties, protected stages, standalone stages, or arbitrary
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
