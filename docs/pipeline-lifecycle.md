# Deal pipeline lifecycle

`hubspot_pipeline` owns a complete deal pipeline and its writable stages. Stage
map keys are Terraform-local for newly created pipelines; imported or observed
out-of-band stages use their HubSpot stage IDs permanently. Keys are not labels
and cannot be renamed in place.

Deal stage `metadata.probability` must be between `0.0` and `1.0` in `0.1`
increments. Refresh is read-only and preserves stage IDs. Pipeline deletion is
archive-based and can fail when CRM records still reference a stage.
