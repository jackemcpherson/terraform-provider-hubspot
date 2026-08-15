# CRM User Profile v0.6.0 Specification

This specification freezes the public and operational contract for the v0.6.0
CRM user profile release. The adjacent research notes separate official API
guarantees, provider policy, and live evidence.

## Public Resource

Register `hubspot_crm_user_profile`. Keep the resource separate from
`hubspot_account_membership`.

| Attribute | Contract |
| --- | --- |
| `id` | Computed canonical account-specific CRM user ID. |
| `account_membership_id` | Required Settings user ID. A change replaces the management relationship. |
| `job_title` | Optional managed `hs_job_title` property. An empty string clears it. |
| `availability_status` | Optional managed value. Accept `available` or `away`. |
| `time_zone` | Optional managed `hs_standard_time_zone` property. |
| `working_hours` | Optional set of `days`, `start_minute`, and `end_minute` objects. |

A null optional field is unmanaged. Require at least one managed field.
Require `time_zone` when `working_hours` is managed.

## Working Hours

Accept all seven individual day values and these grouped values:

- `MONDAY_TO_FRIDAY`.
- `SATURDAY_SUNDAY`.
- `EVERY_DAY`.

Accept minute values from `0` through `1440`. Require each end minute to be
later than its start minute. Expand grouped day values before overlap checks.
Adjacent intervals do not overlap.

Sort intervals by explicit day rank, start minute, and end minute before JSON
serialization. The canonical order and overlap model are provider policy. The
HubSpot documentation does not define these details.

## Identity And Readiness

Use the CRM response `id` as canonical state identity. Join the configured
Settings user ID through `hs_internal_user_id`. Page the complete active CRM
user collection and require exactly one match.

Creation starts management of an existing CRM profile. Poll the join up to 20
times at one-second intervals. A timeout explains that the membership has not
materialized and can require human activation. The bound and activation
condition come from provider policy and prior live evidence.

Read and update require all of these identity checks:

- The exact Settings read returns `account_membership_id`.
- The exact CRM read returns state `id`.
- The CRM `hs_internal_user_id` equals `account_membership_id`.

Only HTTP `404` proves remote absence. A missing membership or CRM projection
removes this management relationship from state. Other errors retain state.

## Update And Destroy

Compare managed properties semantically. Send no `PATCH` for a no-op. Send only
changed managed properties. If timezone and working hours both change, verify
the timezone update before the working-hours update.

Verify each update through fresh exact CRM and Settings reads. An ambiguous
transport result succeeds only when read-back verifies every changed property.

Destroy performs no remote write. It stops management and retains profile
values in HubSpot as documented residual configuration.

## Import

Accept one canonical CRM user ID or `membership:<Settings-ID>`. Resolve the
membership form through the unique paginated join. Store the CRM user ID as the
canonical state ID for both forms.

Import adopts job title and each other nonempty supported property. A later
configuration selects the final managed property set.

## Permissions And Exclusions

Require exactly these scopes for this surface:

- `crm.objects.users.read`.
- `crm.objects.users.write`.

Exclude account membership, roles, teams, seats, permission sets, invitations,
welcome-email delivery, activation, email or name changes, and global identity
deletion.

## Verification And Delivery

Test the typed client, provider resource, and real command-line lifecycle
seams. Cover validation, canonical JSON, pagination, readiness timeout,
ambiguous identity, import, drift, no-op writes, API rejection, unknown fields,
and zero-write destroy.

Preserve ADR 0003. Pin maintenance to the exact merged v0.6.0 demo commit.
Publish only after the guarded live probe, local gates, specialist reviews,
hosted required check, and immutable release preconditions pass.
