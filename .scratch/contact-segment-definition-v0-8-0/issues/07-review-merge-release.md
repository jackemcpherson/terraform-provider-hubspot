# 07 Review, Merge, and Release

Type: task

Status: open

Blocked by: 01, 02, 03, 04, 05, 06

## Acceptance

- Pass `make check`, `make test-hermetic`, `make test-race`,
  `make fuzz-seeds`, `make check-security`, and `make check-release`.
- Pass the complete normalized demo check under current OpenTofu and
  Terraform.
- Clear fixed-point standards, specification, test-quality, Actions, and
  release-readiness reviews.
- Require green `Required` and protected maintenance on exact final provider
  and demo `main` commits.
- Publish one signed immutable v0.8.0 release, verify its complete asset set,
  and record Terraform and OpenTofu Registry ingestion separately.

## Comments

- 2026-08-18: The standalone release-slice discipline change merged in
  provider PR 96 as `c9aabc0`.
- 2026-08-18: Demo PR 30 normalized cumulative history into demo `main` as
  `2ef32f7`. The original dirty demo checkout remained unchanged. Provider PR
  97 pinned that exact merge as `f6fc450`.
- 2026-08-18: Initial baseline run 32090418370 passed. Run 32090906501 failed
  during Terraform File-folder refresh because HubSpot returned a stale
  terminal task payload after applying the async rename.
- 2026-08-18: Provider PR 98 added deterministic dual-engine regression
  coverage and changed stale terminal results to require exact-ID convergence.
  It merged as `5aaf330`. Local `make check`, complete hermetic acceptance, and
  race checks passed.
- 2026-08-18: Restarted baseline runs 32092526989 and 32093110565 passed
  consecutively on provider `5aaf330` and demo `2ef32f7`, including terminal
  cleanup.
- 2026-08-18: Provider and demo repositories are public and every workflow
  uses the standard `ubuntu-24.04` hosted runner. [GitHub documents standard
  hosted runners](https://docs.github.com/en/actions/reference/runners/github-hosted-runners)
  as free and unlimited for public repositories, so hosted runner minute
  budget is not a release constraint. The authenticated billing endpoint was
  not used because the CLI token deliberately lacks `user` scope.
- 2026-08-18: Probe prerequisite PR 99 merged as `686c176`. Protected run
  32095048270 stopped before implementation and demo execution when the guide's
  `IS_NOT_KNOWN` filter operator produced `ListError.ENUM_CONVERSION`. Deferred
  cleanup verified the exact manual-definition tombstone.
- 2026-08-18: Probe correction PR 100 merged as `f01fcc5`. Protected run
  32095515566 proved `IS_UNKNOWN` and dynamic text/presence round-trip, then a
  select predicate rejected `MULTISTRING` with
  `ListError.INVALID_OPERATION_FOR_PROPERTY_TYPE`. Deferred cleanup verified
  both created tombstones.
- 2026-08-18: Operation-matrix PR 101 merged as `874d0ea`. Protected run
  32096106495 proved `MULTISTRING` text=true/select=false, `STRING`
  text=true/select=false, and `ENUMERATION` text=false/select=true. Every
  accepted candidate was deleted and verified by exact-ID tombstone read. The
  release stopped before implementation because the agreed public filter and
  lists-only scope contract cannot select the required operation type.
- 2026-08-18: The user selected author-declared property kind after a design
  review. ADR 0004 retains v0.8.0 and Lists-only permissions, specifies
  conditional `property_kind`, and freezes deterministic text, select, and
  presence wire mappings. The full protected contract probe is the next gate.
- 2026-08-18: Provider PR 103 froze that decision and merged as `7eb7e36`.
  Provider PR 104 added the amended probe to maintenance and merged as
  `1ed0f2d`. Protected run 32101261970 proved all three Free variants,
  text/select/presence round-trip, dynamic replacement, a readable exact-ID
  tombstone, and same-ID restore from list version 2 to 3. It then failed
  because the synthetic maximum-integer unknown ID returned a different 4xx
  classification than the probe required. Exact-ID cleanup completed.

## Failure Ledger

| Phase | Run | Provider | Demo | Cause | Fix |
| --- | --- | --- | --- | --- | --- |
| Terraform refresh | 32090906501 | `f6fc450` | `2ef32f7` | Stale terminal folder task result | Provider PR 98, `5aaf330` |
| Contact segment probe | 32095048270 | `686c176` | `2ef32f7` | Guide operator `IS_NOT_KNOWN` rejected | Use dated `IS_UNKNOWN` wire enum |
| Contact segment probe | 32095515566 | `f01fcc5` | `2ef32f7` | Select property rejected `MULTISTRING` | Test universal operation matrix |
| Contact segment probe | 32096106495 | `874d0ea` | `2ef32f7` | No universal text/select operation type | User contract decision required |
| Contact segment probe | 32101261970 | `1ed0f2d` | `2ef32f7` | Unknown-ID sentinel had a different 4xx class | Record any authoritative 4xx outcome |
