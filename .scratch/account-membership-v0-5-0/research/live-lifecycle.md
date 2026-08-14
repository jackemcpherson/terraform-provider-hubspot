# Account Membership Live Lifecycle

This note records the guarded account-membership probe for v0.5.0. The probe
used the approved normal-Free portal and an existing non-deliverable identity.

## Execution Boundary

- Execution completed at `2026-08-14T22:42:55Z`.
- The portal fingerprint was `sha256:c26c791399aeb246`.
- The portal had one protected baseline membership.
- The probe reused the approved `example.com` global identity.
- Every create sent `sendWelcomeEmail: false`.
- The probe did not call a CRM user-profile endpoint.
- Output contained no names, emails, user IDs, or credential values.

The guarded script is
[`account-membership-lifecycle.zsh`](../probe/account-membership-lifecycle.zsh).

## Results

Creation returned `201`, a canonical Settings user ID, and
`superAdmin: false`. Exact reads by ID and by email returned the same identity.
Later reads returned `sendWelcomeEmail: false`.

The membership had no role or team assignment. An exact name no-op returned
`400 USER_NOT_ON_ANY_HUBS`. The probe did not retry that name update.

Deletion returned `204`. Exact ID and email reads then returned `404`.
Same-email reprovision returned `201` and reused the Settings user ID. The
second guarded deletion restored the opening membership count.

The final checks found one protected baseline membership and no fixture
membership. The probe left no account-scoped residual.

## Reconciliation Observation

An earlier run at `2026-08-14T22:41:23Z` completed the first deletion but got
`AppUserErrors.NO_SEAT_AVAILABLE_FOR_ROLE_COMBINATION` on immediate reuse.
The cleanup trap removed the fixture. A read-only inventory then confirmed one
protected baseline membership and no fixture membership.

The probe now retries only that exact known rejection. Before each retry, it
requires exact-email `404` and collection absence. The successful rerun needed
one reuse attempt. This evidence shows that seat reconciliation can lag even
after direct absence, but the lag did not recur on the final run.

## Contract Consequences

- Treat the Settings ID as canonical state and import identity.
- Keep email lookup explicit through the `email:` import prefix.
- Keep welcome delivery as a required creation-only choice.
- Do not retry `USER_NOT_ON_ANY_HUBS`.
- Require empty role and team assignments before a name PUT.
- Verify deletion by exact ID, exact email, and eventual collection absence.
- Preserve the local removal guard and Super Admin refusal.
- Document that account removal does not delete the global HubSpot identity.
