# HubSpot provider

This provider manages HubSpot configuration with OpenTofu or Terraform. The
v0.3.0 release is a public beta and does not promise compatibility with v1.

Managed resources:

- property groups and property definitions
- narrowly typed contact email Form definitions
- explicit File folders and locally sourced Managed files

The two property-definition data sources inspect non-sensitive schema metadata.
The provider does not read CRM records, record values, form submissions, or
responses. v0.3.0 supports ordinary non-sensitive CRM property schema plus one
contact email Form definition aggregate on HubSpot Free. Current source also
contains the additive v0.4.0 Files configuration candidate; pipelines, custom
schemas, and sensitive definitions are deferred.

Observed custom-property limit telemetry is advisory, not local admission
control. Review [permissions, account tiers, and exclusions](docs/permissions-and-limits.md)
before applying managed definitions; remote create responses remain authoritative.

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
- [Form definition surface](docs/surfaces/form-definition.md)
- [Files configuration surface](docs/surfaces/files-configuration.md)
- [Destroy semantics](docs/destroy-semantics.md)
- [State portability](docs/state-portability.md)
- [Troubleshooting](docs/troubleshooting.md)
- [Maintainer release operations](docs/release-operations.md)

Generated field references are under [docs/resources](docs/resources) and
[docs/data-sources](docs/data-sources). Reviewed configurations are under
[examples](examples).

Generate the cumulative provider/module HTML portal from executable provider
registration and the sibling demo's HCL:

```sh
make docs-portal
make docs-portal-update # after reviewed provider schema, module HCL, or portal changes
make docs-portal-serve # localhost only
```

Set `HUBSPOT_DEMO_REPO` when the demo is not at
`../terraform-hubspot-demo`. Candidate gates also set exact expected commits and
require clean inputs through the `DOCS_PORTAL_*` environment variables.
`make docs-portal` rejects a stale committed source digest and smoke-renders the
localhost build; `make docs-portal-update` intentionally refreshes that digest.

## Exclusions

The v0.4.0 candidate does not manage CRM records, record values, form
submissions, pipelines, custom schemas, association labels, sensitive
definitions, OAuth, consent, notification or automation behavior, non-email
form fields, HubSpot-defined properties, CMS Developer File System content, URL
imports, signed URLs, or arbitrary HTTP/JSON payloads. It does not migrate state
from third-party HubSpot providers.

## Development

The local gate uses the exact Go, OpenTofu, and Terraform versions in `Makefile`.

```sh
make tools
make check
```

`make check` formats and tests the Go code, checks generated references, and
validates each reviewed example with both CLIs. Live HubSpot acceptance runs in
protected workflows against disposable accounts.
