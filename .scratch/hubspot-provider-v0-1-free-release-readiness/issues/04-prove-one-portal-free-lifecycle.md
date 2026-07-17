# Prove deterministic one-portal Free-tier lifecycle coverage

Type: task
Status: resolved
Blocked by: 02, 03

## Question

What black-box acceptance and cleanup contract proves every advertised Free-tier
behavior against the single disposable portal—create, import, refresh, drift,
warnings, archive/absence, destroy/recreate, discovery, and quota-safe cleanup—on
both OpenTofu and Terraform without mutating CRM records or leaving test state?

## Answer

`make one-portal-free-lifecycle` now coordinates the shared portal: it applies the
demo's reviewed destroy plan, runs only the `free_properties` acceptance shard, and
always recreates the Git-authored demo through a fresh reviewed plan after the
suite, including after an acceptance failure. Its shell-level black-box test proves
both the success and failure call order.

The Free capability manifest and quota preflight cover contacts, companies, deals,
and tickets. The OpenTofu tracer performs create, drift, import, destroy, and
verified cleanup for each; the existing Free lifecycle and Terraform-parity cases
continue to cover archive/absence, discovery, warnings, and cross-engine behavior.
All owned configuration is prefix-scoped, and no test creates or reads CRM records.
`make check` passes.
