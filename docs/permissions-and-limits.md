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

The Free alpha accepts `data_sensitivity = "non_sensitive"` only and does not
request CRM record scopes or read CRM record values. Paid pipeline, custom-schema,
advanced-property, and sensitive-property surfaces remain unregistered until
their isolated live acceptance gates pass.

HubSpot editions, feature flags, quotas, and scope names can change separately
from this provider. A 403 usually means the token lacks a scope or the account
does not have the required product feature. A quota response requires cleanup or
capacity changes in HubSpot before another apply.
