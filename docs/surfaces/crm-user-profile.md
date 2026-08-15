# CRM User Profile

`hubspot_crm_user_profile` manages selected profile properties on the
account-specific CRM projection of one account membership. The resource uses
`/crm/objects/2026-03/users` and remains separate from
`hubspot_account_membership`.

The resource stores the numeric CRM user `id` as canonical identity. It joins
the configured Settings user ID through `hs_internal_user_id`. The two IDs are
different identity domains and need not have the same value. Create polls the
complete paginated CRM collection 20 times at one-second intervals and requires
one exact join. A timeout can require human activation or CRM materialization.
HubSpot does not publish this readiness bound.

The resource can manage these properties:

- `job_title`.
- `availability_status` with `available` or `away`.
- `time_zone`.
- `working_hours`.

Null fields are unmanaged. Configure at least one managed field. An empty
`job_title` clears that property. Working hours require a managed timezone and
use typed intervals instead of HubSpot's stringified JSON. The provider accepts
documented individual or grouped day values, minutes from 0 through 1440,
strictly increasing ranges, and no overlap after grouped days expand. Adjacent
ranges are valid.

The provider sorts working hours before serialization. Read and update verify
the exact CRM and Settings identities. PATCH sends only changed managed fields,
sets a changed timezone before changed working hours, and sends no request for
a semantic no-op.

Import accepts the canonical CRM ID or `membership:<Settings-ID>`. Both forms
write the CRM ID to state. Import always adopts job title and adopts each other
nonempty supported property. A later configuration selects the final managed
set.

Destroy sends no HubSpot request. The resource stops management and retains the
profile values as a documented non-destructive residual. Remove account
membership separately when required.

The token needs exactly `crm.objects.users.read` and
`crm.objects.users.write` for this surface. The resource excludes membership,
roles, teams, seats, permission sets, invitations, names, email, activation,
and global identity deletion.

## Northstar

The sibling demo's `crm-user-profile` module uses stable local map keys and
consumes the account-membership module's canonical Settings ID. The reference
creates the lifecycle dependency. Teardown stops profile management before it
removes account membership and verifies the retained profile residual.

## References

- [HubSpot CRM Users API guide](https://developers.hubspot.com/docs/api-reference/latest/crm/objects/users/guide)
- [HubSpot CRM Users list endpoint](https://developers.hubspot.com/docs/api-reference/latest/crm/objects/users/get-users)
- [HubSpot CRM Users update endpoint](https://developers.hubspot.com/docs/api-reference/latest/crm/objects/users/update-user)
- [HubSpot Settings API user provisioning guide](https://developers.hubspot.com/docs/api-reference/latest/account/settings/user-provisioning/guide)
