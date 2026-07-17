# Reduce the provider and documentation to the Free-only surface

Type: task
Status: resolved
Blocked by: 01, 02

## Question

How should the provider registration, schemas, generated reference pages,
examples, imports, changelog, permissions guidance, tests, and release metadata be
changed so v0.1.0 exposes only property groups, ordinary non-sensitive properties,
and property-definition discovery, while deferred code cannot become an accidental
public support contract?

## Answer

v0.1 now registers only `hubspot_property_group` and `hubspot_property`; the two
property-definition data sources remain registered. The public sensitivity selector
accepts only `non_sensitive`, rejecting `sensitive` and `highly_sensitive` during
planning.

Generated pipeline/custom-schema references, examples, imports, lifecycle guides,
and consumer links were removed. Paid/Enterprise acceptance and released-artifact
workflow matrices now permit only `free_properties`; deferred test fixtures are
kept behind the `deferred` Go build tag, alongside the source preserved at
`deferred/v0.2-paid-enterprise-baseline`.

`make check` passes, including the deterministic Go, generated-doc, workflow, and
OpenTofu/Terraform example gates.
