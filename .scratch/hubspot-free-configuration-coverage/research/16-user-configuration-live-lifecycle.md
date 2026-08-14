# User-configuration live lifecycle

- Execution timestamps: 2026-08-02T02:39:16Z to 2026-08-02T02:48:23Z.
- Final successful prefix: `tfhs_probe_16_20260802024807`.
- Snapshot date: 30 July 2026.
- Credential class: normal-Free static token from the dedicated local probe
  Keychain entry.
- Portal fingerprint: `sha256:c26c791399aeb246`.
- Account response: `STANDARD`, AP1.
- APIs: `/settings/users/2026-03` and `/crm/objects/2026-03/users`.
- Exact scopes: `crm.objects.users.read`, `crm.objects.users.write`.

Harnesses: [read-only reachability](../probe-app/16-user-configuration-reachability.zsh), [owned lifecycle](../probe-app/16-user-configuration-lifecycle.zsh), and [owned cleanup](../probe-app/16-user-configuration-owned-cleanup.zsh)

## Classification

The candidate is **eligible_narrowed** as two related contracts:

1. an account-membership resource can create, discover/import, read, and remove
   a Free user by the generated Settings ID, with email replacement-only; and
2. profile settings can be managed only after a CRM user record materializes,
   using the distinct CRM record identity joined through
   `hs_internal_user_id`.

Fresh welcome-disabled users are not a complete mutable resource before human
activation. Their Settings name update returned `400 USER_NOT_ON_ANY_HUBS`, and
the first fresh identity had no CRM profile record. A previously created global
identity did materialize a CRM record on reprovision and passed job-title,
availability, timezone, and working-hours update, no-op, drift, readback, and
restoration. A provider must therefore model profile materialization as a
bounded readiness gate, not assume it follows account membership synchronously.

Teams, reusable permission sets, granular permissions, paid seats, invitation
delivery/acceptance, password/login/2FA, email mutation, deactivation/reactivation,
and permanent global-identity deletion remain excluded.

## Authorization and opening state

The complete least-privilege scope set is exactly
`crm.objects.users.read` and `crm.objects.users.write`. Those scopes authorized
both Settings provisioning and CRM profile operations. No `settings.users.*`,
team, owner, or billing scope was needed.

The pinned portal opened with one protected baseline Settings user, who was the
only Super Admin. No user had a role or team assignment. The opening active CRM
user collection contained three records, but only one mapped to an active
Settings membership through `hs_internal_user_id`. The probe emitted no names,
emails, IDs, role names, or profile values.

The role endpoint returned `400 VALIDATION_ERROR` with `Account doesn't have
access to roles.` This agrees with reusable permission sets being an
Enterprise boundary. `archived=true` on CRM users also returned
`400 VALIDATION_ERROR`: deleted-object paging is not supported for object type
`0-115 (USER)`.

## Schema and identity

The account-specific user schema exposed eight of the nine documented writable
properties with `readOnlyValue:false`: additional phone, availability status,
job title, main language skill, out-of-office hours, timezone, uncategorized
skills, and working hours. The secondary-language property was absent from this
portal's schema. `hs_internal_user_id` was present as the join to the Settings
identity.

Create returned `201`, a generated Settings ID, `superAdmin:false`, and
`sendWelcomeEmail:false`. Direct reads by generated ID and by exact email with
`idProperty=EMAIL` returned the same identity. Subsequent reads exposed no
pending/accepted state and reported no durable welcome-email state. Generated
Settings ID is canonical state/import identity; exact email is discovery for an
explicit adoption workflow, not implicit collision adoption.

Email is not writable on PUT and must be replacement-only. Settings ID,
account-scoped CRM user ID, and owner ID are separate identity domains; this
probe joined Settings to CRM by `hs_internal_user_id` and never assumed numeric
equality.

## Human-acceptance and update boundary

Every create used a unique reserved `example.com` address and
`sendWelcomeEmail:false`, so no delivery was attempted. The fresh account
membership was readable immediately, but an exact name-only PUT—even a semantic
no-op—returned `400` with the nested reason `USER_NOT_ON_ANY_HUBS`. Its CRM
profile join count was zero. Name update, profile update, invite acceptance,
password setup, and login therefore cannot be promised as an immediate
post-create lifecycle for a new non-accepted user.

On a later reprovision of the already-created global identity, the CRM profile
join count was one. The owned record then passed:

- job title, availability, and `Australia/Melbourne` timezone PATCH;
- a subsequent non-overlapping Monday-to-Friday 09:00–17:00 working-hours PATCH;
- canonical readback through the CRM user ID;
- a semantic no-op that preserved identity and did not change `updatedAt`;
- owned out-of-band job-title/availability drift and direct observation; and
- restoration of every captured original profile value before deprovisioning.

This proves profile manageability when the derived record exists, while also
proving that membership creation alone is not its readiness condition.

## Permissions, teams, quota, and safety

The Free portal had no accessible role catalogue, so no opaque role was assigned.
Teams are paid-tier and no team endpoint or team scope was used. Arbitrary
individual permissions are not exposed by this API. Permission assignment is
therefore excluded from the normal-Free managed surface rather than represented
as an unverified field.

The official Free entitlement is two total users. With one baseline user, the
probe created one fixture and deliberately did not attempt a third user: a
surprising success would manufacture another global identity. At-limit errors
remain remote API diagnostics.

All opening user IDs were protected before mutation. A local self-removal guard
proved that the deletion function refuses a baseline ID without making a network
request. Before every fixture deletion, a fresh read had to match the exact
owned email, report `superAdmin:false`, and have an ID absent from the baseline.
No baseline user was changed or deleted.

## Removal, reuse, and cleanup

DELETE by owned Settings ID returned `204`. Direct reads by both ID and email
then returned `404`. Repeating DELETE returned `204`, so removal is idempotent.
Immediate reprovisioning of the same email returned `201` and reused the same
global Settings ID. The recreated membership was deleted under the same guards.

Settings collection removal can lag direct reads. Diagnostic exits required a
separate exact-prefix cleanup, and the final harness polls and repeats guarded
deletion until the collection no longer contains the fixture. Final state was:

- one Settings user, equal to the protected opening baseline;
- zero owned `tfhs-probe-16-…@example.com` memberships;
- three active CRM user records, equal to the opening count; and
- no queryable archived CRM-user collection because the selector is unsupported.

Three unique non-deliverable emails were created across diagnostic runs. Their
account memberships were all removed, but HubSpot may retain their global user
identities. The admin API exposes no permanent-global-delete operation; only
the user can complete that human cleanup. The final evidence run reused an
existing diagnostic identity to avoid adding another residual.

## Lifecycle gates

| Gate | Result |
| --- | --- |
| Exact Free scopes | Passed with only `crm.objects.users.read` and `crm.objects.users.write`. |
| Account identity/import | Passed by generated Settings ID; exact email lookup maps to it. |
| Create/read | Passed with welcome delivery disabled and non-Super-Admin fixture. |
| Invitation state | Excluded; pending/accepted status is not exposed. |
| Settings update | Human-acceptance-gated; fresh user returned `USER_NOT_ON_ANY_HUBS`. |
| CRM identity mapping | Passed through `hs_internal_user_id` when a profile exists. |
| Profile/working hours | Passed on the materialized owned profile, including no-op, drift, and restoration. |
| Permissions/roles/teams | Excluded on normal Free; role API is unavailable and teams are paid. |
| Two-user quota | Official exact entitlement recorded; unsafe third-user attempt omitted. |
| Self-removal safety | Passed provider-side baseline guard; no baseline DELETE was sent. |
| Deprovision/read/repeat | Passed with `204`, ID/email `404`, and repeat `204`. |
| Email reuse | Passed immediately and reused the global Settings ID. |
| Cleanup | Passed with one baseline membership and unchanged active CRM-profile count. |

## Specification consequences

- Model account membership by generated Settings ID, with email required and
  replacement-only; offer exact-email discovery only for explicit import/adoption.
- Treat `sendWelcomeEmail` as a create-only operator choice, never durable state;
  acceptance and login remain human operations.
- Do not promise first/last-name updates or CRM profile availability until the
  user is active/materialized. Surface `USER_NOT_ON_ANY_HUBS` and bounded
  readiness diagnostics rather than retrying indefinitely.
- Model an optional profile subresource or separately ready profile capability
  keyed by CRM user ID and joined through `hs_internal_user_id`. Use typed
  working-hours state and canonical JSON serialization.
- Expose only account-present writable profile properties; the live schema is
  authoritative because the secondary-language property was absent.
- Exclude roles, granular permissions, teams, paid seats, email changes,
  deactivation, and global identity deletion from the Free resource.
- Guard destroy against baseline/self/last-Super-Admin targets, verify exact
  ownership immediately before DELETE, poll Settings collection absence, and
  document that account removal cannot purge the global HubSpot identity.
