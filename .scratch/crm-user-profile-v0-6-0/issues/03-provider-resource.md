# 03: Implement The Provider Resource

Type: task
Status: resolved

Blocked by: 02

## Scope

Register `hubspot_crm_user_profile` with validation, managed-property semantics,
verified updates, both import forms, drift repair, and zero-write destroy.

## Comments

- The schema, validators, exact identity checks, update ordering, import, and
  non-destructive destroy are under test-first implementation.

## Answer

`hubspot_crm_user_profile` is registered with the frozen schema, null-as-
unmanaged semantics, validation, changed-only ordered reconciliation, exact
identity read-back, both import forms, drift repair, and zero-write destroy.
