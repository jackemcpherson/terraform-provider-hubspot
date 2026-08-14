# Account Membership v0.5.0 Specification

This specification freezes the public and operational contract for the v0.5.0
account-membership release. Official and live evidence is in the adjacent
`research` directory.

## Public Resource

Register `hubspot_account_membership`. Do not add a data source.

| Attribute | Contract |
| --- | --- |
| `id` | Computed canonical Settings user ID. |
| `email` | Required. A change replaces the membership. |
| `first_name` | Optional and computed. A configured value must be nonblank. |
| `last_name` | Optional and computed. A configured value must be nonblank. |
| `send_welcome_email` | Required creation choice. A change replaces the membership. |
| `allow_removal` | Optional local guard. Defaults to `false`. |
| `super_admin` | Computed safety observation. |

The resource manages account membership only. It excludes CRM user-profile
properties, roles, teams, seats, permissions, activation state, deactivation,
and global identity deletion.

## Identity And Import

Use the Settings user ID returned by `/settings/users/2026-03` as canonical
state identity. Do not equate it with CRM user or owner IDs.

Import accepts one canonical Settings ID or `email:<address>`. Email import
uses an exact `idProperty=EMAIL` read and stores the returned Settings ID.
Import records `send_welcome_email = false` and `allow_removal = false`.

A duplicate or ambiguous create must not search by email for adoption. A
successful response that contains a canonical ID can recover only by exact-ID
read-back of every configured remote value.

## Read And Update

Only HTTP `404` proves absence. All other errors retain state.

Unconfigured names remain observations. Configured name drift is repaired only
after a fresh exact-ID and exact-email match. A name PUT requires all current
role and team fields to be empty because HubSpot does not document omission
semantics.

Surface `USER_NOT_ON_ANY_HUBS` as one terminal update error. Do not retry it.
Warn that a name change can affect the global HubSpot identity.

## Removal

Destroy requires all of these conditions:

- `allow_removal` is `true` in state.
- A fresh read returns the exact state ID and email.
- The fresh read returns `superAdmin: false`.

After DELETE, require exact ID and email `404` plus eventual collection
absence. A known remote absence completes destroy. State removal is the
documented non-destructive alternative.

## Typed Client And Fake

The typed client must paginate list reads, ignore unknown response fields, and
use exact ID and email routes. Create is not replayable. Name PUT is not
replayable. Exact DELETE can use explicit replay safety and must verify absence.

The behavioural fake must cover duplicate and ambiguous create, activation
failure, role and team assignments, Super Admin state, delayed collection
absence, disappearance, and same-email ID reuse.

## Verification

Run lifecycle tests through current OpenTofu and Terraform command-line
interfaces. Never send a live welcome email. Test `true` only against the fake
and typed request boundary.

The final gate includes `make check`, `make test-hermetic`, race tests, fuzz
seeds, generated-document checks, security checks, and release checks. Review
the complete diff from `b2ca1fcb5f54696b34bdfa14e6442da052921487` against
this specification and repository standards.

## Delivery

Add the stable-keyed `account-membership` demo module. Extend cumulative
maintenance and guarded archival cleanup without changing ADR 0003. Publish
through the manual immutable release workflow after the provider and demo pull
requests pass their required checks.
