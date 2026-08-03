# 02 — Manage one typed Form definition end to end

**What to build:** Let a consumer declare one live-proven contact-email Form definition, observe its generated identity, reach an empty follow-up plan, and archive it safely through the real provider protocol.

**Blocked by:** None — can start immediately.

**Status:** resolved

- [x] The provider registers exactly one new type named `hubspot_form_definition` while every previously released type remains registered and every deferred pipeline type remains unregistered.
- [x] The resource exposes only computed `id` plus the complete explicit managed presentation contract accepted by the v0.3.0 specification.
- [x] Field groups and fields are ordered lists validated to exactly one default group and one contact email field.
- [x] Form type, contact/email identity, no dependent fields, no consent, no notifications, no automation, no contact creation, safe rendering mode, and default theme remain provider-owned invariants rather than configurable escape hatches.
- [x] A typed client creates through production Forms v3, accepts the live `201` result, reads by generated ID, and archives by exact ID using the `forms` scope.
- [x] The shared behavioral fake models generated IDs, active and archived views, canonical supported response state, and terminal archive for Form definitions.
- [x] Real OpenTofu and Terraform CLIs can create the same supported configuration against the fake, assert the generated ID, re-plan with no changes, destroy it, and verify active absence plus the exact Archived form definition.
- [x] Every new schema and nested attribute has a complete consumer-facing Markdown description, including the narrow evidence boundary and terminal archive behavior.
- [x] Existing property-schema, protocol, state-upgrade, registration, and schema-description tests remain green.
