# Security policy

## Reporting

Report suspected vulnerabilities privately through the repository's configured
security advisory channel. Do not publish credentials, HubSpot account details,
CRM configuration identifiers, CRM record data, or API response bodies in an issue.

## Secret handling

Provider tokens belong in `HUBSPOT_ACCESS_TOKEN` or a protected CI environment.
They must never be committed, logged, persisted in state, or included in support
artifacts. Pull-request workflows do not receive HubSpot or release secrets.
