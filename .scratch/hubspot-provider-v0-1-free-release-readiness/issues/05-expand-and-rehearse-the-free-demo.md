# Expand and rehearse the Northstar Free-tier demo

Type: task
Status: resolved
Blocked by: 03, 04

## Question

How should `../terraform-hubspot-demo` be updated within HubSpot Free's ten-custom-
property limit to demonstrate the whole v0.1 surface, deterministic teardown and
rebuild, authored drift repair, import/discovery readback, and the exact candidate
through both local filesystem-mirror registry identities?

## Comments

- 2026-07-17 — The deterministic local demo gate passes with the expanded
  ten-property model under both engines and their respective registry identities.
  A live rehearsal with the supplied token confirmed contact, company, and deal
  property creation but failed safely on ticket-property creation and the
  custom-property limit preflight with `MISSING_SCOPES`. The recovery destroy
  completed: all seven created properties and all four groups (including the
  ticket group) are absent from the active portal. Resume only with a protected
  token that grants the ticket-property and custom-property-limit permissions;
  then run the one-portal lifecycle to rebuild and verify the demo.

## Answer

The Northstar demo now exercises the complete Free v0.1 property surface within
the ten-property account budget: three contact properties, two company
properties, two deal properties, and three ticket properties, each under its
own managed group. It uses the unqualified provider source so OpenTofu and
Terraform resolve their respective local filesystem-mirror identities.

The live Free portal reports one overall custom-property limit of ten and does
not supply a usable per-object limit for every standard object. The quota
preflight therefore correctly gates on ten overall free slots rather than
inventing per-object requirements. The ticket datetime property uses HubSpot's
valid `datetime` / `date` type-field-type pairing.

On 2026-07-17 the full one-portal lifecycle passed from provider commit
`960658b`: teardown, quota preflight, all Free acceptance lifecycle and
Terraform-parity tests, and deterministic demo rebuild. Those tests include
authored drift repair and property import/discovery readback. The rebuilt demo
at demo commit `0f4a959` then had a zero-change live verification under both
OpenTofu and Terraform using their separate local mirror identities. The portal
is left in the intended Northstar desired state.
