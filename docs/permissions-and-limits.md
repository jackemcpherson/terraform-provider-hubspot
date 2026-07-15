# Permissions, account tiers, and limits

HubSpot evaluates scopes against the exact object type. A contacts property
group, for example, needs `crm.schemas.contacts.read` and
`crm.schemas.contacts.write`. Read-only property data sources need the read
scope. Grant the corresponding schema scopes for every object type in the
configuration, using the current scope names shown by HubSpot when the static app
is configured.

Some resources also depend on account features:

| Surface | Account requirement | Additional risk |
| --- | --- | --- |
| Property group | Supported CRM object schema access | Nonempty or protected groups may reject archive. |
| Property | Supported CRM object schema access | Definition archive has no provider restore operation. |
| Deal pipeline | Sales Hub Starter or above, plus pipeline capacity | Records that reference stages may block archive. |
| Custom object schema | Enterprise, Sensitive Data eligibility when enabled, and custom-object capacity | External properties and other HubSpot references may block deletion. |
| Sensitive property | Enterprise with Sensitive Data enabled and current object-specific sensitive write scope | HubSpot permanently deletes archived definitions after 90 days. |

`data_sensitivity` describes a property definition and is therefore visible in
state. The provider never requests CRM record scopes and never reads sensitive
record values. Use `non_sensitive` unless the account and static app have been
prepared for `sensitive` or `highly_sensitive` definitions.

HubSpot editions, feature flags, quotas, and scope names can change separately
from this provider. A 403 usually means the token lacks a scope or the account
does not have the required product feature. A quota response requires cleanup or
capacity changes in HubSpot before another apply.
