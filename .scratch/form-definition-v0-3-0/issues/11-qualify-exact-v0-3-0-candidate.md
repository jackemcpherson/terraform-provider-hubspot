# 11 — Qualify the exact v0.3.0 candidate

**What to build:** Produce one clean, reviewable v0.3.0 provider/demo candidate whose complete cumulative behavior and publication contract are proven before any immutable release is created.

**Blocked by:** 01 — Align release observation with ADR 0002; 06 — Publish executable Forms documentation locally; 07 — Generalize shared-portal maintenance and cleanup; 08 — Prove the candidate in the protected Forms shard; 09 — Add Forms to the cumulative Northstar lifecycle.

**Status:** claimed

- [ ] The provider and demo version constraints, changelog, release notes, permissions guidance, lifecycle guidance, examples, generated references, and local portal all describe the exact accepted v0.3.0 surface and exclusions.
- [ ] Exact clean provider and demo commits are selected and recorded; unrelated pre-existing planning work is preserved rather than discarded or folded into the candidate.
- [ ] Formatting, linting, unit, race, hermetic dual-engine, live dual-engine, module, cumulative Northstar, documentation-generation, portal-build, workflow-policy, security, license, and release-tool checks all pass against those exact commits.
- [ ] Existing v0.1.6 state upgrades and the complete v0.2.0 CRM property schema matrix remain green under both engines.
- [ ] Provider schema inspection exposes exactly the released cumulative types and no dormant pipeline or other deferred type.
- [ ] Candidate live evidence is bound to the exact commit and proves one successful Forms lifecycle per engine with verified terminal cleanup.
- [ ] Release preflight builds the exact v0.3.0 archive/manifest/checksum set for all supported platforms and installs it through both engine mirror identities.
- [ ] Release observation tests match ADR 0002 and contain no SPDX or provenance-attestation requirement.
- [ ] The separately operated released journey is fully prepared, contract-tested, and points to the exact demo candidate needed after registry ingestion.
- [ ] Any unavailable protected token, dirty checkout, stale generated content, failed gate, cleanup residue, or evidence mismatch leaves qualification incomplete with no waiver.

## Comments

- 2026-08-03: Removed Ticket 10 as a blocking dependency because its accepted contract forbids completion until the v0.3.0 packages published by Ticket 12 exist in both registries. Candidate qualification still requires the released journey to be fully prepared and contract-tested; Ticket 10 remains claimed until its post-publication live evidence exists.
