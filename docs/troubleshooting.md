# Troubleshooting

## Missing authentication

Confirm that `HUBSPOT_ACCESS_TOKEN` is exported in the same shell or CI step that
runs OpenTofu. Aliased providers using different accounts need separate sensitive
variables because the shared environment variable supplies only one token.

## HubSpot returns 403

Check the static app's schema scopes for the configured `object_type`, then check
the HubSpot edition and feature flags listed in
[permissions and limits](permissions-and-limits.md). The provider removes neither
state nor remote configuration after a permission error.

## A create or update returned an ambiguous diagnostic

Do not immediately replay an uncertain create outside OpenTofu. Run `tofu plan`
again after the diagnostic. Resources with an immutable recovery key perform a
bounded read-back. Updates and deletes retain state until a read confirms the
result.

## Destroy is blocked

Read the diagnostic before changing state manually. Property groups may be
nonempty. Remove dependent property configuration, apply, and retry.
Account membership removal requires `allow_removal = true`, an exact current
ID/email match, and a non-Super-Admin member. State removal leaves membership
active when deletion is not intended.

## Account Membership Name Update Is Blocked

HubSpot rejects name updates until the user activates and can return
`USER_NOT_ON_ANY_HUBS`. The provider does not retry that response indefinitely.
It also refuses PUT while role or team assignments are present because omission
semantics are undocumented. Activate the user or manage the global name outside
this resource; roles and teams remain outside the v0.6 contract.

## CRM User Profile Has Not Materialized

The Settings membership exists before HubSpot exposes a matching CRM user in
some lifecycles. The provider polls the paginated `hs_internal_user_id` join 20
times at one-second intervals. If the timeout remains, complete any required
human activation and retry. Do not substitute a CRM ID or owner ID for the
Settings user ID.

## CRM User Working Hours Are Rejected

Configure `time_zone` with `working_hours`. Use a documented day value, minute
values from 0 through 1440, and an end later than the start. Grouped day values
expand before overlap checks. Adjacent intervals are valid.

## Drift returns after apply

HubSpot may normalize display order or reject a field combination. Refresh is
read-only, so repeated drift indicates a remote constraint, another writer, or a
provider defect. Save the sanitized diagnostic and provider version when filing
an issue. Never include the token, state file, full response body, CRM record
values, or account-specific IDs.

## State source changed

Use the commands in [state portability](state-portability.md). Keep the generated
backup and confirm an empty plan before removing it.
