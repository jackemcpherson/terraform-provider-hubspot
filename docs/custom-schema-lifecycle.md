# Custom schema lifecycle

`hubspot_custom_object_schema` owns its bootstrap property map for the lifetime
of the schema. Every role reference must point to an owned property. Import may
use the returned object type ID or fully qualified name; state stores the
canonical object type ID. This core surface does not read CRM records or perform
split-ownership teardown; those safeguards are covered by later frontiers.
