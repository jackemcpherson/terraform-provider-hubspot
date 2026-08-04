# Files configuration

The Files configuration surface manages explicit HubSpot File Manager hierarchy
and reusable Managed files. It requires the exact `files` scope and is proven on
a normal HubSpot Free account. CMS Developer File System content, runtime file
delivery, signed URLs, URL import, overwrite, cascade deletion, and GDPR purge
are excluded.

`hubspot_file_folder` and `hubspot_file` own only non-zero decimal generated IDs.
Names, folder paths, file paths, URLs, hashes, sizes, and timestamps are mutable
or computed observations. Import therefore accepts an exact generated ID and
never searches by name or path. Create and update reject collisions instead of
adopting an existing asset.

If create returns a known generated ID but its read-back is normalized or does
not match the plan, the provider attempts to delete that exact residual and
verify active absence. If cleanup cannot be proven, retain the state written by
the provider when available and use the diagnostic's exact generated ID: inspect
that asset directly, import only that confirmed ID if it is the intended asset,
or remove the residual before retrying. When create returns no ID, inspect only
the exact intended parent or folder. Never search for or adopt collision
recovery state by name, path, or URL.

Managed file bytes come from a sensitive local `source_path` bound to a reviewed
lowercase SHA-256 digest. The provider validates the source at plan and apply,
enforces the Free 20,000,000-byte file limit and blocked executable extensions,
and never stores bytes in state or diagnostics. Remote MD5 and size observations
drive content drift repair. `PRIVATE` is the default; `PUBLIC_INDEXABLE` and
`PUBLIC_NOT_INDEXABLE` are the other supported access states.

The `files-configuration` consumer module manages one hierarchy level with
stable map keys. Compose deeper hierarchy by passing a generated `folder_ids`
value to the child module's `parent_folder_id`. This makes dependencies explicit
and orders teardown file-first and leaf-first. Rename a map key only with a
reviewed `moved` block or OpenTofu will propose a new generated identity.

Folder deletion refuses active children. File and empty-folder deletion verify
active absence but do not claim physical erasure: HubSpot controls subsequent
Trash retention. The executable Northstar composition example demonstrates two
folder levels and private and public non-indexable files. The protected
cumulative journey adds byte replacement, drift repair, exact-ID import, path
refresh, and ordered cleanup before candidate qualification.
