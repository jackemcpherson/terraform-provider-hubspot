# 02 Move the Current Engine Pins

Type: task

Status: open

Label: ready-for-agent

Blocked by: None

## What to Build

"Current engine" qualification means what it says. The current OpenTofu and
Terraform pins (now 1.12.3 and 1.15.8) move to the current stable releases,
and local checks and hosted workflows cannot test different engines.

## Acceptance

- Verify the current stable OpenTofu and Terraform releases against each
  project's release page at implementation time.
- Change the current pins in the Makefile, the validation workflow, and the
  maintenance workflow together.
- Add a guard to the workflow-check script that fails when those pins
  disagree.
- Keep minimum-supported engine versions and recorded state fixtures
  unchanged.
- Pass `make check`, including its engine smoke test, and the workflow checks.
- Merge through `main` as its own pull request with green `Required`, before
  ticket 03's baseline runs.

## Comments
