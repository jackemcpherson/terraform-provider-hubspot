# Custom schema teardown

Schema deletion protection defaults on. Disable it in a prior authored apply;
destroy then performs a read-only preflight and refuses mutation when properties
outside the owned map remain. `expected_external_properties` documents sibling
ownership without adopting or deleting those definitions. CRM records and
associations are never read by this provider.
