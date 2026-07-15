# Pipeline lifecycle

`hubspot_pipeline` owns a complete pipeline and its writable stages. Stage
map keys are Terraform-local for newly created pipelines; imported or observed
out-of-band stages use their HubSpot stage IDs permanently. Keys are not labels
and cannot be renamed in place.

Deal stage `metadata.probability` must be between `0.0` and `1.0` in `0.1`
increments. Ticket metadata accepts `ticketState = "OPEN"` or `"CLOSED"`.
Metadata for other object types is preserved as string keys and values.

Refresh is read-only and preserves known stage IDs and local keys. Removing a
writable stage from configuration asks HubSpot to remove it; records that refer to
the stage may block the operation. Import rejects pipelines containing
`READ_ONLY` or `INTERNAL_ONLY` stages because the resource could not own the full
set.

Pipeline deletion uses HubSpot archival and removes state only after a confirming
read. A create with an uncertain response is not replayed because a label is not a
safe identity. The provider does not promise restore behavior.
