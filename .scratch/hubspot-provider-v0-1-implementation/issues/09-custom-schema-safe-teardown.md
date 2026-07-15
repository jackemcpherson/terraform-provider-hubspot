# Make custom-schema split ownership and teardown safe

Type: task
Status: resolved
Blocked by: 08

## Outcome

Users can compose schema-owned bootstrap properties with separate ordinary
properties and can deliberately dismantle the schema without silent adoption,
partial state loss, or avoidable mutation before blocker detection.

## Scope

- Implement `expected_external_properties`, quiet expected externals, unexpected
  external warnings, overlap rejection, and external destroy blockers.
- Implement default-on provider-local deletion protection with a required prior
  authored disable apply and warning.
- Implement least-privilege read-only preflight, role clearing, owned-property
  teardown, schema deletion, terminal verification, idempotence, and partial
  teardown retry from observed reality.
- Prove composition ordering through a separate `hubspot_property` reference while
  never requiring CRM record scopes.
- Add complete unit/HTTP/Framework/engine/Enterprise live, docs, examples, and
  changelog coverage.

## Non-goals

- No auto-adoption, automatic cleanup of separate properties, CRM record reads,
  false `allow_destructive_changes` safety switch, or state removal after an
  unverified/partial teardown.

## Acceptance

- Expected externals neither enter ownership nor warn; unexpected externals warn;
  any external definition blocks destroy before mutation.
- Protection cannot be disabled and destroyed in one authored apply; the prior
  transition is stored and warns.
- Known preflight blockers produce zero mutation. Invisible least-privilege
  blockers preserve state and surface HubSpot's safe error.
- Partial role/property/schema teardown retains sufficient state to resume
  idempotently from a fresh read.
- Full Enterprise split-ownership, protection, blocker, teardown, absence, and
  cleanup scenarios pass; missing eligibility blocks release.

## Authorities

- [Normative specification](../spec.md)
- [Ownership prototype](../../hubspot-provider-rewrite/prototypes/custom-schema-lifecycle/README.md)
- [Lifecycle semantics](../../hubspot-provider-rewrite/research/06-state-lifecycle-semantics.md)
- [Verification matrix](../../hubspot-provider-rewrite/research/08-verification-matrix-proposal.md)

## Answer

Added provider-local deletion protection, expected external-property tracking,
unexpected-external warnings, read-only destroy preflight, external blockers,
and safe archive sequencing. Added teardown documentation and changelog
coverage. Full Enterprise teardown/live acceptance remains release-gated.
