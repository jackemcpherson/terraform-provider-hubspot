# State portability

The CRM schema, Form, and Files resources use schema version 1. Their version-0
boundary is an offline, byte-preserving migration: IDs, nested map keys,
ownership sets, and teardown safeguards are copied without contacting HubSpot
or normalizing state. The account-membership resource begins at schema version
0 and needs no legacy migration. The CRM-user-profile and Product resources also
begin at schema version 0. Each keeps its own documented numeric identity.

The provider supports forward upgrades only. Downgrades are not promised.
Legacy flatmap state is rejected before writing a replacement state file; first
refresh it with the provider version that wrote it using Terraform or OpenTofu.

To move between the Terraform and OpenTofu registry source identities, use the
engine command in both directions and keep the generated backup. Run `plan`
afterward; a successful source replacement is expected to produce an empty plan
and makes no HubSpot API calls.

```sh
tofu state replace-provider \
  registry.terraform.io/jackemcpherson/hubspot \
  registry.opentofu.org/jackemcpherson/hubspot

terraform state replace-provider \
  registry.opentofu.org/jackemcpherson/hubspot \
  registry.terraform.io/jackemcpherson/hubspot
```

The CLIs write a state backup before replacement. Do not delete it until the
subsequent plan is empty. These commands change provider source addresses in
state; they do not migrate third-party provider schemas.
