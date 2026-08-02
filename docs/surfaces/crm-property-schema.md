# CRM property schema

The v0.2.0 surface manages ordinary non-sensitive property groups, property
definitions, and parent-owned enumeration options for `contacts`, `companies`,
`deals`, and `tickets` on a normal HubSpot Free account. OpenTofu is primary;
Terraform uses the same protocol contract.

## Permissions

- Contacts: `crm.schemas.contacts.read` and `crm.schemas.contacts.write`.
- Companies: `crm.schemas.companies.read` and `crm.schemas.companies.write`.
- Deals: `crm.schemas.deals.read` and `crm.schemas.deals.write`.
- Tickets: `tickets` for read and write.

The provider manages account-level schema only. It does not require CRM record
scopes and never reads CRM records or record values.

## Identity and ownership

Groups and properties use canonical `object_type/name` identity. Consumer module
map keys are immutable remote names. Option map keys are immutable enumeration
option values. Labels, descriptions, display order, visibility, grouping, and
option presentation remain mutable desired state.

One `crm-schema` module instance owns one CRM object type. It derives `text` as
HubSpot `string`/`text` and `select` as `enumeration`/`select`. Direct resources
remain the advanced interface; the module has no raw type pair or escape hatch.

## Lifecycle, drift, and teardown

Creates and updates are verified by read-back. Refresh observes out-of-band
changes; repair requires an authored apply. Destroy archives properties before
their groups through resource references. Nonempty-group archival fails without
discarding state. Archived properties remain discoverable, while property and
group names can be reused immediately. Reuse is creation, not restoration, and
does not migrate CRM record values.

Generated ordering is normalized to the configured `-1` append sentinel so an
unchanged second plan remains empty. Quota telemetry is advisory: no aggregate
ten-property or observed 1000-property value is local admission control. Remote
create responses are authoritative.

## Exclusions

Sensitive definitions, CRM record values, calculated and unique-ID properties,
external options, references, validation rules, custom object schemas, additional
editor kinds, restoration, and permanent deletion are outside this surface.

## Northstar

The sibling Northstar demo instantiates the generated `crm-schema` module for all
four supported object types. Local candidate and registry journeys use reviewed
plans, empty-plan verification, drift repair, canonical adoption, refresh, and
reviewed archival teardown under both engines.
