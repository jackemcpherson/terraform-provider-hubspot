# 03 — Reconcile supported presentation and reject unsafe drift

**What to build:** Make supported Form definition presentation authoritative and convergent while refusing to ignore or erase remote structure outside the provider's evidence boundary.

**Blocked by:** 02 — Manage one typed Form definition end to end.

**Status:** resolved

- [x] Required text, email validation, language, alignment, colors, fonts, widths, and sizes are rejected at plan time when they violate the narrow typed contract.
- [x] A semantic no-op produces an empty plan and sends no PATCH request, avoiding HubSpot's timestamp-changing no-op behavior.
- [x] An in-place change sends bounded PATCH content only for changed supported top-level subtrees and never sends identity, timestamps, archive flags, capability metadata, or excluded side effects.
- [x] Every update is read back by exact ID and must match all planned managed values before state reports convergence.
- [x] Out-of-band changes to supported name, email presentation, validation, thank-you display, submit presentation, or style appear as plan drift and are repaired to an empty plan.
- [x] Additive unowned service metadata is decoded and ignored without state churn or a non-empty plan.
- [x] Additional groups or fields, non-email property structure, dependent fields, another consent model, notifications, automation, contact creation, raw HTML, another theme, or another post-submit action fail refresh without mutating remote state or overwriting prior state.
- [x] Fake hooks and regression tests cover supported drift, unsupported structural drift, and harmless metadata evolution under both engines.
- [x] Diagnostics identify the unsupported contract category without exposing raw response bodies, portal identifiers, or form content.

## Answer

Implemented the authoritative presentation lifecycle around the generated Form ID. Schema validators now pin the live-proven input grammar; updates compute a top-level managed diff, issue one non-replayed bounded PATCH, and require exact read-back before state convergence. The typed response classifier ignores additive unknown metadata but rejects every excluded owned structure with sanitized category diagnostics. The shared fake supplies timestamped partial PATCH behavior and supported, unsupported, and future-metadata hooks. Real OpenTofu and Terraform tests prove invalid-plan rejection, no-op suppression, update, drift repair, harmless metadata, fail-closed refresh with prior state retention, and terminal archive.
