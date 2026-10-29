# 03 Add the Contact Segment Resource

Type: task

Status: resolved

Blocked by: 02

## Acceptance

- Register `hubspot_contact_segment` with the frozen schema and generated ID
  as its sole state and import identity.
- Validate processing type, manual restrictions, dynamic and snapshot filter
  requirements, operator and value combinations, and canonical set ordering.
- Keep name updates in place, update dynamic filters in place, and replace on
  processing-type or snapshot-filter changes.
- Reject unsupported remote definitions instead of discarding them.

## Comments

## Answer

`hubspot_contact_segment` is registered with the frozen four-attribute public
schema and generated ILS list ID as its sole identity. Resource validation
enforces the three processing types, omitted manual filters, nonempty dynamic
and snapshot groups, and the conditional operator, `property_kind`, and value
contract.

Nested sets make group and filter order non-semantic. Processing-type changes
replace, snapshot filter changes replace, and dynamic filter or name changes
update in place. Exact remote reads remain strictly typed, so unsupported
branches or operations fail refresh without discarding prior state.
