# Property Group Lifecycle

`hubspot_property_group` manages one account-level CRM configuration group. It
does not manage CRM records or values.

## Permissions

Use the least-privilege schema read and write scopes for the CRM object type.
Contacts use `crm.schemas.contacts.read` and `crm.schemas.contacts.write`. Other
object types use their corresponding schema scopes.

## Import

Import uses the exact composite identity:

```sh
tofu import hubspot_property_group.marketing contacts/marketing
```

The import string must contain exactly one slash: `object_type/group_name`.
Import is explicit adoption. The provider does not auto-adopt a create conflict.

## Lifecycle and Drift

Refresh only observes HubSpot. It never repairs a group on its own. A changed
label or display order appears in the next plan. Only an authored apply repairs
the group.

`object_type` and `name` are immutable identities and force replacement. `label`
and `display_order` update in place. HubSpot archives a group on destroy. The
provider cannot restore a group. Confirmed absence or archival removes state, while
transient, permission, malformed, or ambiguous responses retain state and show a
diagnostic.

HubSpot may reject archival of a nonempty or protected group. The provider retains
state and reports that rejection. It does not delete contained properties or CRM
records.
