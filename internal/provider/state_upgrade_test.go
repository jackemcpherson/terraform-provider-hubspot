package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

func TestIdentityStateUpgradePreservesJSONBytes(t *testing.T) {
	input := []byte(`{"id":"obj/one","stages":{"closed":{"id":"2"}},"deletion_protection":true}`)
	resp := &resource.UpgradeStateResponse{}
	identityStateUpgrade().StateUpgrader(context.Background(), resource.UpgradeStateRequest{RawState: &tfprotov6.RawState{JSON: input}}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
	}
	if resp.DynamicValue == nil || string(resp.DynamicValue.JSON) != string(input) {
		t.Fatalf("state bytes changed during identity migration: got %q", resp.DynamicValue.JSON)
	}
}

func TestIdentityStateUpgradeRejectsFlatmapWithoutRewrite(t *testing.T) {
	resp := &resource.UpgradeStateResponse{}
	identityStateUpgrade().StateUpgrader(context.Background(), resource.UpgradeStateRequest{RawState: &tfprotov6.RawState{Flatmap: map[string]string{"id": "obj/one"}}}, resp)
	if !resp.Diagnostics.HasError() {
		t.Fatal("expected unsupported encoding diagnostic")
	}
	if resp.DynamicValue != nil {
		t.Fatal("upgrade returned state after rejecting encoding")
	}
}
