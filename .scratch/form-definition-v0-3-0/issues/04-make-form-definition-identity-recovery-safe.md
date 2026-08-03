# 04 — Make Form definition identity recovery safe

**What to build:** Make generated-ID import, ambiguous writes, external archival, disappearance, and repeated destroy recover safely without replaying creates or guessing ownership from duplicate names.

**Blocked by:** 03 — Reconcile supported presentation and reject unsafe drift.

**Status:** resolved

- [x] Import accepts only one exact generated form ID and populates the complete canonical supported active state.
- [x] Import rejects malformed identifiers, URLs, names, composite identifiers, Archived form definitions, non-`hubspot` forms, and unsupported active definition shapes without remote mutation.
- [x] Exact duplicate active names remain legal for direct resources and never participate in identity, import, or recovery.
- [x] An ambiguous create is sent exactly once and is never retried or recovered by name or payload similarity.
- [x] When an ambiguous or unverifiable create has returned an ID, that identity remains recoverable through state or exact import; when no ID is known, the diagnostic directs explicit operator identification and import without guessing.
- [x] An ambiguous update succeeds with a warning only when exact-ID read-back matches the complete plan; otherwise prior state is retained with an error.
- [x] External archival is verified through the archived view, removes the absent active identity from state, and plans creation of a new generated ID.
- [x] Complete absence from both active and archived views is treated as missing state and can be recreated.
- [x] Destroy handles active, already archived, permanently absent, successful, and ambiguous archive outcomes idempotently, removing state only after exact active-absence and terminal-state evidence where available.
- [x] OpenTofu and Terraform hermetic lifecycles cover import round-trip, external archive and recreation, disappearance, ambiguous create/update/archive, and final zero-active cleanup.

## Answer

Implemented exact generated-UUID import with canonical active-state hydration and fail-closed rejection of malformed, archived, non-`hubspot`, and unsupported definitions. Create responses now preserve any decoded generated ID before reporting an error, retain that ID for exact recovery, never replay POST, and direct no-ID outcomes to operator identification and exact import without name lookup. Refresh classifies the same ID across active and archived views; destroy uses that classifier plus bounded terminal verification so active, archived, absent, successful, applied-ambiguous, and unapplied-ambiguous outcomes preserve ownership safely. The behavioral fake now provides canonical generated IDs, duplicate-name fixtures, mutation counters, disappearance, structural variants, and deterministic one-shot create/update/archive faults. OpenTofu and Terraform hermetic lifecycles prove import round-trip and rejection, external archive recreation, complete disappearance, exact-once ambiguous create, update/archive recovery, repeated destroy, and final zero-active cleanup. `make check-go` and `make test-hermetic` pass.
