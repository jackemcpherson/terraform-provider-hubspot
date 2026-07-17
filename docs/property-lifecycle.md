# Property lifecycle

`hubspot_property` owns one definition and, when present, the complete option
map. The option key is the value stored on CRM records; labels and display order
may change without changing that value. Removing or renaming a key can leave
existing records with a value that no longer has a current option, so the update
emits a warning.

The accepted `type` values are `bool`, `enumeration`, `date`, `datetime`,
`string`, and `number`; `field_type` must be nonempty. HubSpot still validates
whether a particular pair and its advanced fields are compatible. When
`external_options = true`, leave `options` unset because HubSpot owns that set.

Changing `type` or `field_type` updates the existing definition, emits a warning,
and can change how HubSpot interprets record values. Review the plan and the
affected data before applying. The provider does not inspect those values.
Unique-value, sensitivity, referenced object type, object type, and internal name
changes replace the resource. v0.1 accepts only
`data_sensitivity = "non_sensitive"`; sensitive and highly-sensitive definitions
are deferred.

Destroy archives a property and removes it from state after a confirming read.
The provider has no restore operation. HubSpot-defined and read-only definitions
cannot be imported as managed resources; use the property-definition data sources
to inspect them.

The singular data source reports a missing definition. The collection returns a
map keyed by internal property name and may return an empty map. `archived = true`
selects archived definitions rather than mixing them with active definitions.
