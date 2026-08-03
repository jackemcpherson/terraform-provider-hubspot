# 06 — Publish executable Forms documentation locally

**What to build:** Make the shipped Form definition resource and module discoverable through generated, provenance-bound local documentation that explains the real lifecycle and limitations before users apply it.

**Blocked by:** 05 — Compose keyed Form definitions for consumers.

**Status:** resolved

- [x] Generated provider reference includes `hubspot_form_definition` only because the resource is actually registered, with complete nested schema descriptions and an exact-ID import example.
- [x] Generated module reference reflects the actual keyed inputs, safe defaults, typed overrides, validation, provider requirement, resources, and ID output.
- [x] The Form definition surface overview documents exact `forms` scope, identity, duplicate-name semantics, bounded PATCH, drift, unsupported-structure failure, terminal archive, recreation, and retained tombstones.
- [x] Documentation prominently states the no-consent-only limitation and excludes submissions, notifications, automation, contact creation, non-email fields, property options, branding removal, restore, and beta APIs.
- [x] Direct-resource and module examples are complete, formatted, and validated by the supported OpenTofu toolchain.
- [x] Module key-move, ambiguous-create recovery, active-only import, external archive, and destroy guidance match the executable contract.
- [x] The local documentation portal groups the surface overview, provider reference, and module reference and records exact clean provider/demo commits plus candidate version.
- [x] Documentation regeneration produces a clean diff and the local portal builds successfully.
- [x] Existing CRM property schema and global documentation views remain present and correct.

## Answer

Generated the registered `hubspot_form_definition` reference from the real provider schema, including every nested description and the active-only exact-UUID import example. Added the Form definition surface overview and updated global permissions, imports, destroy, authentication, README, and provider descriptions to cover the exact `forms` scope, identity, bounded writes, drift, unsupported-structure safety, no-consent boundary, terminal archival, recreation, recovery, and exclusions. The documentation portal now requires and groups both released surfaces, discovers the real `form-definition` module plus its generated README and standalone usage, renders Forms-specific import/lifecycle guidance, defaults to candidate version 0.3.0, and tests clean exact provider/demo provenance anchors and deterministic regeneration. The demo repository adds the complete module example and validates it under both engines while checking generated module docs. Provider engine smoke validates the exact direct resource example under both engines. `make check-go`, `make check-docs`, `make engine-smoke`, `make docs-portal`, and the demo `make check` pass.
