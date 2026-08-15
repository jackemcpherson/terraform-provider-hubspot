# 04: Prove The Command-Line Lifecycle And Maintenance

Type: task
Status: resolved

Blocked by: 03

## Scope

Extend the behavioural fake and cumulative Northstar journey for readiness,
identity failures, drift, import, no-op updates, and non-destructive destroy.

## Comments

## Answer

The behavioural fake and real-CLI acceptance suite cover delayed readiness,
missing and duplicate joins, drift and repair, API rejection, both import forms,
semantic no-op updates, and retained values after zero-write destroy under both
engines. The cumulative Northstar helper verifies, drifts, and terminally checks
the separate profile residual without logging account identities.
