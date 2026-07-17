# Expand and rehearse the Northstar Free-tier demo

Type: task
Status: claimed
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
