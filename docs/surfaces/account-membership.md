# Account Membership

`hubspot_account_membership` manages one user's membership in one HubSpot
account through `/settings/users/2026-03`. Its canonical identity is the numeric
Settings user `id`, which is distinct from CRM user and owner IDs. The provider
does not manage a CRM user profile.

Creation requires `email` and an explicit `send_welcome_email` choice. Both are
replacement-only because HubSpot accepts them only when adding membership.
Provider acceptance and maintenance never request a real welcome email.
HubSpot does not report historic welcome delivery, so imports record
`send_welcome_email = false`.

Names are optional configured or computed global-user attributes. HubSpot blocks
name updates until activation. The provider surfaces `USER_NOT_ON_ANY_HUBS`
without indefinite retry. A fresh read must show no role or team assignment
before PUT because HubSpot does not document whether omitted assignment fields
are cleared, retained, or defaulted. Unconfigured name changes remain observed
state and never send PUT.

Import accepts one canonical Settings ID or the explicit
`email:operator@example.com` form, then stores the canonical ID. A create
conflict never adopts an existing membership. Only HTTP 404 means absence.

Destroy is deliberately guarded. `allow_removal` defaults to `false`; when it is
true, the provider rereads the exact ID and email, refuses a Super Admin, sends
DELETE, and proves ID, email, and collection absence. This removes account
membership, not the global HubSpot identity. State removal is the
non-destructive alternative.

The token needs `settings.users.read` and `settings.users.write`. HubSpot also
documents `crm.objects.users.read` and `crm.objects.users.write` as an
alternative for these endpoints.

This surface excludes roles, teams, seats, granular permissions, activation
state, CRM profile fields, deactivation, and deletion of the global user
identity.

## Northstar

The sibling demo's `account-membership` module uses stable local map keys,
requires an explicit welcome choice, exposes canonical generated IDs, and keeps
removal disabled unless its caller opts into a disposable-account teardown.

## References

- [HubSpot Settings API user provisioning guide](https://developers.hubspot.com/docs/api-reference/latest/account/settings/user-provisioning/guide)
- [HubSpot Settings API user endpoints](https://developers.hubspot.com/docs/api-reference/latest/account/settings/user-provisioning/users/get-users)
- [HubSpot app scope catalogue](https://developers.hubspot.com/docs/apps/developer-platform/build-apps/authentication/scopes)
- [HubSpot user profiles](https://knowledge.hubspot.com/help-and-resources/understand-hubspot-user-profiles)
