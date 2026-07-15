# Authentication and aliases

Create a HubSpot static app with the least set of schema permissions needed by
your configuration. Put its access token in `HUBSPOT_ACCESS_TOKEN`; an empty
provider block reads that variable. `access_token` is sensitive when supplied as
a provider argument, but environment-based authentication keeps it out of HCL
and state.

Provider configuration does not contact HubSpot. The first resource or data
source request reports a missing or rejected token.

## Multiple accounts

Each alias has its own client and rate controller. Pass aliases from the root
module instead of configuring providers inside child modules:

```hcl
variable "sandbox_hubspot_access_token" {
  type      = string
  sensitive = true
}

provider "hubspot" {
  alias        = "sandbox"
  access_token = var.sandbox_hubspot_access_token
}

module "sandbox_groups" {
  source = "./modules/property_groups"
  providers = {
    hubspot = hubspot.sandbox
  }
}
```

Set the variable through a secret store or `TF_VAR_sandbox_hubspot_access_token`.
Do not commit `.tfvars` files containing tokens. See the complete
[alias example](../examples/aliases).

`api_base_url` exists for local testing. It accepts HTTPS origins and loopback
HTTP URLs, rejects credentials, query strings, and fragments, and should not be
used as a generic API routing mechanism.
