# ADR 0003: Keep v0.x Delivery Out of the Way

- Status: Accepted
- Date: 2026-08-14

During v0.x development, delivery must provide fast feedback and immutable
releases without making exhaustive qualification part of each change. Use one
required pull-request job, one manual publication job, one weekly or manual
maintenance job, and one manual archival job. Run the same fast check locally
and in pull requests. Keep live acceptance, slower security analysis, and broad
compatibility evidence outside the merge and publication paths.

The publication job releases the current passing `main` commit through the
`release` environment. The environment isolates signing credentials but does
not require human approval during v0.x. Publication keeps signed tags and
published releases immutable. It can retry only when the existing signed tag
identifies the same commit and no GitHub release exists.

Remove candidate qualification, historical engine matrices, registry polling,
released-provider journeys, custom evidence bundles, workflow-topology locks,
CodeQL, Scorecard, and version-specific release checks. Keep direct checks for
artifact naming, checksums, signatures, tag immutability, current Terraform and
OpenTofu compatibility, workflow safety, dependency vulnerabilities, and one
cumulative live HubSpot journey. Reconsider hosted security reporting and human
release approval during v1.0 readiness.
