# Released-Artifact Fixtures

Each capability shard must provide `acceptance/released/<shard>/main.tf.tmpl`
before the release workflow announces a candidate. The fixture uses the literal
placeholders
`__PROVIDER_SOURCE__` and `__PROVIDER_VERSION__`, plus the sensitive variables
`hubspot_access_token` and `acceptance_prefix`. It must create, reconcile, import,
drift-check, and destroy only configuration owned by that prefix.

The verification process installs the published provider from its registry. It
does not use a development override. A missing fixture, unavailable entitlement,
non-empty second plan, or failed destroy keeps release verification red.
