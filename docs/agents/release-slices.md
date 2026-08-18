# Release Slice Discipline

Use this sequence for every configuration surface that will ship in a provider
release. It keeps cumulative failures separate from the new surface and binds
live evidence to exact provider and demo commits.

## 1. Establish a Green Baseline

Before research or implementation:

1. Record the exact `main` commits for the provider and cumulative demo.
2. Confirm `gh auth status` succeeds for the intended GitHub account.
3. Confirm enough GitHub Actions budget remains for the planned hosted runs.
4. Confirm the protected environment still has the required variables and
   surface scopes without reading or printing secret values.
5. Run `Provider maintenance` twice on the unchanged provider and demo commits.

The baseline is complete only when both consecutive runs succeed against the
same commits. Record their URLs in the slice's append-only release ticket while
the slice is still moving.

A baseline failure is inherited maintenance work. Resolve it in a separate
ticket and restart the two-run baseline before adding the new surface.

## 2. Freeze the Slice

Create `.scratch/<surface>-v<version>/` with research, `spec.md`, `map.md`, and
append-only tickets. The frozen specification must define identity, ownership,
refresh, drift, import, update, destroy, permissions, and cleanup contracts.

Include these slice-specific proof obligations:

- A narrow protected live probe that tests uncertain API assumptions.
- Fake behavior for every asynchronous or eventually consistent observation
  that the surface can encounter.
- A stable-keyed demo module and the cumulative demo change.
- Documentation, release notes, and the exact demo-pin update.

Run the narrow live probe as soon as the smallest safe vertical slice exists.
Reserve the complete cumulative journey for final qualification.

## 3. Model Remote Timing Before the Happy Path Expands

Extend the behavioral fake when the API can expose any of these states:

- A terminal asynchronous task result that is newer than a stale object read.
- Stale exact-ID reads or searches with explicit revision or timestamp order.
- Delayed path or parent propagation.
- A projection that disappears during teardown.
- A local-only configuration change that must not issue a remote write.
- Success, rejection, timeout, and ambiguous terminal outcomes.

Each modeled state needs a deterministic test that proves the provider's state
transition and request count. The fake is complete when the important failure
can be reproduced on demand without a live account.

## 4. Work in Focused Loops

For each vertical slice, run the smallest test that proves the new behavior,
then its package tests. Run affected static, hermetic, and workflow checks after
each fix.

Run the complete local release suite once after the candidate source is frozen:

```sh
make check
make test-hermetic
make test-race
make fuzz-seeds
make check-security
make check-release
```

Restart the complete suite only when a later source change can affect a gate.
Update post-freeze evidence outside Git so recording a hosted result does not
move the candidate commit.

## 5. Enter Stabilisation Deliberately

Treat two unrelated cumulative maintenance failures during one slice as a
stabilisation signal. Pause feature expansion, create separate append-only
tickets for the failures, repair the cumulative lifecycle, and re-establish the
two-run green baseline.

Keep a failure ledger in the release ticket with the failing phase, run URL,
provider commit, demo commit, cause, and fix. Never include credentials, portal
identities, or raw remote object IDs.

## 6. Make Hosted Maintenance Observable

Maintenance output must distinguish these boundaries:

1. Surface contract probe.
2. OpenTofu cumulative lifecycle.
3. Terraform cumulative lifecycle.
4. Terminal cleanup and absence verification.

When changing maintenance automation, expose these as named GitHub Actions
steps or equally clear grouped log sections. A failure must identify the engine
and lifecycle phase without reading the complete raw log.

Observe long runs with bounded, structured queries such as:

```sh
gh run view <run-id> \
  --json status,conclusion,headSha,jobs,url \
  --jq '{status, conclusion, headSha, jobs, url}'
```

Poll at a moderate interval and report only state changes. Use raw job logs for
a failed step, not as the primary progress display.

## 7. Qualify Exact Commits

Merge the demo change first. Pin its exact merge commit in provider
maintenance, then freeze the provider changelog, release notes, scratch
tickets, and generated documentation before choosing the final provider
commit. After that freeze, hosted run URLs and mutable status belong in a
GitHub pull-request comment or another operator record outside Git.

The intended steady-state run budget is:

| Evidence | Minimum successful runs |
| --- | ---: |
| Unchanged provider and demo baseline | 2 |
| Protected release candidate | 1 |
| Exact merged `main` commit | 1 |
| Publication | 1 |

Extra runs need a reason in the failure ledger.

### Protected Candidate Prerequisite

ADR 0003 and the current workflow permit protected maintenance only from
`main`. Before v0.8.0 relies on candidate evidence, add a dedicated
maintainer-dispatched candidate path and an ADR that explicitly supersedes the
candidate-qualification part of ADR 0003.

The candidate path must:

- Accept only the exact head of a protected `release/v<version>` branch from
  this repository.
- Require the candidate commit to pass `Required` and the fixed-point reviews
  before it can enter the protected environment.
- Reject pull-request forks, arbitrary refs, dirty source, and moving demo
  references.
- Use the existing protected environment and non-cancelling portal concurrency
  group.
- Pin the exact merged demo commit and expose no protected credentials to
  unreviewed code.
- Run the same contract probe and cumulative dual-engine journey used on
  `main`.
- Have workflow-policy tests for every ref, permission, and provenance guard.

Until that prerequisite is implemented and reviewed, qualify only the exact
merged `main` commit. Do not weaken the current `main`-only guard or claim
candidate evidence.

## 8. Publish Once

After the candidate run, merge without changing the qualified content. Require
a green `Required` check and one green manual maintenance run on the exact
`main` commit. Then follow `docs/release-operations.md` for signed publication
and asset verification.

A slice is complete when the signed immutable release targets the reviewed
`main` commit, its GitHub assets verify, cleanup is terminal, and registry
ingestion status is recorded separately. Registry ingestion can remain pending
because it is asynchronous.
