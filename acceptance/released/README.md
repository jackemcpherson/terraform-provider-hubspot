# Released-artifact fixtures

Each capability shard must provide `acceptance/released/<shard>/main.tf.tmpl`
before a candidate can be announced. The fixture uses the literal placeholders
`__PROVIDER_SOURCE__` and `__PROVIDER_VERSION__`, plus the sensitive variables
`hubspot_access_token` and `acceptance_prefix`. It must create, reconcile, import,
drift-check, and destroy only configuration owned by that prefix.

The `form_definitions` fixture is intentionally driven as one serialized state
journey rather than as independent per-engine shards. Terraform creates one Form
definition, the state and generated ID migrate to OpenTofu's provider source and
back, and the final Terraform phase archives that exact identity once.

The verification harness installs the published provider from its registry; it
does not use a development override. A missing fixture, unavailable entitlement,
non-empty second plan, or failed destroy keeps release verification red.
