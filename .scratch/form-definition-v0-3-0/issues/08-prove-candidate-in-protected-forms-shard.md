# 08 — Prove the candidate in the protected Forms shard

**What to build:** Prove the exact candidate's Form definition behavior against a guarded normal-Free portal with one bounded fixture per engine and no active residue.

**Blocked by:** 04 — Make Form definition identity recovery safe; 07 — Generalize shared-portal maintenance and cleanup.

**Status:** resolved

- [x] The live shard refuses mutation without its enabled acceptance gate, exact `forms` credential, expected portal identity, unique owned prefix, cleanup ledger, and shared portal lock.
- [x] OpenTofu creates one supported form, verifies generated identity and canonical state, reaches an empty plan, updates presentation, detects and repairs supported drift, imports by exact ID, handles external archive by recreation, and archives the final identity.
- [x] Terraform independently performs the same complete lifecycle with one separate uniquely prefixed form.
- [x] No live candidate run repeats duplicate-name, archived-name-reuse, or destructive failure discovery fixtures already pinned by guarded probe and hermetic evidence.
- [x] Each engine verifies zero active prefix-owned forms and the exact terminal tombstone identities before releasing the portal lock.
- [x] Evidence records exact candidate commit, engine, API family, scope family, hashed portal fingerprint, timestamp, generated-identity hash, and cleanup result without raw IDs, names, payloads, or credentials.
- [x] A missing scope, unavailable token, wrong portal, registry-independent provider failure, unsupported normalization, or incomplete cleanup is a blocking failure with no waiver.
- [x] Existing live property-schema acceptance remains green and serialized against the same account boundary.

## Answer

Added a protected `form_definitions` candidate suite with one OpenTofu lifecycle and one independently prefixed Terraform lifecycle. Each creates canonical state, proves a no-op plan, applies the complete supported presentation update, detects and repairs supported out-of-band drift, imports by exact generated ID, observes external archive as absence, recreates under a new identity, archives the final identity, and proves zero active owned Forms plus both exact retained tombstones. Duplicate-name, archived-name-reuse, unsupported-structure, and ambiguous destructive matrices remain in the hermetic suite.

The shared runner still binds an exact clean commit to one locally built provider binary, requires the `forms` capability preflight and expected portal guard, creates a cleanup ledger, and acquires the account-wide lock. Successful engine runs emit separate hashed-only evidence with candidate commit, engine, API/scope family, portal fingerprint, generated and terminal identity hashes, UTC timestamp, and cleanup result; stale or missing engine evidence fails closed. The workflow uploads the bounded Forms evidence set, while shell policy tests prove complete/missing evidence routing and unsupported-shard rejection. `make check`, both-engine hermetic Forms tests, live-safe syntax/evidence tests, actionlint, and zizmor pass. The credentialed protected run remains mandatory during candidate qualification and has no local waiver.
