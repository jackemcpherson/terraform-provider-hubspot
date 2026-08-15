# Permissions, account tiers, and limits

HubSpot evaluates scopes against the exact CRM object type. Grant only the rows
used by the configuration:

| CRM object type | Read | Write |
| --- | --- | --- |
| `contacts` | `crm.schemas.contacts.read` | `crm.schemas.contacts.write` |
| `companies` | `crm.schemas.companies.read` | `crm.schemas.companies.write` |
| `deals` | `crm.schemas.deals.read` | `crm.schemas.deals.write` |
| `tickets` | `tickets` | `tickets` |

Read-only property data sources need only the read scope. Form definition
management needs the exact `forms` scope. Files configuration management needs
the exact `files` scope. Account membership needs `settings.users.read` and
`settings.users.write`; HubSpot documents `crm.objects.users.read` and
`crm.objects.users.write` as an alternative for the same Settings endpoints.
CRM user profile configuration needs the exact pair
`crm.objects.users.read` and `crm.objects.users.write`. The same pair covers the
linked Settings identity reads.
The provider needs no CRM record, form-submission, or
CMS content scope and never reads CRM record values or submissions.

Some resources also depend on account features:

| Surface | Account requirement | Additional risk |
| --- | --- | --- |
| Property group | Supported CRM object schema access | Nonempty or protected groups may reject archive. |
| Ordinary non-sensitive property | Supported CRM object schema access | Limit telemetry is advisory; remote create responses are authoritative. Definition archive has no provider restore operation. |
| Contact email Form definition | HubSpot Free plus `forms` scope | Only no-consent definitions are supported. Archive is terminal and retained as a tombstone. |
| File folder and Managed file | HubSpot Free plus `files` scope | Only explicit folders and locally supplied reviewed bytes are supported. Normal deletion leaves HubSpot-managed Trash retention. |
| Account membership | HubSpot account plus Settings users read/write or CRM users read/write scopes | Names affect global user identity. Removal is locally guarded and refuses Super Admin membership. |
| CRM user profile | HubSpot account plus CRM users read/write scopes | Profile materialization can require human activation. Destroy retains profile values without a remote write. |

v0.6 accepts only `data_sensitivity = "non_sensitive"`. Sensitive and
highly-sensitive definitions, pipelines, custom schemas, form consent,
notifications, automation, non-email form structures, roles, teams, seats,
granular permissions, activation, deactivation, additional CRM user profile
properties, and global identity deletion are deferred from this release.

HubSpot editions, feature flags, quotas, and scope names can change separately
from this provider. A 403 usually means the token lacks a scope or the account
does not have the required product feature. The provider does not reject creates
from an aggregate ten-property or observed 1000-property value. Missing,
inconsistent, or low limit telemetry cannot fail a plan or preflight; a remote
create error requires cleanup or a capacity change before another apply.

## References

- [HubSpot app scope catalogue](https://developers.hubspot.com/docs/apps/developer-platform/build-apps/authentication/scopes)
- [HubSpot Settings API user provisioning guide](https://developers.hubspot.com/docs/api-reference/latest/account/settings/user-provisioning/guide)
- [HubSpot CRM Users API guide](https://developers.hubspot.com/docs/api-reference/latest/crm/objects/users/guide)
