# Contact Segment Definition

`hubspot_contact_segment` manages one Contact segment definition through
HubSpot's Lists `2026-03` API. It works with HubSpot Free and requires only
`crm.lists.read` and `crm.lists.write`. It does not read Contact records,
memberships, property values, or property schema.

The resource stores the generated positive-decimal ILS list `id` as its only
state and import identity. Name is mutable presentation and HubSpot enforces
its uniqueness. The provider never searches, adopts, or recovers by name.

`processing_type` accepts `manual`, `dynamic`, or `snapshot` and changes replace
the resource. Manual definitions have no filters. Dynamic and snapshot
definitions require at least one OR group, with each group's filters combined
using AND. Group and filter order is not semantic.

Supported Contact property predicates are deliberately narrow:

| Operator | Required fields | Meaning |
| --- | --- | --- |
| `is_equal_to` | `property_kind`, `value` | One exact text or select value; unset properties are excluded. |
| `is_not_equal_to` | `property_kind`, `value` | One different text or select value; unset properties are excluded. |
| `is_known` | neither | The property has a value. |
| `is_not_known` | neither | The property has no value. |

For equality operators, declare `property_kind = "text"` or `"select"`.
HubSpot validates that declaration against the named property. The provider
does not request Contact property schema. Presence operators forbid both
`property_kind` and `value`.

Dynamic filter changes update in place. Snapshot filter changes replace the
definition because a snapshot's evaluated population is immutable. Name
changes update in place for every processing type.

Create, dynamic filter updates, and restoration poll the exact ID and current
`listVersion` for up to five minutes. Pending and stale observations continue;
terminal failure or rejection stops immediately. An ambiguous write succeeds
only when exact-ID read-back proves the complete desired definition. A create
without a returned ID stops with exact-ID import guidance and never searches by
name.

Refresh is read-only. Supported out-of-band changes enter state and appear in
the next plan. Unsupported filter meaning stops refresh without discarding the
prior state. Permanent absence removes state, so a later apply creates a new
ID.

HubSpot retains deleted lists for up to 90 days. When the exact-ID read returns
the supported definition with `deletedAt`, refresh keeps state and records a
private tombstone marker. The next apply restores that same ID and verifies the
current definition. Import supports both active definitions and readable
tombstones; importing a tombstone likewise plans same-ID restoration.

Destroy deletes the exact ID and verifies the same tombstoned identity before
removing state. Already deleted or permanently absent definitions complete
idempotently. The provider never purges retained tombstones.

Memberships, derived size, folders, permissions, conversions, custom
properties, record reads, raw JSON, event or association filters, historical
predicates, multiple values, numeric, Boolean, datetime, contains, prefix, and
suffix operations are outside this surface.

## Northstar

The sibling demo's stable-keyed `contact-segment-definition` module manages
manual, dynamic, and snapshot definitions. Its cumulative lifecycle covers
rename, dynamic drift repair, snapshot replacement, active and tombstoned
import, same-ID restoration, destroy, and terminal cleanup under OpenTofu and
Terraform without managing Contact membership.

## References

- [HubSpot Lists API guide](https://developers.hubspot.com/docs/api-reference/latest/crm/lists/guide)
- [HubSpot list filter structure](https://developers.hubspot.com/docs/api-reference/latest/crm/lists/list-filters)
- [HubSpot app scope catalogue](https://developers.hubspot.com/docs/apps/developer-platform/build-apps/authentication/scopes)
