# Issue tracker: Local Markdown

Issues and specs for this repo live as Markdown files in `.scratch/`.

## Conventions

- One feature per directory: `.scratch/<feature-slug>/`
- The spec is `.scratch/<feature-slug>/spec.md`
- Implementation issues are one file per ticket at `.scratch/<feature-slug>/issues/<NN>-<slug>.md`, numbered from `01`
- Triage state is recorded as a `Status:` line near the top of each issue file
- Comments and conversation history append under a `## Comments` heading

## Publishing and fetching

When a skill says "publish to the issue tracker," create a file under `.scratch/<feature-slug>/`. When a skill says "fetch the relevant ticket," read the referenced path or issue number.

## Wayfinding operations

- **Map**: `.scratch/<effort>/map.md`
- **Child ticket**: `.scratch/<effort>/issues/NN-<slug>.md`, with `Type:` (`research`, `prototype`, `grilling`, or `task`) and `Status:` (`open`, `claimed`, or `resolved`)
- **Blocking**: `Blocked by: NN, NN`; a ticket is unblocked when every listed ticket is resolved
- **Frontier**: open, unblocked, unclaimed tickets; lowest number wins
- **Claim**: set `Status: claimed` before work
- **Resolve**: append an `## Answer`, set `Status: resolved`, then add a gist and link under the map's Decisions-so-far
