# 02 Move the Current Engine Pins

Type: task

Status: resolved

Label: ready-for-agent

Blocked by: None

## What to Build

"Current engine" qualification means what it says. The current OpenTofu and
Terraform pins (now 1.12.3 and 1.15.8) move to the current stable releases,
and local checks and hosted workflows cannot test different engines.

## Acceptance

- Verify the current stable OpenTofu and Terraform releases against each
  project's release page at implementation time.
- Change the current pins in the Makefile, the validation workflow, and the
  maintenance workflow together.
- Add a guard to the workflow-check script that fails when those pins
  disagree.
- Keep minimum-supported engine versions and recorded state fixtures
  unchanged.
- Pass `make check`, including its engine smoke test, and the workflow checks.
- Merge through `main` as its own pull request with green `Required`, before
  ticket 03's baseline runs.

## Comments

## Answer

The current pins move to OpenTofu 1.13.2 and Terraform 1.17.1, checked on
2026-10-29 against each project's GitHub releases:

- OpenTofu v1.13.2, published 2026-10-21, marked latest:
  <https://github.com/opentofu/opentofu/releases/tag/v1.13.2>
- Terraform v1.17.1, published 2026-10-28, marked latest:
  <https://github.com/hashicorp/terraform/releases/tag/v1.17.1>

Terraform 1.17.1 was one day old at selection. Renovate's seven-day
`minimumReleaseAge` covers Renovate-authored updates, not these hand-moved
pins. 1.17.1 only fixes bugs in 1.17.0 (2026-10-21). To apply the cooldown
anyway, pin 1.17.0 in the Makefile and both workflows. Both Linux amd64
archives download successfully. The Northstar demo at the pinned commit
requires `>= 1.8, < 2.0`, so maintenance accepts the new minors.

The Makefile, `validate-provider.yml`, and `provider-maintenance.yml` move
together. `scripts/engine-smoke.sh` loses its separate 1.12.3 and 1.15.8
defaults. `make engine-smoke` now passes the Makefile pins, so the Makefile is
the only local source. The minimum-supported versions and recorded state
fixtures are unchanged.

`scripts/check-workflows.sh` reads both pins from the Makefile. It fails when
any workflow `tofu-version` or `terraform-version` input disagrees, and when
either qualification workflow drops a pin.
`scripts/check-workflows_test.sh` covers a pristine tree, a disagreeing
workflow pin, a Makefile-only bump, and a removed maintenance pin. The
disagreeing-pin and removed-pin cases failed before their guard code existed.
`make check-workflows` now runs the test.

Gate results, local on darwin/arm64 with Go 1.26.6:

- `make tools`: passed; installed OpenTofu v1.13.2 and Terraform v1.17.1.
- `make check`: passed, including `check-workflows` (actionlint, guard, and
  script tests) and `engine-smoke` against both engines.
- `zizmor .`: no findings.
