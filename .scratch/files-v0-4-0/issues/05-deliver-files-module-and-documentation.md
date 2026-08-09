# 05 — Deliver the Files module and executable documentation

**What to build:** Give OpenTofu consumers a stable-keyed, composable Files configuration module and generated local documentation that accurately exposes the shipped provider contract, hierarchy, byte ownership, and teardown behavior.

**Blocked by:** 02 — Accept clones and worktrees as exact provenance; 04 — Deliver Files provider resources end to end.

**Status:** resolved

- [x] A local module named `files-configuration` supports OpenTofu and Terraform `>= 1.8, < 2.0` and requires the HubSpot provider at `>= 0.4.0, < 0.5.0`.
- [x] One module instance manages one hierarchy level through an optional explicit parent ID, stable-keyed direct child folders, and stable-keyed files.
- [x] Deeper hierarchy composes module instances by passing a generated `folder_ids` value downward, creating real dependency and teardown edges without recursive HCL or implicit paths.
- [x] Folder and file keys, parent IDs, names, destinations, access states, SHA-256 values, duplicate names, blocked extensions, and missing destinations are validated at plan time.
- [x] A file may target a stable child-folder key or the explicit parent; a root-level instance cannot upload a file without selecting a managed File folder.
- [x] Resources use `for_each`, ordinary generated-ID references, and no redundant dependency declarations.
- [x] Outputs are exactly stable-keyed folder IDs, file IDs, and the bounded file observations defined by the specification; source paths and complete resource objects are not echoed.
- [x] The module exposes no raw upload options, paths as ownership, URL import, overwrite, TTL, unsupported access, signed URL, cascade deletion, GDPR purge, MIME/charset override, or API route.
- [x] Key-rename guidance requires explicit state movement and explains the otherwise destructive identity change.
- [x] Module tests cover default expansion, typed overrides, invalid keys and relationships, stable addresses, hierarchy dependencies, output shape, and file-first/leaf-first teardown planning.
- [x] The cumulative root and every prior consumer module use provider constraints that admit v0.4.0 without erasing their actual minimum supported release.
- [x] Committed OpenTofu and Terraform locks select the exact v0.4.0 candidate when candidate mode is prepared.
- [x] Generated provider references include both Files resources only because they are registered, and the generated module reference reflects the actual HCL contract.
- [x] The Files surface overview and complete examples document exact scope, Free limit, identity, hierarchy composition, access, source digest, drift, import, collision recovery, Trash retention, exclusions, and ordered teardown.
- [x] Portal generation uses clone/worktree-compatible exact provenance, produces a clean diff, builds and renders successfully, and preserves every prior surface and global index.

## Answer

Delivered the stable-keyed `files-configuration` module and a complete two-level
Northstar example in the demo repository. The module uses ordinary generated-ID
references for hierarchy and teardown ordering, validates the bounded Files
contract at plan time, and exposes only generated IDs and bounded file
observations. OpenTofu and Terraform module tests cover creation, validation,
composition, deletion planning, and automatic teardown; dual-engine graph checks
pin the file-first and leaf-first dependency contract.

Registered Files provider resources now generate reference pages and direct
examples, while the local portal includes the module contract, complete example,
surface overview, lifecycle, import, collision recovery, and provenance. Both
repository gates, generated-documentation checks, portal build/render, and the
independent standards/specification review passed.

Candidate mode is deliberately not prepared in this delivery slice: the
committed OpenTofu and Terraform locks remain prior-release evidence rather than
fabricated v0.4.0 registry selections. Ticket 08 owns candidate preparation and
must regenerate both locks at exactly 0.4.0; its existing compatibility preflight
blocks any live mutation while those selections are stale.
