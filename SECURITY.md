# Security Policy

This policy explains how to report vulnerabilities and protect release secrets.

## Reporting

Report suspected vulnerabilities privately through the repository's configured
security advisory channel. Do not publish credentials, HubSpot account details,
CRM configuration identifiers, CRM record data, or API response bodies in an issue.

## Secret Handling

Provider tokens belong in `HUBSPOT_ACCESS_TOKEN` or a protected CI environment.
Do not commit, log, or persist a token in state. Do not include a token in support
artefacts. Pull-request workflows do not receive HubSpot or release secrets.

Enable private vulnerability reporting, dependency graph/review, secret scanning,
and push protection in the repository settings. Security updates require review
but do not wait for the routine dependency cooldown. Workflow, release, manifest,
and dependency files require CODEOWNER review.

Release builds run without HubSpot or signing credentials. The protected signing
job receives only verified artefacts and GPG material. Published tags and assets
are immutable. Publish a new patch release to correct an artefact defect.

Repository settings must require GitHub-hosted runners for protected workflows.
Each capability and release environment requires approval. Require signed
commits, `CI / Required`, security checks, and immutable releases. Do not store
HubSpot credentials or GPG material in repository-level secrets.
