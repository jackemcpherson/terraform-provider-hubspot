# 05 Discover the Repository Root on Case-insensitive Filesystems

Type: task

Status: open

Label: ready-for-agent

Blocked by: None

## What to Build

Acceptance tests find the repository root and build the provider whatever the
case of the working-directory path.

## Acceptance

- Resolve the working directory to its canonical on-disk form before walking
  up to the module root.
- Never return a path that the Go command treats as outside the main module.
- Add one regression test at the harness entry point that simulates a
  case-variant working directory and asserts that the resolved root builds the
  provider.
- Pass the existing harness and property harness tests from a case-variant
  working directory.
- Merge through `main` before v0.8.0 source freeze.

## Comments
