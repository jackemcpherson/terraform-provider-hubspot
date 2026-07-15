# Expand pipelines to ticket and custom object types

Type: task
Status: resolved
Blocked by: 06

## Outcome

The pipeline resource safely supports ticket and eligible custom-object pipelines
without fixing an object-type allowlist or discarding unknown metadata.

## Scope

- Add ticket `ticketState` validation and generic custom-pipeline metadata
  preservation through typed clients and the existing resource boundary.
- Keep exact object type identifiers and shared identity/import/stage ownership.
- Complete ticket/custom create, update, reference-failure, import, drift,
  archive/restore, ambiguous-operation, quota, and cleanup behavior.
- Add all unit, HTTP, Framework, engine, isolated paid live, docs, examples, and
  changelog cases.

## Non-goals

- No fixed object-type allowlist, metadata key rewriting, association labels,
  standalone stages, protected-stage adoption, or CRM records.

## Acceptance

- Ticket metadata accepts only `OPEN`/`CLOSED`; invalid casing/value fails at plan.
- Unknown custom-pipeline metadata keys survive read, state, and update without
  provider loss or invented semantics.
- `ticket_pipelines` and `custom_pipelines` live shards pass every named lifecycle,
  failure, ambiguity, and cleanup case with eligible credentials.
- Existing deal behavior remains unchanged and regression-green.
- Missing eligible account access blocks release rather than marking a live shard
  skipped.

## Authorities

- [Normative specification](../spec.md)
- [Resource model](../../hubspot-provider-rewrite/research/04-v0-1-resource-model.md)
- [Verification matrix](../../hubspot-provider-rewrite/research/08-verification-matrix-proposal.md)
- [Live probe checklist](../../hubspot-provider-rewrite/research/12-hubspot-live-lifecycle-probes.md)

## Answer

Extended `hubspot_pipeline` to exact ticket and custom object identifiers,
preserving unknown metadata and validating ticket `ticketState` values. Stage
remote-ID keys remain stable across refresh and import. Docs, example, and
changelog coverage are included; paid live acceptance remains release-gated.
