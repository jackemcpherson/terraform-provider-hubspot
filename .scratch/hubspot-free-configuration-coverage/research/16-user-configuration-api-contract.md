# User-configuration API contract

- Research date: 2 August 2026.

Evidence type: official HubSpot developer documentation, scope catalogue,
product catalogue, and first-party user-management documentation only; live
behavior belongs in a separate note

## Conclusion

HubSpot documents a Free-capable, GA account-user lifecycle split across two
surfaces:

- the user-provisioning API at `/settings/users/2026-03`, which creates and
  removes account membership and reads or updates email, name, role, and team
  references; and
- the CRM users API at `/crm/objects/2026-03/users`, which reads the
  account-scoped user record and updates selected profile properties such as
  job title, timezone, and working hours.

The least-privilege scope pair for the complete ticket is exactly:

- `crm.objects.users.read`; and
- `crm.objects.users.write`.

The provisioning references accept `crm.objects.users.read` as an alternative
to `settings.users.read`, and `crm.objects.users.write` as an alternative to
`settings.users.write`. The CRM users API requires the corresponding
`crm.objects.users.*` scopes, so requesting the two CRM scopes covers both
surfaces without also requesting `settings.users.read` or
`settings.users.write`. Both scopes and the current user endpoints are
documented for Free accounts.

Do **not** request `settings.users.teams.read` or
`settings.users.teams.write` for this normal-Free lifecycle. Although HubSpot's
scope catalogue lists them as obtainable, teams themselves require a
Professional or Enterprise subscription. Do **not** request
`settings.billing.write`: that scope is needed only when modifying a permission
set that has paid seats attached, which is outside Free. The current exact name
is `settings.billing.write`; the provisioning guide's prose still shortens it
to `billing-write`.

This is an `eligible_narrowed` candidate, subject to live evidence. Provisioning,
generated-ID import, first/last-name update, and CRM user-profile fields are
plausible provider state on Free. Reusable permission-set definitions, arbitrary
granular permissions, teams, paid seats, invitation delivery/acceptance, user
login email changes, deactivation/reactivation, and permanent deletion of the
global HubSpot identity are not a complete stable API lifecycle for this
resource.

Sources: [user-provisioning guide](https://developers.hubspot.com/docs/api-reference/latest/account/settings/user-provisioning/guide),
[retrieve users](https://developers.hubspot.com/docs/api-reference/latest/account/settings/user-provisioning/users/get-users),
[update a provisioned user](https://developers.hubspot.com/docs/api-reference/latest/account/settings/user-provisioning/users/update-user),
[archive a provisioned user](https://developers.hubspot.com/docs/api-reference/latest/account/settings/user-provisioning/users/delete-user),
[CRM users guide](https://developers.hubspot.com/docs/api-reference/latest/crm/objects/users/guide),
[CRM user update](https://developers.hubspot.com/docs/api-reference/latest/crm/objects/users/update-user),
[scope catalogue](https://developers.hubspot.com/docs/apps/developer-platform/build-apps/authentication/scopes),
[create and manage teams](https://knowledge.hubspot.com/user-management/create-and-manage-teams),
and [create and assign permission sets](https://knowledge.hubspot.com/user-management/create-permission-sets).

## Stable endpoint and request matrix

`2026-03` is the current GA date-versioned surface. Numeric `/settings/v3` and
`/crm/v3` pages now live under HubSpot's legacy reference and should not be the
basis of a new provider implementation.

| Concern | Current method and path | Contract |
| --- | --- | --- |
| List account users | `GET /settings/users/2026-03` | Paged list; accepts `limit` and `after`. Returns `email`, generated `id`, `roleIds`, `superAdmin`, names, team IDs, and a non-durable `sendWelcomeEmail` field. |
| Read/import account user | `GET /settings/users/2026-03/{userId}` | Default path identity is `USER_ID`; add `?idProperty=EMAIL` for discovery/adoption by email. |
| Create membership | `POST /settings/users/2026-03` | Required: `email`, `sendWelcomeEmail`. Optional: `firstName`, `lastName`, `roleId`, `primaryTeamId`, and `secondaryTeamIds`. Returns `201` and generated `id`. |
| Update membership | `PUT /settings/users/2026-03/{userId}` | Optional body members are `firstName`, `lastName`, `roleId`, `primaryTeamId`, and `secondaryTeamIds`; email and `superAdmin` are not writable. Returns `200`. |
| Remove membership | `DELETE /settings/users/2026-03/{userId}` | Accepts ID or `idProperty=EMAIL`; returns `204`. The reference title says “Archive a user,” while its description says it removes the user. There is no documented body or restore endpoint. |
| List assignable roles | `GET /settings/users/2026-03/roles` | Returns `id`, `name`, and `requiresBillingWrite`. It does not create roles or expose the granular permissions behind one. |
| List CRM user records | `GET /crm/objects/2026-03/users` | Page with `limit`, `after`, `archived`, and requested `properties`. |
| Read CRM user record | `GET /crm/objects/2026-03/users/{crmUserId}` | Request selected properties explicitly. |
| Search CRM users | `POST /crm/objects/2026-03/users/search` | Active-record discovery only; search can lag recent writes and excludes archived records. |
| Update profile data | `PATCH /crm/objects/2026-03/users/{crmUserId}` | Body `{ "properties": { ... } }`; supports only documented writable user properties. |
| Inspect property metadata | `GET /crm/properties/2026-03/user` | Account-specific property types, labels, read-only flags, and options. The guide uses singular `user` for this schema path. |

Although generic generated CRM-object references expose create/archive shapes
for object type `0-115`, HubSpot's dedicated user guidance assigns provisioning
and deprovisioning to the Settings API and describes the CRM user API as a
retrieve/update surface. A provider resource must therefore create and remove
through `/settings/users/2026-03`, not by manufacturing or archiving the derived
CRM user record.

Sources: [create a user](https://developers.hubspot.com/docs/api-reference/latest/account/settings/user-provisioning/users/create-user),
[retrieve a user](https://developers.hubspot.com/docs/api-reference/latest/account/settings/user-provisioning/users/get-user),
[retrieve users](https://developers.hubspot.com/docs/api-reference/latest/account/settings/user-provisioning/users/get-users),
[update a user](https://developers.hubspot.com/docs/api-reference/latest/account/settings/user-provisioning/users/update-user),
[archive a user](https://developers.hubspot.com/docs/api-reference/latest/account/settings/user-provisioning/users/delete-user),
[retrieve roles](https://developers.hubspot.com/docs/api-reference/latest/account/settings/user-provisioning/roles/get-roles),
[CRM users guide](https://developers.hubspot.com/docs/api-reference/latest/crm/objects/users/guide),
and [CRM search guide](https://developers.hubspot.com/docs/api-reference/latest/crm/search-the-crm).

## Identity, mapping, import, and adoption

HubSpot exposes three different identifiers for one human:

- the Settings/provisioning `id`, identified by the CRM users guide as
  `hs_internal_user_id`, refers to the global HubSpot user across accounts;
- the CRM users `id` equals `hs_object_id` and identifies the user record only
  in the account from which it was requested; and
- the Owners API `id` is `hubspot_owner_id`, which alone is valid when assigning
  a CRM record or activity to an owner. Its `userId` field maps back to the
  Settings user ID.

The account-user resource should store the Settings-generated `id` as canonical
state identity. Import should accept that ID. Email lookup with
`idProperty=EMAIL` is useful for operator-directed adoption, but create must
never silently adopt an existing email after a conflict. The derived CRM user
record can be joined by requesting `hs_internal_user_id` and `hs_email`, and the
owner mapping can be exposed as computed data if needed; neither derived ID
replaces the Settings ID.

Email is a global login identifier. It is required on create but absent from
both API update bodies. HubSpot says only the logged-in user can change their
own login email, the new email must be confirmed, and Super Admins and Support
cannot directly perform that change. A provider must treat email as
replacement-only configuration and surface a user-originated email change as
unreconcilable drift, not try to PATCH it.

The same email can identify one HubSpot user across multiple HubSpot accounts,
and profile changes affect that user's profile in all accounts. Consequently,
first/last-name management is safe for a newly created owned fixture, but a
provider should warn that changing an adopted user's name can have cross-account
effects. The API update is authoritative even though the UI documentation
normally reserves profile-name changes to the user or, narrowly, impersonating
Super Admins.

Sources: [CRM users identity note](https://developers.hubspot.com/docs/api-reference/latest/crm/objects/users/guide),
[Owners API](https://developers.hubspot.com/docs/api-reference/latest/crm/owners/guide),
[specifying a user](https://developers.hubspot.com/docs/api-reference/latest/account/settings/user-provisioning/guide),
[change a user's email address](https://knowledge.hubspot.com/account-management/how-do-i-change-the-email-address-of-a-user),
and [understand HubSpot users and profiles](https://knowledge.hubspot.com/help-and-resources/understand-hubspot-user-profiles).

## Profile and working-hours state

The CRM users API documents these writable properties:

- `hs_additional_phone`;
- `hs_availability_status` (`available` or `away`);
- `hs_job_title`;
- `hs_main_user_language_skill`;
- `hs_out_of_office_hours`;
- `hs_secondary_user_language_skill`;
- `hs_standard_time_zone`;
- `hs_uncategorized_skills`; and
- `hs_working_hours`.

For a narrow Free resource, prioritize `hs_job_title`,
`hs_standard_time_zone`, and `hs_working_hours`. Working hours are a stringified
JSON array of objects with `days`, `startMinute`, and `endMinute`. `days` accepts
the documented single-day values plus `MONDAY_TO_FRIDAY`, `SATURDAY_SUNDAY`, and
`EVERY_DAY`; minutes range from `0` to `1440`; ranges cannot overlap; and
`hs_standard_time_zone` must be set first. A provider schema should expose a
typed keyed/set representation and serialize canonically rather than expose the
raw JSON string.

Out-of-office hours are also serialized JSON and have ordering/non-overlap
rules. Language and uncategorized skills depend on account-defined option sets.
Those are reasonable later additions after property metadata and Free runtime
behavior are known, but they should not be needed to resolve the first narrow
lifecycle.

The dedicated CRM endpoint uses the account-scoped CRM user ID, not the Settings
ID. A live probe must first map the newly provisioned user through
`hs_internal_user_id` or exact owned email; it must never assume the two numeric
IDs are equal.

Source: [CRM users guide](https://developers.hubspot.com/docs/api-reference/latest/crm/objects/users/guide).

## Permissions, roles, teams, and seats on Free

The Settings API can list existing roles and assign one `roleId` to a user.
However, the response exposes only role ID, name, and whether modifying it needs
billing write. It does not expose a complete granular permission document, and
there is no current public role-create endpoint in this API.

Reusable permission-set definitions are Enterprise-only, with a limit of 100.
An account must create a permission set in the HubSpot application before the
API can assign it. Free users can be configured with individual UI permissions,
but the provisioning API does not expose an arbitrary per-permission write
contract. Therefore ticket 16 should treat role assignment as read-only
capability discovery unless the live account already has an unambiguous,
non-billing role fixture. It must not assign an opaque pre-existing role merely
to prove the field works, and permission-set definition management is out of
scope for normal Free.

Teams require Professional or Enterprise, and users are removed from teams when
an account downgrades to Free. Team IDs happen to be optional members of the
provisioning request, but that does not make team assignment a Free lifecycle.
The probe should omit all team fields.

Free tools are a single edition with **up to two free users total**. This product
entitlement is the applicable lifecycle quota. The separate catalogue technical
ceiling of 2,500 users per account is not a Free-seat entitlement and must not be
used as the resource quota. With the original account owner present, a probe can
normally create at most one fixture. It must inventory users first and stop if
the account already has two; it must never remove an unowned user to free a slot.

Sources: [user-provisioning guide](https://developers.hubspot.com/docs/api-reference/latest/account/settings/user-provisioning/guide),
[retrieve roles](https://developers.hubspot.com/docs/api-reference/latest/account/settings/user-provisioning/roles/get-roles),
[create and assign permission sets](https://knowledge.hubspot.com/user-management/create-permission-sets),
[create and manage teams](https://knowledge.hubspot.com/user-management/create-and-manage-teams),
[downgrade effects](https://knowledge.hubspot.com/account-management/downgrade-a-paid-subscription),
and [HubSpot Product & Services Catalog](https://legal.hubspot.com/hubspot-product-and-services-catalog).

## Invitation and human-acceptance boundary

`sendWelcomeEmail` is required on create. Set it to `false` for any automated
probe. The response reports whether a welcome email was sent only for the
provisioning request; subsequent reads return `false`. It is not an invite-state
field and cannot be used for drift detection.

HubSpot's UI has a Pending Invites view, but the documented Settings response
does not expose pending/accepted status. A newly invited user receives a welcome
email, then sets a password and logs in. HubSpot states that the user becomes
eligible for notifications only after being added and setting up their
password. Invite acceptance, password setup, login, 2FA, and self-service email
confirmation are therefore human operations outside a Terraform CRUD
lifecycle.

HubSpot also warns that an invitation address should exist and be active. If the
initial delivery attempt fails, HubSpot blocks that address and Support must
manually unbounce it. Therefore
`tfhs_probe_16_<stamp>@example.com` is **not a safe deliverable invite fixture**
and must never be used with `sendWelcomeEmail: true`.

With `sendWelcomeEmail: false`, the reserved address avoids a delivery attempt
and is suitable for a bounded *account-membership* probe only if the live API
accepts it. HubSpot does not explicitly certify reserved/non-deliverable domains,
so acceptance remains a live question. It also is not residual-free: HubSpot
says a removed account user continues to exist as a global HubSpot identity
until that user logs in and permanently deletes their own user account. A
non-deliverable fixture cannot perform that human cleanup. The probe can verify
zero residual membership in the test account, but it must record the possible
global identity as an external residual.

A genuinely end-to-end invite/accept/delete test requires a controlled,
deliverable mailbox and a human to accept the invitation, then later permanently
delete the global user identity. That is direct operator input and is not needed
to determine whether account-user provisioning is provider-eligible.

Sources: [create a user](https://developers.hubspot.com/docs/api-reference/latest/account/settings/user-provisioning/users/create-user),
[add HubSpot users](https://knowledge.hubspot.com/account-management/add-hubspot-users),
[manage user properties and preferences](https://knowledge.hubspot.com/user-management/manage-user-properties-and-preferences),
and [deactivate and remove users](https://knowledge.hubspot.com/user-management/remove-hubspot-users).

## Removal, restore, reuse, and safety

The public Settings API exposes DELETE but no separate deactivate, reactivate,
restore, or permanent-global-delete operation. HubSpot's first-party UI guidance
distinguishes:

- deactivation, which preserves the account profile and assets and can be
  reversed; and
- removal, which requires prior UI deactivation, removes the account profile,
  cannot be restored, and requires creating the user again.

The API reference calls DELETE “Archive a user” but says it “removes” the user.
The exact live semantics must be probed on an unaccepted, owned fixture: settings
list/read absence, CRM active/archived visibility, and whether recreating the
same email succeeds. Documentation supports re-adding a removed user through
the normal creation process, but does not say whether the Settings ID or CRM ID
is reused. Because the global email identity persists after account removal,
same-email re-add may reuse the global Settings ID while creating or restoring a
different account-scoped record; that is an inference, not a documented fact.

Removal can unassign conversations/assets and affect reports, scheduling pages,
private apps, record ownership, and billing contacts. Never test DELETE on a
pre-existing user. In particular, removing the creator of the private app can
make association calls fail with `USER_DOES_NOT_HAVE_PERMISSIONS`. HubSpot also
prevents a user from permanently deleting their global account when they are
the only remaining Super Admin. The provider needs an explicit safety guard
against deleting itself/the credential owner and the last Super Admin, even if
the remote API also rejects some cases.

Safe destroy preconditions for the probe are all of:

- the target ID is exactly the ID returned by this run's create;
- the target email exactly matches this run's `tfhs_probe_16_` value;
- a fresh read reports `superAdmin: false`;
- the ID is not any baseline user ID and not the credential owner's user ID;
- the target owns no deliberately created assets or CRM records; and
- at least the original baseline Super Admin remains.

Sources: [archive a user](https://developers.hubspot.com/docs/api-reference/latest/account/settings/user-provisioning/users/delete-user),
[deactivate and remove users](https://knowledge.hubspot.com/user-management/remove-hubspot-users),
[HubSpot user permissions guide](https://knowledge.hubspot.com/user-management/hubspot-user-permissions-guide),
and [understand HubSpot users and profiles](https://knowledge.hubspot.com/help-and-resources/understand-hubspot-user-profiles).

## Safe read-only matrix

Use the approved portal fingerprint and emit only aggregate counts, booleans,
field names, and hashed identifiers. Never print baseline emails, names, role
names, user IDs, owner IDs, or profile values.

1. With exact scope `crm.objects.users.read`, page
   `GET /settings/users/2026-03` and record only total count, Super Admin count,
   pending-field presence (expected absent), and whether any owned-prefix user
   exists.
2. Call `GET /settings/users/2026-03/roles` and record only role count and the
   count where `requiresBillingWrite` is true; do not emit names or IDs.
3. Page `GET /crm/objects/2026-03/users` requesting only
   `hs_internal_user_id`, `hs_email`, `hs_job_title`,
   `hs_standard_time_zone`, and `hs_working_hours`; record counts, property
   presence, and whether every Settings user maps uniquely. Hash mapping IDs.
4. Read `GET /crm/properties/2026-03/user` and retain metadata only for the nine
   documented writable properties: type, read-only flags, and option count.
5. Optionally page `GET /crm/owners/2026-03` only if the token already has exact
   scope `crm.objects.owners.read`; record whether Settings-to-owner mappings
   are unique without exposing owner data. Do not request this extra scope just
   for ticket 16; owner mapping is a separate data-source concern.

## Bounded owned lifecycle

Run only if the read-only inventory proves there is exactly one baseline user,
no owned residual, and the exact scopes `crm.objects.users.read` and
`crm.objects.users.write` are usable.

1. Choose one unique `tfhs_probe_16_<UTC run>@example.com` email and call
   `POST /settings/users/2026-03` with `sendWelcomeEmail: false`, short prefixed
   first/last names, and **no** `roleId`, team IDs, seat, or Super Admin request.
   Capture the returned Settings ID. If HubSpot rejects the reserved address,
   stop; do not substitute an operator email or enable delivery.
2. Read the created user by Settings ID and by exact email with
   `idProperty=EMAIL`. Verify identity, `superAdmin: false`, and the non-durable
   nature of `sendWelcomeEmail`.
3. PUT updated owned first/last names by Settings ID. Repeat the exact desired
   request once to characterize no-op behavior, then simulate and reconcile one
   name drift on this owned user only.
4. Locate the derived CRM user record by `hs_internal_user_id` or exact owned
   email. If it does not exist before invite acceptance, record that human
   acceptance gates profile management and skip CRM writes.
5. If the CRM record exists, PATCH `hs_job_title`, then
   `hs_standard_time_zone: "Australia/Melbourne"`, then a non-overlapping
   `hs_working_hours` value. Verify canonical readback, semantic no-op behavior,
   external drift, and clearing only on the owned record.
6. Prove import/adoption by direct Settings ID and discovery by exact email.
   Do not treat an email collision as implicit adoption.
7. Do not assign a role unless the account contains a purpose-built safe role
   whose permissions are independently known. Role-count/readability evidence
   is sufficient to classify the Free permission boundary.
8. Apply the destroy preconditions above, DELETE by Settings ID, and verify
   absence from the Settings list and direct read. Check CRM active and archived
   views only for the mapped owned CRM ID. Do not touch any baseline user.
9. Recreate the same owned email once with `sendWelcomeEmail: false` to settle
   immediate email reuse and ID-reuse behavior; delete the second membership
   under the same guards.
10. Finish with the baseline Settings and CRM aggregate counts restored and zero
    account-scoped `tfhs_probe_16_` membership. Record any CRM tombstone and the
    possible persistent global HubSpot identity as residuals; they cannot be
    permanently cleaned by the admin API.

Do not attempt a third concurrent user merely to prove the documented two-user
quota. A surprising success would create another global identity and consume
additional cleanup capacity. The first-party product catalogue is authoritative
for the exact entitlement; ordinary create errors at quota remain remote errors.

## Unresolved gaps for live evidence

- Whether both exact CRM-user scopes are selectable and usable on the pinned
  normal Free private app.
- Whether `POST /settings/users/2026-03` accepts a reserved `example.com`
  address when `sendWelcomeEmail` is false, and whether it consumes the second
  Free user slot immediately.
- Whether an unaccepted/no-email user appears immediately in the CRM users API,
  and how Settings ID maps to CRM ID before acceptance.
- Live property metadata and Free writability for job title, timezone, and
  working hours; canonical JSON normalization and clearing behavior.
- Whether exact repeated PUT/PATCH requests change timestamps.
- DELETE semantics on the pinned portal: idempotence, direct-read status,
  Settings-list absence, CRM archived tombstone, and propagation delay.
- Whether same-email re-create succeeds immediately and whether it reuses the
  global Settings ID, CRM ID, both, or neither.
- Whether the normal-Free account exposes any assignable roles, and whether the
  API provides enough information to manage one safely. Documentation strongly
  indicates reusable permission-set definitions are not a Free resource.
