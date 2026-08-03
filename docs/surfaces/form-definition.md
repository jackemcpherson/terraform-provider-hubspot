# Form definition

The v0.3.0 surface manages reusable active HubSpot Form definitions through the
production `/marketing/v3/forms` API on a normal Free account. OpenTofu is the
primary engine; Terraform uses the same provider protocol contract. The access
token needs the exact `forms` scope.

## Identity and composition

HubSpot's generated lowercase UUID is the only remote identity. Names are
mutable presentation and exact duplicate active names are legal. Direct
resources therefore never find, import, or recover a form by name.

The `form-definition` consumer module creates one resource for each stable local
map key and returns only generated IDs by key. It does not depend on the
`crm-schema` module: the built-in contacts `email` Property definition is a
semantic prerequisite supplied by HubSpot. A map-key rename without migration
archives and recreates the form; use an explicit `moved` block to preserve the
generated identity during an intentional refactor.

## Supported contract and exclusions

The supported aggregate is one `hubspot` Form definition containing one visible
contacts `email` field in one default group. It owns the email label,
description, placeholder, required state and block-list settings; English
language and safe known-value behavior; reCAPTCHA; thank-you display; submit
presentation; and the live-proven style fields.

Only the no-consent model is supported. Do not use this resource where the form
must collect consent. Submissions, responses, notification recipients,
automation, contact creation, non-email or dependent fields, CRM property option
values, branding removal, restore, raw HTML, arbitrary JSON, and the beta
date-versioned Forms API are outside this surface. Unsupported owned structure
fails refresh without mutation or state replacement; additive unowned service
metadata is ignored.

## Drift and bounded writes

Refresh observes supported presentation drift by exact ID. Apply sends one
non-replayed PATCH containing only changed supported top-level subtrees, reads
the exact ID back, and reports convergence only when every planned managed value
matches. A semantic no-op sends no PATCH.

Create sends one non-replayed POST. The wire request supplies HubSpot's required
creation and update timestamps plus `archived = false`; this service metadata is
not Terraform desired state and is never included in PATCH. HubSpot's returned
`[""]` sentinel for no blocked email domains is normalized to the configured
empty list. If HubSpot returns a generated ID before the response or verification
fails, that ID remains in state for refresh or exact import. If no ID is known,
identify the active form in HubSpot and import its exact generated ID; the
provider never guesses from name or payload similarity.

## Import, external archive, and destroy

Import accepts one exact lowercase generated UUID for a supported active form.
Names, URLs, composite identifiers, Archived form definitions, and unsupported
definitions are rejected without remote mutation:

```sh
tofu import hubspot_form_definition.contact \
  '01234567-89ab-cdef-0123-456789abcdef'
```

External archival is terminal active absence. Refresh verifies the exact UUID
through the archived view, removes it from state, and plans a new generated ID
for still-declared configuration. Complete absence from both active and archived
views behaves the same way.

Destroy sends DELETE only for a verified active UUID, then verifies active
absence and the exact Archived form definition before removing state. Already
archived or permanently absent identities complete idempotently. HubSpot retains
Archived form definitions as tombstones for about three months; the provider
does not purge or restore them. Use `tofu state rm` instead of destroy when the
active form must remain unmanaged.
