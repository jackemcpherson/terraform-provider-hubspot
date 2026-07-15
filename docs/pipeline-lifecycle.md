# Pipeline lifecycle

> Development roadmap: `hubspot_pipeline` is not registered in
> `v0.1.0-alpha.1`; its live gates require an eligible paid HubSpot account.

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
read proves that the same pipeline ID is archived. If refresh or import observes
that canonical archived pipeline, the provider retains its pipeline and stage
identities in Terraform state and marks its archived status privately; the next
plan proposes an in-place restore, and apply verifies the same active identity
before reconciling configuration. A create with an uncertain response is not
replayed because a label is not a safe identity.

Omitting pipeline or stage `display_order` requests HubSpot's append behavior. The
provider retains the `-1` intent only while the pipeline or stages remain at the
end of their remote ordering, so an out-of-band reorder remains visible as drift.
