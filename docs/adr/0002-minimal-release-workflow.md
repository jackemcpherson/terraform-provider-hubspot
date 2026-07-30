# ADR 0002: Keep provider release publication minimal

- Status: Accepted
- Date: 2026-07-30

## Context

The consolidated provider lifecycle mixed scheduled portal maintenance with a
13-job release state machine. Publication rebuilt artifacts, passed them between
jobs, classified recovery states, created attestations, polled two registries,
installed from both registries, and ran live provider journeys. Those controls
made routine release operation difficult to understand and coupled unrelated
portal maintenance to publication.

Terraform Registry and OpenTofu Registry both ingest the same signed assets from
a GitHub release. They do not require separate uploads from this repository. The
essential publication transaction is therefore a versioned tag plus one complete,
correctly named and signed GitHub release asset set.

## Decision

Use a single manually dispatched `release.yml` job. It accepts a v-prefixed
SemVer, binds the dispatch to the current `main` commit and its successful
`Required` check, enters the protected `release` environment, creates a signed
tag, and invokes pinned GoReleaser once. GoReleaser builds every supported
platform archive, signs the Registry checksum inventory, publishes the versioned
Registry manifest, and creates the GitHub release consumed by both registries.

The job may resume only the narrow failure case where its signed tag exists at
the same commit but the GitHub release does not. It never moves a tag or replaces
an existing release.

Scheduled source acceptance and read-only janitor reporting live in
`provider-maintenance.yml`. Manual prefix archival remains isolated in
`archive-crm-configuration.yml`. Pull-request and main validation remain in
`validate-provider.yml`.

SPDX generation, provenance attestation, duplicate builds, draft-state recovery,
registry polling, registry install matrices, and released-provider live journeys
are not part of the publication transaction.

## Consequences

A maintainer can trace the complete release in one job: validate, tag, build,
sign, and publish. The protected environment still contains the private signing
key, release attempts remain serialized, third-party actions remain pinned, and
the exact archive/checksum/manifest contract remains covered by the ordinary
quality pre-flight.

Both registries publish asynchronously from the GitHub release. Ingestion delays
are observed in the registry interfaces rather than held open inside the release
workflow. A bad published asset requires a new patch version; existing tags and
releases remain immutable.
