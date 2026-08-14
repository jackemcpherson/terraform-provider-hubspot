# Infrastructure Design Guides

These guides are standing design authorities for this provider:

- [Infrastructure Development Style Guide](https://jackemcpherson.com/docs/infrastructure-style-guide.md)
- [Infrastructure Authoring Style Guide](https://jackemcpherson.com/docs/infrastructure-authoring-guide.md)

Read both in full before provider design, implementation, HCL examples, release automation, or CI work. Re-read when the linked documents change.

## Provider translation

Apply their principles to provider development without copying their consumer-repository HCL layout literally:

- OpenTofu is primary; Terraform protocol compatibility remains supported.
- Model declarative desired state. Treat unexplained drift as a correctness failure.
- Make provider schemas typed, validated, documented contracts.
- Prefer stable identity and keyed ownership over positional identity.
- Accept credentials only at boundaries; mark them sensitive; never log them or persist them unnecessarily.
- Document minimum HubSpot scopes and use least privilege.
- Pin dependencies, GitHub Actions, and runner environments to immutable references.
- Run the same format, lint, test, and security gates locally and in CI.
- Test rejection, drift, import, refresh, and deletion behavior alongside happy paths.
- Make releases Git-authored, reviewable, reproducible, signed, and traceable to exact source.
- Explain why in comments and docs; generate repetitive reference documentation where possible.
