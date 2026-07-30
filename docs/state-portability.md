# State Portability

This guide explains schema upgrades and registry source changes. Back up the
state before you change its provider source.

---

## Schema Upgrades

Managed resources use schema version 1. The version 0 migration runs offline.
It preserves identifiers, map keys, ownership sets, and teardown safeguards.
The migration does not contact HubSpot or normalise state values.

The provider supports only forward upgrades. Refresh legacy flatmap state with
the provider version that wrote it before you install a newer provider.

## Registry Source Changes

Use the applicable command to move state between registry source identities:

```shell
tofu state replace-provider \
  registry.terraform.io/jackemcpherson/hubspot \
  registry.opentofu.org/jackemcpherson/hubspot

terraform state replace-provider \
  registry.opentofu.org/jackemcpherson/hubspot \
  registry.terraform.io/jackemcpherson/hubspot
```

Each command writes a state backup. Run `plan` after the replacement and keep
the backup until the plan is empty.

These commands change only the provider source address. They do not migrate
schemas from a third-party provider.
