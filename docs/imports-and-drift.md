# Imports and drift

Import is explicit adoption. It reads HubSpot configuration and writes local
state; it does not change the remote object. Existing objects are never adopted
after a create conflict.

| Resource | Import string |
| --- | --- |
| `hubspot_property_group` | `object_type/group_name` |
| `hubspot_property` | `object_type/property_name` |

Examples:

```sh
tofu import hubspot_property_group.marketing 'contacts/marketing'
tofu import hubspot_property.customer_tier 'contacts/customer_tier'
```

Replace `tofu` with `terraform` when using Terraform.

Refresh only observes HubSpot. Scalar drift enters state and appears in the next
plan. Property options are complete owned sets. Apply the plan to repair drift,
amend the configuration to accept it, or use import when adopting a supported
object deliberately.
