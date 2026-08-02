# Acceptance state fixtures

`v0.1.6-property-state.json` is captured with the released v0.1.6 provider by
running `TestCaptureV016PropertyState` with `HUBSPOT_ACCEPTANCE_PROVIDER_BINARY`
pointing at that released binary and `HUBSPOT_V016_STATE_CAPTURE` pointing at
the fixture path. Normal tests load the committed state through each real CLI
and require an empty v0.2.0 plan.
