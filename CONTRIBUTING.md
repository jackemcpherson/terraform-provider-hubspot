# Contributing

Install the exact Go and OpenTofu versions named in `Makefile`, then run:

```sh
make tools
make check
```

Provider development must preserve protocol 6, typed validated schemas,
observation-only refresh, authored drift repair, and secret-safe diagnostics.
Do not use real HubSpot credentials in pull requests or local fixtures. Live
acceptance belongs only to protected workflows with isolated account capability
manifests.

Every user-visible change updates `CHANGELOG.md`. Keep generated documentation
in sync with provider schema changes by running `make docs`.
