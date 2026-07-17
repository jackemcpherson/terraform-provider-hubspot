# Write the Free-only v0.1.0 public contract

Type: task
Status: resolved
Blocked by: 01

## Question

What exact normative specification, compatibility boundary, resource/data-source
inventory, scope/limit statement, acceptance criteria, and deferred-capability
notice define the Free-only v0.1.0 public beta without weakening OpenTofu-first or
Terraform/dual-registry compatibility?

## Answer

[Free-only v0.1.0 public contract](../free-only-v0-1-contract.md) is now the
normative scope delta for this effort. It fixes the registered surface to property
groups, ordinary non-sensitive properties, and two non-sensitive discovery data
sources; retains protocol-6 dual-engine/dual-registry compatibility; and makes the
ten-property one-portal lifecycle, demo rehearsal, and protected publish-ready
candidate explicit release gates.

It defers pipelines, custom schemas, sensitive definitions, custom-object
configuration, and their capability shards from every v0.1 public artifact while
preserving the source baseline at the deferred tag. The next tickets own removing
that surface, proving the one-portal contract, rehearsing the demo, and configuring
the protected controls.
