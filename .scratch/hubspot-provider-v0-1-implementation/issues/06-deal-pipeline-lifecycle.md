# Manage a deal pipeline with stable nested stages

Type: task
Status: resolved
Blocked by: 02

## Outcome

Users can manage an entire deal pipeline and its writable stages as one resource,
with stable nested identity and no label-based recovery.

## Scope

- Implement pinned typed pipeline/stage clients and `hubspot_pipeline` for deals.
- Model caller logical stage keys, computed remote IDs/write permissions, labels,
  display order, and probability metadata in 0.1 increments.
- Implement exclusive complete-stage ownership, remote-ID refresh matching,
  permanent imported/out-of-band remote-ID keys, canonical import, protected-stage
  rejection, scalar/stage drift, replacement, archive, verified restore, and safe
  ambiguous-operation handling.
- Add unit, HTTP, Framework, engine, paid live, docs, examples, and changelog cases.

## Non-goals

- Ticket/custom pipeline semantics belong to 07. No standalone stage resource,
  label-derived identity, nested-key rename, protected-stage management, or blind
  ambiguous-create replay.

## Acceptance

- Deal create/read/update/import/drift/archive/restore/reference-failure/cleanup
  cases pass every offline layer and eligible paid live acceptance.
- Refresh preserves known logical keys by remote ID; imported and out-of-band
  stages use remote IDs permanently.
- Missing configured stages are recreated and extra writable stages are planned
  for removal; refresh itself performs no mutation.
- Import rejects protected stages and incomplete configuration without partial
  adoption.
- Ambiguous create does not search by label, replay, or remove prior state.
- Docs clearly explain exclusive ownership, probability validation, and nested-key
  permanence.

## Authorities

- [Normative specification](../spec.md)
- [Resource model](../../hubspot-provider-rewrite/research/04-v0-1-resource-model.md)
- [Lifecycle semantics](../../hubspot-provider-rewrite/research/06-state-lifecycle-semantics.md)
- [Configuration prototype](../../hubspot-provider-rewrite/prototypes/user-facing-configuration/README.md)

## Answer

Implemented the pinned deal pipeline client and nested `hubspot_pipeline`
resource with stage metadata validation, stable remote-ID map keys, import,
archive, readback verification, and protected-stage rejection. Added docs,
example, and changelog coverage. Paid live acceptance remains release-gated.
