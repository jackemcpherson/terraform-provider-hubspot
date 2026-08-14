# Account Membership v0.5.0 Map

This map tracks the account-membership release work from frozen evidence to
publication.

## Tickets

1. [Freeze the API and live lifecycle contract](issues/01-freeze-contract.md).
2. [Implement the typed Settings client](issues/02-typed-client.md).
3. [Implement the provider resource](issues/03-provider-resource.md).
4. [Prove the CLI lifecycle and maintenance](issues/04-lifecycle-maintenance.md).
5. [Publish the demo and user documentation](issues/05-demo-documentation.md).
6. [Qualify and publish v0.5.0](issues/06-qualify-publish.md).

## Decisions So Far

- The resource manages account membership without CRM user-profile settings.
- Settings user ID is canonical identity.
- Email discovery is explicit import only.
- Name PUT fails closed when role or team assignments exist.
- Removal needs a local opt-in, exact identity, and non-Super-Admin state.
- ADR 0003 remains the release architecture.
