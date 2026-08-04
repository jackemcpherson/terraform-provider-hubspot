# 06 — Prove and maintain the protected Files shard

**What to build:** Prove the exact candidate's Files configuration lifecycle against the guarded normal-Free portal under both engines and give operators fail-closed reporting and cleanup controls for interrupted owned fixtures.

**Blocked by:** 01 — Validate candidate compatibility before mutation; 04 — Deliver Files provider resources end to end.

**Status:** resolved

- [x] A `files_configuration` capability shard declares the exact `files` scope and uses a protected least-privilege credential separate from cumulative Northstar.
- [x] Candidate compatibility preflight passes before the shard can mutate live HubSpot configuration.
- [x] Portal identity guard, unique owned prefix, exact candidate commit, cleanup ledger, and shared account-wide lock are mandatory with no waiver.
- [x] OpenTofu and Terraform each run an independently prefixed root-folder, child-folder, and Managed file lifecycle on the normal Free portal.
- [x] Each lifecycle proves generated identity, canonical read, empty plan, folder asynchronous rename/move, descendant path refresh, file PATCH, access transitions, in-place PUT, content drift, exact-ID import, external disappearance/recreation, safe destroy, and new-ID name reuse.
- [x] Live create proves target-name and duplicate rejection without adopting `RETURN_EXISTING` or accepting server name normalization.
- [x] Candidate runs do not repeat excluded destructive discovery for URL overwrite, TTL, signed URLs, cascade deletion, hidden access, or GDPR purge.
- [x] Evidence records exact candidate commit, engine, API and scope families, hashed identities and portal fingerprint, content-change proof, timestamps, active cleanup counts, and final state without raw IDs, names, paths, URLs, bytes, or credentials.
- [x] Cleanup deletes managed files before folders and folders leaf-first, verifies direct active absence for every owned ID, and proves zero active prefix-owned files and folders.
- [x] HubSpot-managed Trash retention is reported as expected cleanup rather than deletion failure or physical purge.
- [x] Scheduled maintenance reports stale active owned Files configuration without mutating it automatically.
- [x] Manual cleanup requires an exact owned prefix, Files-specific literal confirmation, protected credential boundary, exact-ID operations, and final zero-active verification.
- [x] Wrong scope, unavailable token, wrong portal, malformed prefix, lock collision, unowned result, failed task, incomplete deletion, or cleanup residue fails closed.
- [x] Existing property-schema, Form definition, maintenance, workflow-policy, and shared-lock behavior remains green and serialized against the same portal boundary.

## Answer

Added the protected `files_configuration` shard, dual-engine Files lifecycle proof, sanitized evidence, and fail-closed scheduled reporting/manual cleanup controls. Candidate compatibility and all immutable guards run before live mutation; cleanup preflights the complete owned set before deleting files first and folders leaf-first, then verifies direct absence and zero active prefix-owned configuration.
