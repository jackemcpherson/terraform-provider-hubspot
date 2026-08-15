# CRM User Profile v0.6.0 Map

This map tracks the CRM user profile release from current evidence to immutable
publication.

## Tickets

1. [Freeze the API and live contract](issues/01-freeze-contract.md).
2. [Implement the typed CRM profile client](issues/02-typed-client.md).
3. [Implement the provider resource](issues/03-provider-resource.md).
4. [Prove the command-line lifecycle and maintenance](issues/04-lifecycle-maintenance.md).
5. [Publish the demo and user documentation](issues/05-demo-documentation.md).
6. [Qualify and publish v0.6.0](issues/06-qualify-publish.md).

## Decisions So Far

- The CRM profile and account membership remain separate resources.
- The CRM user ID is canonical profile state identity.
- `hs_internal_user_id` is the Settings-to-CRM join.
- Null optional properties are unmanaged.
- Working-hours JSON has provider-owned canonical order.
- Timezone updates precede working-hours updates.
- Destroy stops management without a remote write.
- ADR 0003 remains the release architecture.
- Current official documentation has no blocking contradiction.
- Missing local live credentials and invalid GitHub CLI authentication block
  publication until resolved.
- The typed client, provider resource, behavioural lifecycle, cumulative
  Northstar helper, demo module, and documentation are implemented with all
  local provider and demo gates passing.
