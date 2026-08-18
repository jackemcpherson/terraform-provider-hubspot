# 03 Add the Contact Segment Resource

Type: task

Status: open

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
