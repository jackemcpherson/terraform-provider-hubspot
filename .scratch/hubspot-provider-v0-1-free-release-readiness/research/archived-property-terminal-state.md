# Archived CRM property terminal state

Research date: 2026-07-17. Sources below are HubSpot-owned API references,
the public API specification, and HubSpot's knowledge base.

## Finding

The public CRM Properties API exposes **archive**, not a permanent
delete/purge operation, for both property definitions and property groups:

- `DELETE /crm/properties/2026-03/{objectType}/{propertyName}` is explicitly
  ["Archive a property"](https://developers.hubspot.com/docs/api-reference/latest/crm/properties/delete-property): it moves the property to the recycling bin.
- `DELETE /crm/properties/2026-03/{objectType}/groups/{groupName}` is explicitly
  ["Archive a property group"](https://developers.hubspot.com/docs/api-reference/latest/crm/properties/property-groups/delete-property): it moves the group to the recycling bin.
- HubSpot's [published Properties OpenAPI specification](https://github.com/HubSpot/HubSpot-public-api-spec-collection/blob/main/PublicApiSpecs/CRM/Properties/Rollouts/145899/2026-03/properties.json)
  lists only create/read/update/archive operations for properties and groups.
  It contains no restore or permanent-delete/purge operation.

The API does support observing archive state: the [property read
schema](https://developers.hubspot.com/docs/api-reference/latest/crm/properties/get-property)
contains `archived` and `archivedAt`, and the [property-group read
schema](https://developers.hubspot.com/docs/api-reference/latest/crm/properties/property-groups/get-property)
contains `archived`.

For **property definitions**, HubSpot documents a UI-only lifecycle beyond the
public API: archived properties remain in the Archived tab, are permanently
deleted after 90 days, and may be restored or permanently deleted through the
HubSpot UI before then. See [Organize, delete, and export
properties](https://knowledge.hubspot.com/properties/organize-and-export-properties).
That article does not document an API route for either action. It does not
establish a corresponding property-group restore/purge API.

## Recreate and restore conclusion

There is **no documented public API operation** to restore an archived property
definition or group, nor an API contract guaranteeing that an archived or
permanently deleted internal name can be recreated. The create endpoints alone
are not evidence of that behavior. A lifecycle test must therefore not depend
on API-based restore, permanent purge, or reuse of an archived name.

## Technically supportable cleanup invariant

The strongest fully automated invariant is:

> After destroy, every provider-owned property definition and property group is
> absent from the active configuration surface; it may remain archived in
> HubSpot's recycle-bin retention state.

To guarantee physical removal before the retention window, an authorised human
must use HubSpot's UI permanent-delete control for properties. For repeatable
automated runs, use a fresh, unique name/prefix per run and budget the resulting
archived definitions; do not treat archive as a reusable-name or quota-release
guarantee.
