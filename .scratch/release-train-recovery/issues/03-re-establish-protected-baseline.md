# 03 Re-establish the Protected Baseline

Type: task

Status: open

Label: ready-for-agent

Blocked by: 01, 02

## What to Build

v0.8.0 qualifies against a proven baseline. Two consecutive green protected
maintenance runs on the repaired `main` replace the stale 2026-08-18 baseline,
and release history explains the gap.

## Acceptance

- Confirm the protected environment variables and scopes without reading
  secret values, as release-slice discipline step 1 requires.
- Pass two consecutive protected maintenance runs on the same unchanged
  provider commit and demo `2ef32f7`.
- Add a failure ledger entry for the 2026-10-12, 2026-10-19, and 2026-10-26
  scheduled failures, with phase, run, commits, cause, and fix.
- Record the new baseline provider and demo commits and run URLs.

## Comments
