# CRM User Profile Live Probe

This note records the guarded CRM user-profile probe for v0.6.0. The probe used
the approved normal-Free portal and the existing non-deliverable global
identity from the v0.5.0 account-membership probe.

## Execution Boundary

- Execution completed at `2026-08-15T04:16:43Z`.
- The portal fingerprint was `sha256:c26c791399aeb246`.
- The credential came from the `terraform-provider-hubspot-probes` Keychain
  entry.
- The opening portal had one protected baseline membership.
- Membership creation sent `sendWelcomeEmail: false`.
- The probe output contained no names, emails, CRM IDs, Settings IDs, or
  credential values.

The guarded script is
[`crm-user-profile-lifecycle.zsh`](../probe/crm-user-profile-lifecycle.zsh).
Its local regression test proves Keychain fallback, welcome-disabled creation,
ordered profile writes, exact restoration, owned cleanup, and zero mutation on
a portal-fingerprint mismatch.

## Results

Creation returned one canonical Settings ID for the approved reusable global
identity. An exact Settings read verified that identity without logging it.
Paginated CRM discovery found one `hs_internal_user_id` join within the bounded
20-second materialization window.

The probe then completed these operations:

1. Read the exact CRM profile and verified both identity domains.
2. Set job title, availability, and `Australia/Melbourne` timezone.
3. Set canonical Monday-to-Friday working hours in a later PATCH.
4. Read back the exact profile and verified every managed value.
5. Restored all four opening profile values.
6. Removed only the owned Settings membership.

Exact ID and email reads proved membership absence. The final paginated
inventory matched the opening count and retained the protected baseline. The
CRM profile remains a documented non-destructive residual after membership
removal.

## Contract Consequences

The live evidence confirms the frozen v0.6.0 contract. It found no stop-level
contradiction. Profile identity remains distinct from Settings membership
identity, materialization requires a bounded join, timezone precedes working
hours, and stopping management must not delete the CRM identity.

An earlier local audit incorrectly reported that the Keychain entry was absent.
The audit ran `security` inside the filesystem sandbox, which hid the available
entry. The corrected probe restores the v0.4.0 and v0.5.0 Keychain fallback and
owned welcome-disabled lifecycle.
