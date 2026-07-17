# Deferred paid and Enterprise capabilities baseline

## Preserved source

- Annotated tag: `deferred/v0.2-paid-enterprise-baseline`
- Tag object: `e7b1ace66cba5cf5e14463487674ff63c97da455`
- Preserved commit: `f36e2b251ce2b0e93cd6b85bf4d9c9941701daa1`
- Remote: `origin` (`https://github.com/jackemcpherson/terraform-provider-hubspot.git`)

The tag is a preservation point, not a v0.1 release candidate. Do not move or
replace it. The protected-release-controls work must retain this tag and prevent
force-updates to deferred release references.

## Deferred public surface

The preserved commit registers these resources in addition to the Free-only
surface:

- `hubspot_pipeline` — deal, ticket, and custom-object pipeline variants with
  exclusively owned nested stages.
- `hubspot_custom_object_schema` — custom schema properties, roles, associations,
  expected external properties, and deletion protection.
- `sensitive` and `highly_sensitive` variants of `hubspot_property`, plus
  sensitivity-filtered property-definition discovery.

## Preserved implementation areas

- Provider registration and resource behavior: `internal/provider/provider.go`,
  `internal/provider/pipeline_resource.go`, and
  `internal/provider/custom_schema_resource.go`.
- Typed API support: pipeline, schema, and association contracts in
  `internal/hubspot/`.
- Offline and live acceptance: pipeline, schema, sensitive-property, janitor, and
  released-provider fixtures under `internal/acceptance/`.
- Capability workflows: paid/Enterprise shard manifests in
  `.github/workflows/acceptance.yml`, `acceptance-cleanup.yml`, and
  `verify-release.yml`.
- Consumer material: pipeline/custom-schema examples and their generated docs,
  lifecycle/import/permission/troubleshooting pages, and release verification
  documentation.

## Reintroduction rule

Treat the tag as a read-only source baseline. A later map must define the target
release contract and selectively port the required areas into a new branch; it
must not re-register all deferred resources merely by merging the tag. Before a
reintroduced capability becomes public, refresh its HubSpot contract, restore its
documentation and acceptance shard, and qualify it against the account tier it
advertises.
