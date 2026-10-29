# 05 Discover the Repository Root on Case-insensitive Filesystems

Type: task

Status: resolved

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

## Answer

Cause: `repositoryRoot` walked up from `os.Getwd()`, which returns the
working-directory spelling the shell used (`~/projects/...`). The harness then
ran `go build <root>` with that spelling. The Go command resolves its main
module from the on-disk spelling (`~/Projects/...`), because the inherited
`PWD` names the test package directory, not the build directory. It then
rejected the case-variant build path as outside the main module.

Fix: `repositoryRoot` resolves symbolic links in the working directory, then
rebuilds it from the entry names stored on disk before walking up to `go.mod`. Each component keeps an exact
match, or takes the case-insensitive sibling that `os.SameFile` confirms is
the same directory. The harness now builds from the spelling the Go command
uses.

Test: `TestRunBuildsProviderFromCaseVariantWorkingDirectory` in
`internal/acceptance/harness_test.go` changes into a case-swapped spelling of
the test directory, then runs the OpenTofu property group lifecycle through
`acceptance.Run`. It skips on case-sensitive filesystems. Before the fix it
failed with `outside main module or its selected dependencies`. After the fix
it passes.

Gates, run with Go 1.26.6 and the pinned OpenTofu 1.12.3 and Terraform 1.15.8:

- Harness and property harness tests (`go test ./internal/acceptance -run
  TestRun`) pass from `~/projects/...`.
- `make check` passes.
- `make test-hermetic` passes from `~/projects/...`, with `HUBSPOT_DEMO_REPO`
  set because the worktree is not a sibling of the demo checkout.
