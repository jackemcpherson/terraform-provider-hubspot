# Contact Segment Definition v0.8.0 Specification

This specification freezes the public and remote contract for
`hubspot_contact_segment` in v0.8.0. It manages Contact segment definitions,
not memberships or Contact record values.

## Public Schema

The resource has these root attributes:

- `id` is the computed canonical positive-decimal ILS list ID. It is the sole
  state and import identity.
- `name` is a required, nonblank, mutable name. HubSpot authoritatively
  enforces remote uniqueness. The provider never searches, adopts, or recovers
  a definition by name.
- `processing_type` is required and accepts `manual`, `dynamic`, or `snapshot`.
  A change replaces the resource.
- `filter_groups` is an unordered set of OR branches. Each group contains an
  unordered, nonempty `filters` set whose members are combined with AND.

Each filter has these attributes:

- `property` is a required, nonblank Contact property internal name.
- `operator` is required and accepts `is_equal_to`, `is_not_equal_to`,
  `is_known`, or `is_not_known`.
- `property_kind` accepts `text` or `select`. Equality operators require it.
  Presence operators forbid it.
- `value` is one required, nonblank string for equality operators. Presence
  operators forbid it.

Equality and inequality exclude records whose property is unset. The resource
supports only one string value per predicate. It does not expose multiple
values, numeric comparisons, Boolean comparisons, datetime comparisons,
contains, prefix, suffix, event, association, or historical predicates.

`manual` requires no filter groups. `dynamic` and `snapshot` require at least
one nonempty filter group. The fixed remote object type is Contact `0-1`; the
resource has no public `object_type` attribute.

## Wire Mapping

The resource uses only `/crm/lists/2026-03`.

| Public Predicate | HubSpot Operation Type | HubSpot Operator | Value Field |
| --- | --- | --- | --- |
| text `is_equal_to` | `STRING` | `IS_EQUAL_TO` | `value` |
| text `is_not_equal_to` | `STRING` | `IS_NOT_EQUAL_TO` | `value` |
| select `is_equal_to` | `ENUMERATION` | `IS_ANY_OF` | one `values` item |
| select `is_not_equal_to` | `ENUMERATION` | `IS_NONE_OF` | one `values` item |
| `is_known` | `ALL_PROPERTY` | `IS_KNOWN` | none |
| `is_not_known` | `ALL_PROPERTY` | `IS_UNKNOWN` | none |

Every operation sends `includeObjectsWithNoValueSet=false`. Create and update
bodies use a root OR branch with nonempty nested AND branches. The resource
maps processing types to `MANUAL`, `DYNAMIC`, and `SNAPSHOT`.

Reads accept one-value `STRING` or `MULTISTRING` equality operations as
`property_kind = "text"`. Reads accept one-value `ENUMERATION` equality
operations as `property_kind = "select"`. The provider canonicalises filter
and group ordering before comparison. It rejects multiple values, mismatched
operator shapes, unknown branch forms, and all unsupported remote filters.

## Remote Operations

The typed client uses these exact routes:

- `POST /crm/lists/2026-03` creates a definition.
- `GET /crm/lists/2026-03/{listId}?includeFilters=true` reads one exact ID.
- `PUT /crm/lists/2026-03/{listId}/update-list-name` updates the name.
- `PUT /crm/lists/2026-03/{listId}/update-list-filters` replaces a dynamic
  filter tree.
- `DELETE /crm/lists/2026-03/{listId}` deletes one exact ID.
- `PUT /crm/lists/2026-03/{listId}/restore` restores one exact ID.

The provider validates every returned ID, required field, processing type,
filter shape, tombstone, and list version. It ignores additive response fields.
It does not use name search or record-membership routes.

## Lifecycle

Create preserves a valid returned ID before verification. If HubSpot does not
return an ID, the diagnostic explains that exact-ID import is the only safe
recovery path. The provider never retries an unsafe write automatically.

After create, dynamic filter update, or restore, the provider polls the exact
ID and current `listVersion` for at most five minutes. Completion requires the
same identity and the complete desired definition. Terminal failure and
rejection fail immediately. Stale observations continue polling. Timeout
retains recoverable state. An ambiguous write succeeds only when exact-ID
read-back proves the desired outcome.

Name changes update in place for every processing type. Dynamic filter changes,
including `property_kind`, update in place. Snapshot filter changes replace the
resource. Manual definitions never contain filters.

Refresh is read-only. A supported active definition refreshes canonical state.
Unsupported remote meaning produces an error without dropping it. Permanent
absence removes the resource from state. A later apply creates a new ID.

## Tombstones and Import

HubSpot retains deleted definitions for up to 90 days. When exact-ID read-back
returns the same supported definition with `deletedAt`, refresh retains public
state and stores a private tombstone marker. The next plan shows restoration.
Apply restores the same ID and verifies the active definition.

An active exact-ID import succeeds when the definition is supported. A
tombstoned exact-ID import also succeeds when the complete supported definition
remains readable; the next apply restores the same ID. A malformed ID,
different returned ID, permanently absent ID, or unsupported definition fails
import without adopting by name.

Destroy deletes the exact ID and verifies the same tombstoned identity. An
already deleted or permanently absent definition completes idempotently. The
provider never purges retained tombstones.

## Ownership and Cleanup

The resource owns only the definition ID it created or imported. It does not
own memberships, derived size, folders, permissions, conversions, custom
properties, or Contact records.

Automated cleanup accepts the exact configured ownership prefix and generated
IDs. It deletes only those identities and verifies their tombstones. The
janitor reports retained owned tombstones without broad deletion. Manual
archival requires an explicit surface selection and the existing protected
workflow confirmation.

## Permissions

The complete lifecycle requires only `crm.lists.read` and `crm.lists.write`.
The provider does not read Contact property schema and does not require Contact
record scopes. HubSpot authoritatively rejects a declared `property_kind` that
does not match the named property.

## Demo and Qualification

The cumulative Northstar demo manages stable-keyed manual, dynamic, and
snapshot definitions. Value predicates use existing Contact text and select
property names. The demo never manages segment memberships or Contact values.

Qualification covers create, no-op, rename, dynamic drift and repair, snapshot
replacement, refresh, active import, tombstoned import and restore, destroy,
and terminal cleanup under current OpenTofu and Terraform. Hermetic tests also
cover asynchronous success, failure, timeout, stale reads, ambiguous writes,
purge, unsupported definitions, and exact request counts.

Release qualification follows ADR 0003. It uses exact merged demo and provider
`main` commits and does not add a candidate workflow or release branch.
