# 03 — Revalidate stale registry version responses

**What to build:** Make post-publication observation distinguish a published source release from registry ingestion and recover safely from a stale public registry edge without declaring completion from source metadata or a cache-bypassing response alone.

**Blocked by:** None — can start immediately.

**Status:** resolved

- [x] Immutable release and source discovery is recorded as diagnostic prerequisite evidence and never counts as registry ingestion.
- [x] A valid ordinary public versions response containing the requested version passes immediately.
- [x] A valid ordinary response missing the version triggers a bounded revalidation carrying `Cache-Control: no-cache` and compatible `Pragma: no-cache` semantics.
- [x] A successful revalidation still requires a later ordinary public versions response to advertise the version before ingestion passes.
- [x] Observation remains bounded to at most 12 attempts with ten seconds between attempts and a ten-second request timeout.
- [x] Persistent stale responses, non-success HTTP status, timeout, malformed JSON, a missing versions collection, and invalid or duplicate version records leave ingestion blocked.
- [x] Failure diagnostics identify the registry host and safe response class without printing response bodies that may contain unexpected data.
- [x] Hermetic HTTP tests cover fresh, stale-then-revalidated-then-fresh, revalidation-fresh-but-ordinary-stale, persistent stale, malformed ordinary, malformed revalidation, and source-only discovery.
- [x] Both Terraform Registry and OpenTofu Registry use the same completion contract.
- [x] Registry observation remains outside the protected publication transaction required by ADR 0002.

## Answer

Post-publication observation now records the verified immutable GitHub release as
source prerequisite evidence, then requires both public Registries' ordinary
versions endpoints to advertise the requested release. Valid stale responses get
one no-cache revalidation per bounded attempt, but only a later ordinary response
can complete ingestion.

Hermetic process-level HTTP tests cover fresh and stale recovery, revalidation
without ordinary confirmation, persistent stale data, malformed responses,
HTTP failure, timeout, missing and invalid collections, duplicate versions, safe
diagnostics, and source-only discovery. Registry polling remains separate from
the protected publication transaction described by ADR 0002.
