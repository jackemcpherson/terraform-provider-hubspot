package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

// identityStateUpgrade is the first historical state boundary for every
// managed resource. Version 0 had the same wire shape as version 1, so the
// migration deliberately copies the opaque JSON bytes without contacting
// HubSpot or re-encoding values (which could change set ordering or nulls).
// Flatmap state is rejected explicitly because silently dropping it would
// violate lossless migration; users can first refresh it with a supported
// Terraform/OpenTofu release.
func identityStateUpgrade() resource.StateUpgrader {
	return resource.StateUpgrader{StateUpgrader: func(_ context.Context, req resource.UpgradeStateRequest, resp *resource.UpgradeStateResponse) {
		if req.RawState == nil || req.RawState.JSON == nil {
			resp.Diagnostics.AddError(
				"Unsupported legacy state encoding",
				"This provider can migrate JSON state only. Run `terraform refresh` or `tofu refresh` with the previous provider version, then retry; no state was rewritten.",
			)
			return
		}
		resp.DynamicValue = &tfprotov6.DynamicValue{JSON: append([]byte(nil), req.RawState.JSON...)}
	}}
}
