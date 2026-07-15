# State portability

Managed resources use schema version 1. The version-0 boundary is an offline,
byte-preserving migration: IDs, nested map keys, ownership sets, and teardown
safeguards are copied without contacting HubSpot or normalizing the state.

The provider supports forward upgrades only. Downgrades are not promised.
Legacy flatmap state is rejected before writing a replacement state file; first
refresh it with the provider version that wrote it using Terraform or OpenTofu.

To move between the Terraform and OpenTofu registry source identities, use the
engine command in both directions and keep the generated backup. Run `plan`
afterward; a successful source replacement is expected to produce an empty plan
and makes no HubSpot API calls.
