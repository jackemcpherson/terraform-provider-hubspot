# Repository Instructions

## Agent Skills

### Issue tracker

Issues use local Markdown under `.scratch/`. See `docs/agents/issue-tracker.md`.

### Triage labels

Default canonical triage labels are used. See `docs/agents/triage-labels.md`.

### Domain docs

Single-context layout: root `CONTEXT.md` and `docs/adr/`. See
`docs/agents/domain.md`.

### Infrastructure design guides

Before provider design, implementation, examples, or CI work, read both
infrastructure guides in full. Translate their principles to provider
development; do not apply HCL module layout literally. See
`docs/agents/infrastructure-guides.md`.

### Release slices

Before a new configuration-surface slice or release-preparation work, follow
the baseline, stabilisation, qualification, and publication sequence in
`docs/agents/release-slices.md`.
