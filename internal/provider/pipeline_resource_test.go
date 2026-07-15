package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/jackemcpherson/terraform-provider-hubspot/internal/hubspot"
)

func TestModelFromPipelinePreservesPendingLogicalStageKey(t *testing.T) {
	prior := pipelineModel{Stages: testPipelineStageMap(map[string]pipelineStageModel{
		"closed":  pipelineStage("stage-1", "Closed", 20, "1.0"),
		"nurture": pipelineStage("", "Nurture", 15, "0.5"),
		"open":    pipelineStage("stage-2", "Qualified", 10, "0.2"),
	})}
	remote := hubspot.Pipeline{ID: "pipeline-1", Label: "Updated pipeline", DisplayOrder: 20, Stages: []hubspot.PipelineStage{
		{ID: "stage-1", Label: "Closed", DisplayOrder: 20, Metadata: map[string]string{"probability": "1.0"}, WritePermissions: "CRM_PERMISSIONS_ENFORCEMENT"},
		{ID: "stage-2", Label: "Qualified", DisplayOrder: 10, Metadata: map[string]string{"probability": "0.2"}, WritePermissions: "CRM_PERMISSIONS_ENFORCEMENT"},
		{ID: "stage-3", Label: "Nurture", DisplayOrder: 15, Metadata: map[string]string{"probability": "0.5"}, WritePermissions: "CRM_PERMISSIONS_ENFORCEMENT"},
	}}

	model := modelFromPipeline("deals", prior, remote)
	var stages map[string]pipelineStageModel
	if diagnostics := model.Stages.ElementsAs(context.Background(), &stages, false); diagnostics.HasError() {
		t.Fatalf("decode modeled stages: %v", diagnostics)
	}
	if stages["nurture"].ID.ValueString() != "stage-3" {
		t.Fatalf("new remote stage was not correlated to its caller logical key")
	}
}

func pipelineStage(id, label string, order int64, probability string) pipelineStageModel {
	stageID := types.StringValue(id)
	if id == "" {
		stageID = types.StringUnknown()
	}
	return pipelineStageModel{
		ID:               stageID,
		Label:            types.StringValue(label),
		DisplayOrder:     types.Int64Value(order),
		Metadata:         types.MapValueMust(types.StringType, map[string]attr.Value{"probability": types.StringValue(probability)}),
		WritePermissions: types.StringUnknown(),
	}
}

func testPipelineStageMap(stages map[string]pipelineStageModel) types.Map {
	values := make(map[string]attr.Value, len(stages))
	stageType := types.ObjectType{AttrTypes: map[string]attr.Type{"id": types.StringType, "label": types.StringType, "display_order": types.Int64Type, "metadata": types.MapType{ElemType: types.StringType}, "write_permissions": types.StringType}}
	for key, stage := range stages {
		values[key] = types.ObjectValueMust(stageType.AttrTypes, map[string]attr.Value{
			"id": stage.ID, "label": stage.Label, "display_order": stage.DisplayOrder,
			"metadata": stage.Metadata, "write_permissions": stage.WritePermissions,
		})
	}
	return types.MapValueMust(stageType, values)
}
