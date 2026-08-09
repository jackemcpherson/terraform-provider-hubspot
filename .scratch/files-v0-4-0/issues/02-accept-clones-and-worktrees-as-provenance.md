# 02 — Accept clones and worktrees as exact provenance

**What to build:** Let protected provider and demo evidence originate from either a standalone Git clone or a linked Git worktree while still binding every result to the exact expected clean commit.

**Blocked by:** None — can start immediately.

**Status:** resolved

- [x] Shared provenance validation uses Git plumbing rather than requiring `.git` to have a particular filesystem type.
- [x] A standalone clone with a `.git` directory passes when it is the expected checkout root, exact commit, and clean worktree.
- [x] A linked worktree with a `.git` file passes under the same exact-commit and cleanliness requirements.
- [x] Validation proves the supplied path is the checkout root of a non-bare repository with an attached worktree.
- [x] The expected commit must be a full 40-character commit and must equal the checkout's exact HEAD.
- [x] Both tracked and untracked changes make provenance fail closed.
- [x] A non-repository, bare repository, subdirectory passed as root, wrong commit, malformed Git indirection, or failed plumbing command is rejected clearly.
- [x] Protected Northstar and other existing exact-checkout call sites use the shared contract without weakening their current checks.
- [x] Portal provenance can reuse the same contract for either checkout form.
- [x] Hermetic tests construct real standalone-clone and linked-worktree fixtures and cover every required success and failure state.

## Answer

Implemented `internal/provenance` as the shared Git-plumbing contract and exposed
it to protected shell journeys through `cmd/validate-checkout`.
Provider acceptance, candidate Northstar, release preflight, released-provider
evidence, and generated portal provenance now bind clean checkout roots to exact
40-character commits without inspecting whether `.git` is a directory or file.

Hermetic tests cover standalone clones, linked worktrees, wrong or malformed
commits, bare and non-repositories, subdirectory roots, tracked and untracked
changes, malformed indirection, and failed plumbing. `make check` passes.
