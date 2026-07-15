# Imports and drift

Import is explicit adoption. It reads HubSpot configuration and writes local
state; it does not change the remote object. Existing objects are never adopted
after a create conflict.

| Resource | Import string |
| --- | --- |
| `hubspot_property_group` | `object_type/group_name` |
| `hubspot_property` | `object_type/property_name` |
| `hubspot_pipeline` | `object_type/pipeline_id` |
| `hubspot_custom_object_schema` | returned `2-...` object type ID or fully qualified name |

Examples:

```sh
tofu import hubspot_property_group.marketing 'contacts/marketing'
tofu import hubspot_property.customer_tier 'contacts/customer_tier'
tofu import hubspot_pipeline.sales 'deals/<pipeline-id>'
tofu import hubspot_custom_object_schema.widget '<object-type-id>'
tofu import hubspot_custom_object_schema.widget 'p<hub-id>_widget'
```

Replace `tofu` with `terraform` when using Terraform. A custom-schema fully
qualified name is an input alias; state records the returned `2-...` object type
ID after the read.

Imported pipelines use each HubSpot stage ID as its permanent map key:

```hcl
stages = {
  "<stage-id>" = {
    label    = "Qualification"
    metadata = { probability = "0.1" }
  }
}
```

New pipelines may use readable keys such as `qualification`. Nested map keys
cannot be renamed with a `moved` block. A stage created outside OpenTofu enters
state under its remote ID; the next authored apply removes it because the stage
map is exclusively owned.

Refresh only observes HubSpot. Scalar drift enters state and appears in the next
plan. Property options and pipeline stages are complete owned sets, while custom
schema refresh never adopts an external property into its owned map. Apply the
plan to repair drift, amend the configuration to accept it, or use import when
adopting a supported object deliberately.
