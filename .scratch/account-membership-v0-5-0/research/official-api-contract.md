# Official Account Membership API Contract

Research date: 2026-08-15. This note defines the official HubSpot contract for
the v0.5.0 account-membership resource. It separates published guarantees from
provider safety rules where HubSpot does not publish enough detail.

## Contract Summary

Use the Settings User Provisioning API at `/settings/users/2026-03`. HubSpot
documents this API for provisioning and deprovisioning account users. The API
also sets user names and account permissions. The v0.5.0 resource must not use
the CRM Users API for profile properties.[^provisioning-guide]

Use the returned Settings `id` as the canonical resource identifier. HubSpot
calls this value `hs_internal_user_id` in its CRM Users guide. The value refers
to one user across all HubSpot accounts. It differs from the account-local CRM
user `id` and `hs_object_id`. It also differs from the Owners API
`hubspot_owner_id`.[^crm-users-guide]

The supported v0.5.0 remote fields are:

- `id`, `email`, `firstName`, `lastName`, and `superAdmin`.
- `sendWelcomeEmail` during creation only.
- Role and team fields for safety checks, but not for configuration.

The API can return fields outside this resource, including `seatNames`. A typed
client must ignore unknown response fields. This rule allows HubSpot to add
response fields without breaking account-membership reads.

## List, Read, and Import

`GET /settings/users/2026-03` returns `results` and optional `paging`. The list
endpoint accepts `after` and `limit`. When another page exists,
`paging.next.after` contains its cursor. A complete list operation must follow
the cursor until `paging.next` is absent.[^list-users]

`GET /settings/users/2026-03/{userId}` uses a Settings user ID by default. The
same endpoint accepts an email path value when `idProperty=EMAIL`. The only
documented `idProperty` values are `USER_ID` and `EMAIL`.[^get-user]

These reads support two explicit import forms:

- A plain value is a Settings user ID and uses the default `USER_ID` lookup.
- An `email:<address>` value uses an `EMAIL` lookup after removing the prefix.

This import syntax is a provider convention. HubSpot does not define Terraform
or OpenTofu import syntax. Both successful lookups return the canonical
Settings `id`, so the provider can replace the import input with that value.
The provider must not interpret an unprefixed email as an email lookup.

HubSpot ties one platform user identity to a login email. The same user can
belong to multiple HubSpot accounts when those accounts use the same email.
Removing one account membership does not delete that identity.[^user-profiles]

## Create

`POST /settings/users/2026-03` requires `email` and `sendWelcomeEmail`. It also
accepts optional `firstName`, `lastName`, `primaryTeamId`, `roleId`, and
`secondaryTeamIds`. A successful request returns `201` and the created user,
including the canonical `id`.[^create-user]

`sendWelcomeEmail` records a creation action, not durable remote state. HubSpot
only populates the true delivery choice in the provisioning response.
Subsequent reads return `false`.[^create-user] An import therefore cannot
recover the historic choice and must record `false`.

The create reference does not document idempotency, duplicate-email adoption,
or a conflict recovery contract. A failed duplicate create must remain an
error. The provider must not adopt an existing membership after a failed
create. Explicit import is the documented provider path for adoption.

## Name Update Safety

`PUT /settings/users/2026-03/{userId}` accepts `firstName`, `lastName`,
`primaryTeamId`, `roleId`, and `secondaryTeamIds`. Each field is optional in the
published request schema. The endpoint also accepts `idProperty=EMAIL`, though
the provider can use its canonical Settings ID.[^update-user]

HubSpot does not state whether an omitted role or team field is preserved,
cleared, or recalculated. The method is `PUT`, and the reference does not call
the operation a partial update. A name-only request is therefore unsafe when
the current user has any role or team assignment.

The v0.5.0 resource must fail closed when any current role or team field is
non-empty. It may send a name-only `PUT` when all current role and team fields
are empty. This restriction avoids managing assignments and avoids relying on
undocumented omission semantics.

The official references do not document activation-state restrictions or the
`USER_NOT_ON_ANY_HUBS` error. The provider must return such a response as an
actionable update error. It must not treat the response as absence or retry it
without a bound.

Names require an additional identity warning. HubSpot describes a user as one
identity across the platform and says profile changes affect all accounts.
HubSpot also limits name edits in its user interface.[^user-profiles]
[^profile-preferences] The API guide permits `firstName` and `lastName`, but it
does not limit those changes to one account.[^provisioning-guide] A name update
can therefore affect the user's global HubSpot profile.

## Delete and Absence

`DELETE /settings/users/2026-03/{userId}` removes the identified account user
and returns `204` without a body. The endpoint also supports `idProperty=EMAIL`.
HubSpot titles the operation "Archive a user", but describes it as removing the
user.[^delete-user]

This operation removes an account membership. It is not a global identity
delete. HubSpot states that the identity tied to the email continues to exist
after account access is removed. The user must separately delete their user
account to remove the global identity.[^remove-users]

The API response exposes `superAdmin`, but the delete reference does not state
whether the endpoint refuses Super Admin removal. The provider's
`superAdmin == false` check is a local safety guard, not a published endpoint
precondition. The same applies to the local `allow_removal` opt-in.

The delete reference does not document eventual consistency or a successful
deletion verification response. A bounded exact-ID reread is necessary because
the `204` response has no representation of the deleted membership.

The individual read reference documents `200` plus a generic error response.
It does not publish an endpoint-specific `404` contract.[^get-user] HubSpot has
historically identified `404 Not Found` as the correct response for a missing
specific record.[^missing-record-changelog] The safe provider classification
is therefore narrow: only HTTP `404` proves absence. Authentication,
authorization, validation, rate-limit, transport, and server failures must
remain errors.

HubSpot's user-interface guide has extra removal prerequisites and broad asset
effects. It requires deactivation before removal and warns about assignments,
assets, reports, private apps, and billing contacts.[^remove-users] The API
reference does not publish the deactivation prerequisite. The v0.5.0 resource
must not infer or manage deactivation, assets, seats, or ownership from the
Settings delete endpoint.

## Scopes

Use the Settings scopes in user-facing documentation:

- `settings.users.read` for list and individual reads.
- `settings.users.write` for create, update, and delete.

The endpoint references also accept `crm.objects.users.read` for reads and
`crm.objects.users.write` for writes.[^list-users] [^get-user] [^delete-user]
The official scope catalogue describes the Settings pair as access to account
users and their permissions.[^scopes]

No team, role, seat, or billing scope belongs in the minimal resource contract.
The resource observes role and team identifiers returned by user reads but does
not call the separate team or role endpoints. It does not assign paid seats.

## Published Gaps and Implementation Consequences

Official HubSpot documentation does not define these behaviours:

- Duplicate-create status, response shape, or adoption semantics.
- Case handling for email lookup.
- `PUT` omission semantics for roles and teams.
- Pre-activation name-update behaviour or `USER_NOT_ON_ANY_HUBS`.
- Delete propagation time or a Settings-specific missing-user response body.
- Whether the API itself blocks deletion of a Super Admin.

The provider must preserve errors at these gaps. It must not infer remote state
from a generic error body. Bounded live probes can establish portal behaviour,
but probe evidence does not expand the published API contract.

## References

[^provisioning-guide]: [Settings API user provisioning guide](https://developers.hubspot.com/docs/api-reference/latest/account/settings/user-provisioning/guide)
[^crm-users-guide]: [CRM Users API guide](https://developers.hubspot.com/docs/api-reference/latest/crm/objects/users/guide)
[^list-users]: [Retrieve users](https://developers.hubspot.com/docs/api-reference/latest/account/settings/user-provisioning/users/get-users)
[^get-user]: [Retrieve a user](https://developers.hubspot.com/docs/api-reference/latest/account/settings/user-provisioning/users/get-user)
[^user-profiles]: [Understand HubSpot users and profiles](https://knowledge.hubspot.com/help-and-resources/understand-hubspot-user-profiles)
[^create-user]: [Create a user](https://developers.hubspot.com/docs/api-reference/latest/account/settings/user-provisioning/users/create-user)
[^update-user]: [Update a user](https://developers.hubspot.com/docs/api-reference/latest/account/settings/user-provisioning/users/update-user)
[^profile-preferences]: [Manage your user profile and preferences](https://knowledge.hubspot.com/user-management/profile-and-preferences)
[^delete-user]: [Archive a user](https://developers.hubspot.com/docs/api-reference/latest/account/settings/user-provisioning/users/delete-user)
[^remove-users]: [Deactivate and remove HubSpot users](https://knowledge.hubspot.com/user-management/remove-hubspot-users)
[^missing-record-changelog]:
    [HubSpot changelog correction for missing-record status codes](https://developers.hubspot.com/changelog/2018-10-09-issue-empty-http-responses-incorrectly-returning-http-200)
[^scopes]: [HubSpot app scope catalogue](https://developers.hubspot.com/docs/apps/developer-platform/build-apps/authentication/scopes)
