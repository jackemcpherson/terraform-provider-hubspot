# 04 Decide and Ship Any v0.7.x Patch

Type: task

Status: open

Label: ready-for-agent

Blocked by: 03

## What to Build

Registry users get the patched standard library without waiting for v0.8.0,
but only when the repair changes shipped provider binaries.

## Acceptance

- Record whether any reachable finding from ticket 01 reaches the provider
  binary itself. Findings that reach only the docs-portal server or provenance
  tools do not require a patch.
- If a patch is required, publish one signed immutable v0.7.x release from the
  qualified `main` commit, verify its complete asset set, and record Terraform
  Registry and OpenTofu Registry ingestion separately.
- If no patch is required, record the reasoning and close the ticket.

## Comments
