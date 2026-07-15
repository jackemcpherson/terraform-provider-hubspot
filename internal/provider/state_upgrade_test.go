package provider

import (
	"context"
	"encoding/json"
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

func TestPipelineStateUpgradeComposesCanonicalIdentity(t *testing.T) {
	input := []byte(`{"id":"pipeline-1","object_type":"deals","label":"Pipeline","stages":{}}`)
	resp := &resource.UpgradeStateResponse{}
	pipelineStateUpgrade().StateUpgrader(context.Background(), resource.UpgradeStateRequest{RawState: &tfprotov6.RawState{JSON: input}}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
	}
	var state map[string]any
	if resp.DynamicValue == nil || json.Unmarshal(resp.DynamicValue.JSON, &state) != nil {
		t.Fatalf("invalid upgraded state: %q", resp.DynamicValue.JSON)
	}
	if state["id"] != "deals/pipeline-1" || state["object_type"] != "deals" {
		t.Fatalf("upgraded pipeline identity = %#v", state)
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
