# Imports and drift

Import is explicit adoption. It reads HubSpot configuration and writes local
state; it does not change the remote object. Existing objects are never adopted
after a create conflict.

| Resource | Import string |
| --- | --- |
| `hubspot_property_group` | `object_type/group_name` |
| `hubspot_property` | `object_type/property_name` |
| `hubspot_form_definition` | exact lowercase generated UUID |
| `hubspot_file_folder` | exact non-zero decimal generated folder ID |
| `hubspot_file` | exact non-zero decimal generated file ID |

Examples:

```sh
tofu import hubspot_property_group.marketing 'contacts/marketing'
tofu import hubspot_property.customer_tier 'contacts/customer_tier'
tofu import hubspot_form_definition.contact \
  '01234567-89ab-cdef-0123-456789abcdef'
tofu import hubspot_file_folder.assets '123456789'
tofu import hubspot_file.logo '987654321'
```

Replace `tofu` with `terraform` when using Terraform.

Refresh only observes HubSpot. Scalar drift enters state and appears in the next
plan. Property options are a complete owned set. Apply the plan to repair drift,
amend the configuration to accept it, or use import when adopting a supported
object deliberately.

Form definition import is active-only and never accepts a name, URL, or
composite identifier. Supported presentation drift enters state. Unsupported
form structure stops refresh without mutation or overwriting prior state.

Files import is active-only and accepts only the generated folder or file ID.
Names, paths, URLs, hashes, sizes, and timestamps never identify Files
configuration. Imported files still need a configured `source_path` and
`source_sha256`; the first plan replaces remote bytes in place when their MD5 or
size does not match the reviewed source.
