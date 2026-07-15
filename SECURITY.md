# Security policy

## Reporting

Report suspected vulnerabilities privately through the repository's configured
security advisory channel. Do not publish credentials, HubSpot account details,
CRM configuration identifiers, CRM record data, or API response bodies in an issue.

## Secret handling

Provider tokens belong in `HUBSPOT_ACCESS_TOKEN` or a protected CI environment.
They must never be committed, logged, persisted in state, or included in support
artifacts. Pull-request workflows do not receive HubSpot or release secrets.

Enable private vulnerability reporting, dependency graph/review, secret scanning,
and push protection in the repository settings. Security updates require review
but do not wait for the routine dependency cooldown. Workflow, release, manifest,
and dependency files require CODEOWNER review.

Release builds run without HubSpot or signing credentials. The protected signing
job receives only verified artifacts and GPG material. Published tags and assets
are immutable; an artifact defect is corrected with a new patch release.

Repository settings must require GitHub-hosted runners for protected workflows,
approval on every capability and release environment, signed commits, required
`CI / Required` and security checks, and GitHub immutable releases. Do not place
HubSpot credentials or GPG material in repository-level secrets.
