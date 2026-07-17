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
  Live rehearsal is blocked: the available HubSpot CLI credential can read the
  built-in contact property but returns `MISSING_SCOPES` for all property-group
  creates, including tickets. The Free acceptance runner separately requires the
  protected `HUBSPOT_ACCESS_TOKEN`, which is not present in this session. Its
  teardown/rebuild wrapper left no demo configuration active after the failed
  scope check. Resume only with the protected token that has all four required
  Free schema-scope families; then run the one-portal lifecycle to rebuild and
  verify the demo.
