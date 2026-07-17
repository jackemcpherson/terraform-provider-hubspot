# Prove deterministic one-portal Free-tier lifecycle coverage

Type: task
Status: open
Blocked by: 02, 03

## Question

What black-box acceptance and cleanup contract proves every advertised Free-tier
behavior against the single disposable portal—create, import, refresh, drift,
warnings, archive/absence, destroy/recreate, discovery, and quota-safe cleanup—on
both OpenTofu and Terraform without mutating CRM records or leaving test state?
