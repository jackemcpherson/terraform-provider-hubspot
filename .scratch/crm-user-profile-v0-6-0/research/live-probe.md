# CRM User Profile Live Probe

Status: blocked before execution on 15 August 2026.

The local environment did not contain `HUBSPOT_ACCESS_TOKEN`,
`HUBSPOT_PROBE_EXPECTED_FINGERPRINT`, or the protected Northstar Settings user
ID. The dedicated probe Keychain entry was also absent. No HubSpot request or
mutation was sent.

Run `../probe/crm-user-profile-lifecycle.zsh` only with the approved environment
credential, portal fingerprint, and protected activated Settings identity. The
probe never prints credentials, email, names, CRM IDs, or Settings IDs. It
restores the exact opening profile properties on every guarded exit.

Prior live evidence from 2 August 2026 remains in
`.scratch/hubspot-free-configuration-coverage/research/16-user-configuration-live-lifecycle.md`.
That evidence confirms the distinct identities, unique join in the test portal,
profile property lifecycle, timezone prerequisite, and activation-dependent
materialization. It does not replace the required fresh release probe.
