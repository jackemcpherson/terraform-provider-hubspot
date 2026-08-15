# Official CRM User Profile API Contract

Research date: 15 August 2026. This note checks the v0.6.0 CRM user profile
contract against current first-party HubSpot documentation. It identifies
published API guarantees separately from provider policy and live evidence.

## Result

The planned resource has no direct conflict with the current official API
contract. HubSpot still documents the `2026-03` CRM Users and Settings User
Provisioning surfaces. The exact scopes `crm.objects.users.read` and
`crm.objects.users.write` cover the planned reads, identity join, and profile
updates.[^crm-users-guide] [^settings-list] [^settings-read] [^crm-update]

The following parts of the plan are provider policy or live-evidence rules.
HubSpot does not publish them as API guarantees:

- A CRM user appears within 20 one-second polls after an account membership
  becomes available.
- Invite acceptance, login, or another activation action is the exact condition
  that materialises the CRM user record.
- A working-hours range must have `startMinute < endMinute`.
- Grouped day values must be expanded in a specific way for overlap checks.
- Working-hours array order has no meaning, or one JSON order is canonical.
- A semantic no-op must avoid a `PATCH` request.
- A null Terraform value means that the provider does not manage the property.

These gaps do not block implementation. The resource must describe them as
bounded readiness and provider lifecycle rules. It must not present them as
published HubSpot guarantees.

## Endpoint And Scope Contract

HubSpot assigns account membership and profile data to separate APIs. The
Settings User Provisioning guide manages account users, permissions, and names.
It directs job-title and working-hours operations to the CRM Users
API.[^user-provisioning-guide]

| Operation | Method And Path | Required Scope |
| --- | --- | --- |
| Discover CRM users | `GET /crm/objects/2026-03/users` | `crm.objects.users.read` |
| Read one CRM user | `GET /crm/objects/2026-03/users/{userId}` | `crm.objects.users.read` |
| Update one CRM user | `PATCH /crm/objects/2026-03/users/{userId}` | `crm.objects.users.write` |
| Discover account memberships | `GET /settings/users/2026-03` | `crm.objects.users.read` or `settings.users.read` |
| Read one account membership | `GET /settings/users/2026-03/{userId}` | `crm.objects.users.read` or `settings.users.read` |

The CRM list accepts `after`, `limit`, and requested `properties`. A response
with another page supplies the next cursor as `paging.next.after`. The published
default CRM page size is 10.[^crm-list]

The Settings list also accepts `after` and `limit`. It returns the next cursor
as `paging.next.after`. It does not publish a default or maximum page
size.[^settings-list] Both discovery operations must follow all cursors.

The Settings exact read treats its path value as a Settings user ID by default.
It also accepts `idProperty=EMAIL`. The documented `idProperty` values are
`USER_ID` and `EMAIL`.[^settings-read] The planned profile resource needs only
ID-based reads and does not need a Settings write scope.

The CRM Users guide documents retrieve and update operations. It does not direct
clients to create or delete CRM user records. The Settings API owns provisioning
and deprovisioning.[^crm-users-guide] [^user-provisioning-guide] A profile
resource that performs no remote write during destroy preserves this boundary.

## Identity And Join Contract

A CRM response `id` equals `hs_object_id`. This identity represents the user
only in the HubSpot account that returned it. The Settings User Provisioning
`id` is exposed on the CRM user as `hs_internal_user_id` and identifies the user
across HubSpot accounts. The Owners API `hubspot_owner_id` is a third identity
domain.[^crm-users-guide]

The resource can therefore use this join:

```text
Settings user id == CRM property hs_internal_user_id
CRM canonical state id == CRM response id == hs_object_id
```

HubSpot does not guarantee that the join is immediately present or globally
unique in a list response. A provider should request `hs_internal_user_id`, page
the complete CRM collection, and require exactly one match. Missing and
duplicate matches must remain distinct errors.

Exact reads should verify both identity domains. A CRM read by canonical CRM ID
must still return the configured `hs_internal_user_id`. A Settings read by the
configured account membership ID must return that exact Settings `id`.

Import syntax is a provider contract. HubSpot does not define OpenTofu or
Terraform import forms. A plain CRM ID can use the exact CRM read. A
`membership:<Settings-ID>` form can use the paginated join. Both forms can then
write the account-specific CRM ID as canonical state.

## Managed Property Contract

The four planned properties are writable CRM user properties.[^crm-users-guide]

| Provider Property | HubSpot Property | Published Contract |
| --- | --- | --- |
| `job_title` | `hs_job_title` | String job title. |
| `availability_status` | `hs_availability_status` | Exactly `available` or `away`. |
| `time_zone` | `hs_standard_time_zone` | Standard TZ identifier. Required before working hours. |
| `working_hours` | `hs_working_hours` | Stringified JSON array of working-hours objects. |

HubSpot documents these `days` values:

- `MONDAY_TO_FRIDAY`.
- `SATURDAY_SUNDAY`.
- `EVERY_DAY`.
- `MONDAY`.
- `TUESDAY`.
- `WEDNESDAY`.
- `THURSDAY`.
- `FRIDAY`.
- `SATURDAY`.
- `SUNDAY`.

Each working-hours object contains `days`, `startMinute`, and `endMinute`.
HubSpot documents an inclusive range of `0` through `1440` for both minute
values. It also states that working hours cannot overlap.[^crm-users-guide]
Validation must not reduce the upper bound to `1439`.

HubSpot does not define these details:

- Whether equal start and end values are valid.
- Whether adjacent ranges overlap.
- How overlap applies when aggregate and individual day values intersect.
- Whether duplicate day coverage is valid.
- Whether the JSON array order has semantic meaning.

The planned ordering, expanded-day overlap checks, and canonical serialization
are appropriate provider rules. Tests must identify them as provider semantics.
A stable serializer should sort by an explicit day rank, then by start minute
and end minute. This rule avoids state drift without claiming that HubSpot
requires that order.

HubSpot states that timezone must be set before working hours. The same guide
also shows one `PATCH` body that contains both properties.[^crm-users-guide]
Sending a changed timezone before changed working hours is a conservative client
sequence. The documentation does not require two requests in that case.

## Read And Update Semantics

CRM reads return requested properties only when the caller supplies the
`properties` query parameter. The list reference states that a requested
property which is absent from a record is ignored.[^crm-list] The client must
distinguish a missing property from an empty string where the response permits
that distinction.

The update operation accepts a required `properties` map. The CRM Users guide
says to include the properties to update and lists the writable set. Its example
uses this shape:[^crm-users-guide] [^crm-update]

```json
{
  "properties": {
    "hs_standard_time_zone": "America/Detroit",
    "hs_working_hours": "[{\"days\":\"SATURDAY\",\"startMinute\":540,\"endMinute\":1020}]"
  }
}
```

Changed-only `PATCH` requests fit this partial-update shape. Null-as-unmanaged,
managed-property comparison, and semantic no-op suppression remain provider
rules. The resource should omit unmanaged properties from every request.

The resource should require at least one managed property. This requirement is
not a HubSpot API precondition. It prevents a state-only resource that has no
declarative effect.

## Readiness And Activation Boundary

Official documentation does not state when Settings provisioning creates a CRM
user record. It does not document an activation status on either read response.
The CRM Users guide gives no readiness service-level
objective.[^crm-users-guide]

HubSpot's account setup documentation says a new user receives a welcome email
to set a password and log in. It says a user can set up their profile after
login. It also permits welcome-email suppression when the user already has a
HubSpot password for another account.[^add-users] This describes a human setup
boundary. It does not prove that login is the condition for CRM row
materialisation.

A bounded join poll is therefore justified by live evidence, not by the current
official API contract. A timeout should explain that the account membership has
not materialised as a CRM user profile and may require human activation. The
message should not claim that activation always resolves the condition.

## Lifecycle And Surface Boundaries

The resource can manage job title, availability, timezone, and working hours
without managing account membership, roles, teams, seats, invitations, names,
email, or global identity deletion. These exclusions follow the published split
between the Settings and CRM surfaces.[^user-provisioning-guide]

Stopping management on destroy is non-destructive. HubSpot documents no profile
delete operation in the dedicated CRM Users guide. Retained profile values are
therefore expected residual configuration, and destroy should document this
result.

HubSpot describes one user identity across accounts and says a removed account
user remains in HubSpot until that user deletes their own account. It also says
general HubSpot profile changes affect all accounts.[^user-profiles] The CRM
Users guide separately defines its CRM ID as account-specific. The official
documents do not state whether each of the four planned CRM property values has
cross-account effects. Documentation should avoid promising that a property
update cannot affect another account.

## Contradictions And Published Gaps

No contradiction requires stopping implementation. The following qualifications
must remain visible in the frozen specification:

- The minute range includes `1440`.
- All three grouped day values and all seven individual day values are valid.
- The docs prohibit overlap but do not define the provider's full overlap model.
- The docs require timezone before working hours but show both in one request.
- Readiness timing and activation-dependent materialisation are unpublished.
- Join uniqueness, canonical JSON, and no-op suppression are provider rules.
- A CRM user ID is account-specific. A Settings user ID is cross-account.
- Official product documentation does not resolve whether these four profile
  properties can have effects outside the current account.

## References

[^crm-users-guide]: [HubSpot CRM Users API guide](https://developers.hubspot.com/docs/api-reference/latest/crm/objects/users/guide)
[^settings-list]: [Retrieve Settings users](https://developers.hubspot.com/docs/api-reference/latest/account/settings/user-provisioning/users/get-users)
[^settings-read]: [Retrieve a Settings user](https://developers.hubspot.com/docs/api-reference/latest/account/settings/user-provisioning/users/get-user)
[^crm-update]: [Update a CRM user](https://developers.hubspot.com/docs/api-reference/latest/crm/objects/users/update-user)
[^user-provisioning-guide]: [HubSpot Settings User Provisioning guide](https://developers.hubspot.com/docs/api-reference/latest/account/settings/user-provisioning/guide)
[^crm-list]: [List CRM users](https://developers.hubspot.com/docs/api-reference/latest/crm/objects/users/get-users)
[^add-users]: [Add HubSpot users](https://knowledge.hubspot.com/account-management/add-hubspot-users)
[^user-profiles]: [Understand HubSpot users and profiles](https://knowledge.hubspot.com/help-and-resources/understand-hubspot-user-profiles)
