# 05 — Compose keyed Form definitions for consumers

**What to build:** Give OpenTofu consumers an opinionated `form-definition` module that manages multiple independent forms through stable local keys without exposing raw HubSpot schema choices.

**Blocked by:** 04 — Make Form definition identity recovery safe.

**Status:** resolved

- [x] The module accepts a `forms` map keyed by validated stable local identity and creates one `hubspot_form_definition` per entry with keyed ownership.
- [x] Each entry requires a remote name and offers typed optional overrides for every supported email, validation, thank-you, submit, and style value.
- [x] Defaults expand to the complete conservative live-proven provider contract with no raw JSON, arbitrary property, arbitrary form type, consent, notification, automation, branding, or submission input.
- [x] Duplicate remote names within one module instance fail validation, while direct resources continue to reflect HubSpot's duplicate-name behavior.
- [x] Module output is exactly a map of generated form IDs keyed by the stable input keys.
- [x] The module has no sideways dependency on the CRM property schema module; its use of the built-in contact email Property definition is documented as a semantic dependency.
- [x] Module tests cover default expansion, typed overrides, invalid keys, duplicate names, malformed presentation, stable keyed resource addresses, and ID output shape.
- [x] Key-rename guidance requires an explicit `moved` block and explains that an un-migrated key replacement archives and recreates the form.
- [x] OpenTofu remains primary, Terraform protocol compatibility remains supported, and the module pins a v0.3.0-compatible provider constraint.

## Answer

Added the `modules/form-definition` consumer module to the sibling demo repository, following the established `crm-schema` boundary. Its single `forms` map uses validated stable keys, requires mutable remote names, expands complete conservative defaults, and exposes typed optional overrides for every supported email, validation, configuration, submit, and style value. It creates keyed `hubspot_form_definition` instances and outputs only generated IDs by key; duplicate names fail within the module while no raw or side-effecting Forms schema is exposed. The README documents the built-in contacts `email` semantic dependency, terminal replacement risk, and an explicit caller-side `moved` block. Provider hermetic tests under both engines prove invalid-key and duplicate-name rejection, malformed presentation, exact default and full override payloads, stable child addresses, keyed output shape, moved-key identity preservation without another POST, and zero-active teardown. `make check-go`, `make test-hermetic`, and the demo repository's `make check` pass.
