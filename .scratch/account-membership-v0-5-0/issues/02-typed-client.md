# Implement The Typed Settings Client

Type: task
Status: resolved
Blocked by: 01

Add typed list, ID and email read, create, safe name update, delete, and bounded
absence verification for `/settings/users/2026-03`.

## Answer

The client follows list cursors, ignores unknown fields, preserves generated
IDs for exact recovery, and verifies deletion through ID, email, and list reads.
