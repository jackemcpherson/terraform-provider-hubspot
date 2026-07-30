# Permissions, Account Tiers, and Limits

HubSpot evaluates scopes against the exact object type. A contacts property
group, for example, needs `crm.schemas.contacts.read` and
`crm.schemas.contacts.write`. Read-only property data sources need the read
scope. Grant the corresponding schema scopes for every configured object type.
Use the current scope names that HubSpot shows for the static app.

Some resources also depend on account features:

| Surface                         | Account requirement                                                            | Additional risk                                                                                 |
| ------------------------------- | ------------------------------------------------------------------------------ | ----------------------------------------------------------------------------------------------- |
| Property group                  | Supported CRM object schema access                                             | Nonempty or protected groups may reject archive.                                                |
| Ordinary non-sensitive property | Supported CRM object schema access and available Free custom-property capacity | HubSpot Free permits ten custom properties. The provider cannot restore an archived definition. |

v0.1 accepts only `data_sensitivity = "non_sensitive"`. The provider never
requests CRM record scopes and never reads CRM record values. Sensitive and
highly-sensitive definitions are outside this release. The release also excludes
pipelines and custom schemas.

HubSpot editions, feature flags, quotas, and scope names can change separately
from this provider. A 403 usually means the token lacks a scope or the account
does not have the required product feature. A quota response requires cleanup or
capacity changes in HubSpot before another apply.
