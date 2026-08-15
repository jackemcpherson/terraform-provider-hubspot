# HubSpot provider

This provider manages HubSpot configuration with OpenTofu or Terraform. The
v0.7.0 release is a public beta and does not promise compatibility with v1.

Managed resources:

- property groups and property definitions
- narrowly typed contact email Form definitions
- explicit File folders and locally sourced Managed files
- guarded account membership by canonical Settings user ID
- selected CRM user profile properties by account-specific CRM user ID
- standard Product definitions by HubSpot-generated numeric ID

The two property-definition data sources inspect non-sensitive schema metadata.
The provider does not read CRM records, record values, form submissions, or
responses. v0.7.0 supports ordinary non-sensitive CRM property schema, one
contact email Form definition aggregate, Files configuration, and account
membership plus separate CRM user profile and Product definition configuration
on HubSpot Free.
Pipelines, custom schemas, and sensitive definitions are deferred.

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
- [Account membership surface](docs/surfaces/account-membership.md)
- [CRM user profile surface](docs/surfaces/crm-user-profile.md)
- [Product definition surface](docs/surfaces/product-definition.md)
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
# Run after reviewed provider schema, module HCL, or portal changes.
make docs-portal-update
make docs-portal-serve # localhost only
```

Set `HUBSPOT_DEMO_REPO` when the demo is not at
`../terraform-hubspot-demo`.

`make docs-portal` rejects a stale committed source digest and smoke-renders the
localhost build; `make docs-portal-update` intentionally refreshes that digest.

## Exclusions

The v0.7.0 release does not manage CRM records, record values, form
submissions, pipelines, custom schemas, association labels, sensitive
definitions, OAuth, consent, notification or automation behavior, non-email
form fields, HubSpot-defined properties, CMS Developer File System content, URL
imports, signed URLs, roles, teams, seats, permissions, user activation,
deactivation, global-user deletion, or arbitrary HTTP/JSON payloads. CRM user
profile management is limited to job title, availability, timezone, and working
hours. Product management excludes tiered pricing, folders, status, tax, terms,
URLs, custom properties, line items, associations, and search adoption. It does
not migrate state from third-party HubSpot providers.

## Development

The local gate uses the exact Go, OpenTofu, and Terraform versions in `Makefile`.

```sh
make tools
make check
```

`make check` runs the same fast required gate used by pull requests. It formats
and analyses Go code, runs unit tests, checks generated references and workflow
syntax, and validates reviewed examples with both current CLIs. Slower security
checks and live HubSpot acceptance run in weekly maintenance.
