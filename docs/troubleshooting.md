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
bounded read-back; pipeline creation remains failed when the response cannot be
matched safely. Updates and deletes retain state until a read confirms the result.

## Destroy is blocked

Read the diagnostic before changing state manually. Property groups may be
nonempty, pipelines may have referenced stages, and custom schemas may have
external properties or HubSpot-side references. Remove dependent configuration
through its owner, apply, and retry.

Custom schemas also require `deletion_protection = false` in an earlier apply.
The destroy preflight refuses to start when it can already see an external
property.

## Drift returns after apply

HubSpot may normalize display order or reject a field combination. Refresh is
read-only, so repeated drift indicates a remote constraint, another writer, or a
provider defect. Save the sanitized diagnostic and provider version when filing
an issue. Never include the token, state file, full response body, CRM record
values, or account-specific IDs.

## State source changed

Use the commands in [state portability](state-portability.md). Keep the generated
backup and confirm an empty plan before removing it.
