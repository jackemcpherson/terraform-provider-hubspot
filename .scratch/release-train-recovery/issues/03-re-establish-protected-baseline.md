# 03 Re-establish the Protected Baseline

Type: task

Status: resolved

Label: ready-for-agent

Blocked by: 01, 02

## What to Build

v0.8.0 qualifies against a proven baseline. Two consecutive green protected
maintenance runs on the repaired `main` replace the stale 2026-08-18 baseline,
and release history explains the gap.

## Acceptance

- Confirm the protected environment variables and scopes without reading
  secret values, as release-slice discipline step 1 requires.
- Pass two consecutive protected maintenance runs on the same unchanged
  provider commit and demo `2ef32f7`.
- Add a failure ledger entry for the 2026-10-12, 2026-10-19, and 2026-10-26
  scheduled failures, with phase, run, commits, cause, and fix.
- Record the new baseline provider and demo commits and run URLs.

## Comments

## Answer

The two-run protected baseline is re-established on provider `18224aa` and
demo `2ef32f7`.

- 2026-10-29: `gh auth status` succeeded for the maintainer account. The
  `northstar` environment still defines `HUBSPOT_ACCEPTANCE_PORTAL_ID`,
  `HUBSPOT_NORTHSTAR_MEMBERSHIP_EMAIL`, and the `HUBSPOT_ACCESS_TOKEN` secret,
  checked by name only. The secret was last updated on 2026-08-09, before the
  2026-08-18 baseline, so its surface scopes are unchanged. The environment
  still admits protected branches only.
- Run 1: https://github.com/jackemcpherson/terraform-provider-hubspot/actions/runs/40811520501
  passed every step on `18224aa` and `2ef32f7`.
- Run 2: https://github.com/jackemcpherson/terraform-provider-hubspot/actions/runs/40812498133
  passed every step on the same commits.

## Failure Ledger

| Phase | Run | Provider | Demo | Cause | Fix |
| --- | --- | --- | --- | --- | --- |
| Run maintenance (`check-security`) | https://github.com/jackemcpherson/terraform-provider-hubspot/actions/runs/38249172760 | `168d8e8` | `2ef32f7` | `govulncheck` reported reachable `x/net` HTTP/2 findings, including GO-2026-6617, and Go 1.26.6 standard-library findings | Ticket 01 (PR 115) |
| Run maintenance (`check-security`) | https://github.com/jackemcpherson/terraform-provider-hubspot/actions/runs/39236383532 | `168d8e8` | `2ef32f7` | Same findings as 2026-10-12 | Ticket 01 (PR 115) |
| Run maintenance (`check-security`) | https://github.com/jackemcpherson/terraform-provider-hubspot/actions/runs/40308028464 | `9b285f2` | `2ef32f7` | Same findings, plus GO-2026-6809 (`crypto/x509`, fixed in Go 1.26.10) | Ticket 01 (PR 115) |
