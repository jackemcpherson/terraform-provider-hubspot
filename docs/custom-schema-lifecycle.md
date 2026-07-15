# Custom schema ownership and teardown

`hubspot_custom_object_schema` owns its bootstrap `properties` map for the full
schema lifetime. Every primary, required, searchable, and secondary display role
must refer to a name in that map. Moving a bootstrap property to a standalone
resource is unsupported because HubSpot needs it during schema creation.

## Split ownership

Additional definitions can use `hubspot_property`. Set their `object_type` from
the schema's computed `object_type_id`, then list their names in
`expected_external_properties`:

```hcl
resource "hubspot_custom_object_schema" "widget" {
  name                         = "widget"
  labels                       = { singular = "Widget", plural = "Widgets" }
  primary_display_property     = "name"
  expected_external_properties = ["category"]

  properties = {
    name = {
      label      = "Name"
      type       = "string"
      field_type = "text"
    }
  }
}

resource "hubspot_property" "widget_category" {
  object_type = hubspot_custom_object_schema.widget.object_type_id
  name        = "category"
  label       = "Category"
  group_name  = "widget_information"
  type        = "string"
  field_type  = "text"
}
```

The dependency removes the standalone property before the schema during destroy.
Expected external names are acknowledged but never adopted or deleted by the
schema resource. Unexpected external properties produce a warning. Every external
property blocks schema deletion until its own manager removes it.

## Safe teardown

Deletion protection defaults to `true`. Disable it in one authored apply, review
the resulting state, then destroy:

```hcl
deletion_protection = false
```

```sh
tofu apply
tofu destroy
```

The destroy preflight reads schema and property metadata using existing schema
scopes. It does not request CRM record scopes. A known blocker stops the operation
before mutation. A HubSpot archive error retains state so a later apply can retry.

Import accepts an object type ID or fully qualified name and stores the canonical
object type ID. Confirm ownership before import: the configured property map is
continuous ownership, not a selection of remote fields.
