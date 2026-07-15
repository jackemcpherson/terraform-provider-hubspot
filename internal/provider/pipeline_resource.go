package provider

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/jackemcpherson/terraform-provider-hubspot/internal/hubspot"
)

type PipelineResource struct{ client *hubspot.PipelineClient }
type pipelineStageModel struct {
	ID               types.String `tfsdk:"id"`
	Label            types.String `tfsdk:"label"`
	DisplayOrder     types.Int64  `tfsdk:"display_order"`
	Metadata         types.Map    `tfsdk:"metadata"`
	WritePermissions types.String `tfsdk:"write_permissions"`
}
type pipelineModel struct {
	ID           types.String `tfsdk:"id"`
	ObjectType   types.String `tfsdk:"object_type"`
	Label        types.String `tfsdk:"label"`
	DisplayOrder types.Int64  `tfsdk:"display_order"`
	Stages       types.Map    `tfsdk:"stages"`
}

func NewPipelineResource() resource.Resource { return &PipelineResource{} }
func (r *PipelineResource) Metadata(_ context.Context, _ resource.MetadataRequest, res *resource.MetadataResponse) {
	res.TypeName = "hubspot_pipeline"
}
func (r *PipelineResource) Schema(_ context.Context, _ resource.SchemaRequest, res *resource.SchemaResponse) {
	stageType := types.ObjectType{AttrTypes: map[string]attr.Type{"id": types.StringType, "label": types.StringType, "display_order": types.Int64Type, "metadata": types.MapType{ElemType: types.StringType}, "write_permissions": types.StringType}}
	res.Schema = schema.Schema{Version: 1, Description: "Manages a deal pipeline and its exclusively owned stages.", Attributes: map[string]schema.Attribute{"id": schema.StringAttribute{Computed: true}, "object_type": schema.StringAttribute{Required: true, Validators: []validator.String{identifierValidator{kind: "CRM object type"}}, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}}, "label": schema.StringAttribute{Required: true}, "display_order": schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(-1)}, "stages": schema.MapAttribute{Required: true, ElementType: stageType}}}
}

func (r *PipelineResource) UpgradeState(context.Context) map[int64]resource.StateUpgrader {
	return map[int64]resource.StateUpgrader{0: identityStateUpgrade()}
}
func (r *PipelineResource) Configure(_ context.Context, req resource.ConfigureRequest, res *resource.ConfigureResponse) {
	clients, ok := req.ProviderData.(*hubspot.ClientSet)
	if !ok || clients == nil || clients.Pipelines == nil {
		res.Diagnostics.AddError("Provider is not configured", "The HubSpot pipeline client was not available.")
		return
	}
	r.client = clients.Pipelines
}
func (r *PipelineResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, res *resource.ModifyPlanResponse) {
	if req.Plan.Raw.IsNull() {
		return
	}
	var plan pipelineModel
	res.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if res.Diagnostics.HasError() || plan.ObjectType.IsUnknown() || plan.Stages.IsUnknown() {
		return
	}
	if _, err := pipelineWriteFromModel(ctx, plan); err != nil {
		res.Diagnostics.AddAttributeError(path.Root("stages"), "Invalid pipeline stage metadata", err.Error())
	}
}
func (r *PipelineResource) Create(ctx context.Context, req resource.CreateRequest, res *resource.CreateResponse) {
	var plan pipelineModel
	res.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if res.Diagnostics.HasError() {
		return
	}
	input, err := pipelineWriteFromModel(ctx, plan)
	if err != nil {
		res.Diagnostics.AddError("Invalid deal pipeline", err.Error())
		return
	}
	out, err := r.client.Create(ctx, plan.ObjectType.ValueString(), input)
	if err != nil {
		appendHubSpotDiagnostic(&res.Diagnostics, "Pipeline creation failed", err)
		return
	}
	verified, verifyErr := r.client.Get(ctx, plan.ObjectType.ValueString(), out.ID)
	if verifyErr != nil {
		appendHubSpotDiagnostic(&res.Diagnostics, "Pipeline creation verification failed", verifyErr)
		return
	}
	res.Diagnostics.Append(res.State.Set(ctx, modelFromPipeline(plan.ObjectType.ValueString(), plan, verified))...)
}
func (r *PipelineResource) Read(ctx context.Context, req resource.ReadRequest, res *resource.ReadResponse) {
	var state pipelineModel
	res.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if res.Diagnostics.HasError() {
		return
	}
	out, err := r.client.Get(ctx, state.ObjectType.ValueString(), state.ID.ValueString())
	if err != nil {
		if isNotFound(err) {
			res.State.RemoveResource(ctx)
			return
		}
		appendHubSpotDiagnostic(&res.Diagnostics, "Pipeline refresh failed", err)
		return
	}
	res.Diagnostics.Append(res.State.Set(ctx, modelFromPipeline(state.ObjectType.ValueString(), state, out))...)
}
func (r *PipelineResource) Update(ctx context.Context, req resource.UpdateRequest, res *resource.UpdateResponse) {
	var plan, old pipelineModel
	res.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	res.Diagnostics.Append(req.State.Get(ctx, &old)...)
	if res.Diagnostics.HasError() {
		return
	}
	input, err := pipelineWriteFromModel(ctx, plan)
	if err != nil {
		res.Diagnostics.AddError("Invalid deal pipeline", err.Error())
		return
	}
	if old.ID.IsNull() || old.ID.ValueString() == "" {
		res.Diagnostics.AddError("Pipeline identity missing", "The pipeline ID was not present in state.")
		return
	}
	if _, err = r.client.Update(ctx, "deals", old.ID.ValueString(), input); err != nil {
		appendHubSpotDiagnostic(&res.Diagnostics, "Pipeline update failed", err)
		return
	}
	out, err := r.client.Get(ctx, "deals", old.ID.ValueString())
	if err != nil {
		appendHubSpotDiagnostic(&res.Diagnostics, "Pipeline update verification failed", err)
		return
	}
	res.Diagnostics.Append(res.State.Set(ctx, modelFromPipeline(plan.ObjectType.ValueString(), plan, out))...)
}
func (r *PipelineResource) Delete(ctx context.Context, req resource.DeleteRequest, res *resource.DeleteResponse) {
	var state pipelineModel
	res.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if res.Diagnostics.HasError() {
		return
	}
	if err := r.client.Archive(ctx, state.ObjectType.ValueString(), state.ID.ValueString()); err != nil {
		if isNotFound(err) {
			res.State.RemoveResource(ctx)
			return
		}
		appendHubSpotDiagnostic(&res.Diagnostics, "Pipeline archival failed", err)
		return
	}
	out, err := r.client.Get(ctx, state.ObjectType.ValueString(), state.ID.ValueString())
	if err != nil {
		if isNotFound(err) {
			res.State.RemoveResource(ctx)
			return
		}
		appendHubSpotDiagnostic(&res.Diagnostics, "Pipeline archival verification failed", err)
		return
	}
	if !out.Archived {
		res.Diagnostics.AddError("Pipeline archival was not verified", "The pipeline remains active after archive; state was retained.")
		return
	}
	res.State.RemoveResource(ctx)
}
func (r *PipelineResource) ImportState(ctx context.Context, req resource.ImportStateRequest, res *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, "/")
	if len(parts) != 2 || !validImportPart(parts[0]) || !validImportPart(parts[1]) {
		res.Diagnostics.AddAttributeError(path.Root("id"), "Invalid pipeline import ID", "Use exact object_type/pipeline_id form.")
		return
	}
	out, err := r.client.Get(ctx, parts[0], parts[1])
	if err != nil {
		appendHubSpotDiagnostic(&res.Diagnostics, "Pipeline import failed", err)
		return
	}
	for _, stage := range out.Stages {
		if stage.WritePermissions == "READ_ONLY" || stage.WritePermissions == "INTERNAL_ONLY" {
			res.Diagnostics.AddError("Pipeline contains protected stages", "Protected stages cannot be managed by hubspot_pipeline.")
			return
		}
	}
	state := pipelineModel{ID: types.StringValue(out.ID), ObjectType: types.StringValue(parts[0]), Label: types.StringValue(out.Label), DisplayOrder: types.Int64Value(out.DisplayOrder), Stages: stageMap(out.Stages)}
	res.Diagnostics.Append(res.State.Set(ctx, &state)...)
}

func pipelineWriteFromModel(ctx context.Context, m pipelineModel) (hubspot.PipelineWrite, error) {
	var stages map[string]pipelineStageModel
	if diags := m.Stages.ElementsAs(ctx, &stages, false); diags.HasError() {
		return hubspot.PipelineWrite{}, errors.New("invalid stage map")
	}
	keys := make([]string, 0, len(stages))
	for key := range stages {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := hubspot.PipelineWrite{Label: m.Label.ValueString(), DisplayOrder: m.DisplayOrder.ValueInt64(), Stages: make([]hubspot.PipelineStageWrite, 0, len(keys))}
	for _, key := range keys {
		s := stages[key]
		metadata := map[string]string{}
		if !s.Metadata.IsNull() && !s.Metadata.IsUnknown() {
			if diags := s.Metadata.ElementsAs(ctx, &metadata, false); diags.HasError() {
				return hubspot.PipelineWrite{}, errors.New("invalid stage metadata")
			}
		}
		if probability, ok := metadata["probability"]; ok {
			value, err := strconv.ParseFloat(probability, 64)
			if err != nil || value < 0 || value > 1 || value*10 != float64(int(value*10)) {
				return hubspot.PipelineWrite{}, fmt.Errorf("stage %q probability must be 0.0 through 1.0 in 0.1 increments", key)
			}
		}
		if m.ObjectType.ValueString() == "tickets" {
			if ticketState, ok := metadata["ticketState"]; ok && ticketState != "OPEN" && ticketState != "CLOSED" {
				return hubspot.PipelineWrite{}, fmt.Errorf("stage %q ticketState must be OPEN or CLOSED", key)
			}
		}
		out.Stages = append(out.Stages, hubspot.PipelineStageWrite{Label: s.Label.ValueString(), DisplayOrder: s.DisplayOrder.ValueInt64(), Metadata: metadata})
	}
	if len(out.Stages) == 0 {
		return hubspot.PipelineWrite{}, errors.New("pipeline requires at least one stage")
	}
	return out, nil
}
func stageMap(stages []hubspot.PipelineStage) types.Map {
	vals := map[string]attr.Value{}
	for _, s := range stages {
		metadata := map[string]attr.Value{}
		for k, v := range s.Metadata {
			metadata[k] = types.StringValue(v)
		}
		vals[s.ID] = types.ObjectValueMust(map[string]attr.Type{"id": types.StringType, "label": types.StringType, "display_order": types.Int64Type, "metadata": types.MapType{ElemType: types.StringType}, "write_permissions": types.StringType}, map[string]attr.Value{"id": types.StringValue(s.ID), "label": types.StringValue(s.Label), "display_order": types.Int64Value(s.DisplayOrder), "metadata": types.MapValueMust(types.StringType, metadata), "write_permissions": types.StringValue(s.WritePermissions)})
	}
	return types.MapValueMust(types.ObjectType{AttrTypes: map[string]attr.Type{"id": types.StringType, "label": types.StringType, "display_order": types.Int64Type, "metadata": types.MapType{ElemType: types.StringType}, "write_permissions": types.StringType}}, vals)
}
func modelFromPipeline(objectType string, prior pipelineModel, p hubspot.Pipeline) pipelineModel {
	known := map[string]string{}
	var old map[string]pipelineStageModel
	if !prior.Stages.IsNull() && !prior.Stages.IsUnknown() && !prior.Stages.ElementsAs(context.Background(), &old, false).HasError() {
		for key, stage := range old {
			if !stage.ID.IsNull() {
				known[stage.ID.ValueString()] = key
			}
		}
	}
	vals := map[string]attr.Value{}
	stageType := types.ObjectType{AttrTypes: map[string]attr.Type{"id": types.StringType, "label": types.StringType, "display_order": types.Int64Type, "metadata": types.MapType{ElemType: types.StringType}, "write_permissions": types.StringType}}
	for _, stage := range p.Stages {
		key := stage.ID
		if priorKey, ok := known[stage.ID]; ok {
			key = priorKey
		}
		vals[key] = stageObject(stage, stageType)
	}
	return pipelineModel{ID: types.StringValue(p.ID), ObjectType: types.StringValue(objectType), Label: types.StringValue(p.Label), DisplayOrder: types.Int64Value(p.DisplayOrder), Stages: types.MapValueMust(types.ObjectType{AttrTypes: stageType.AttrTypes}, vals)}
}

func stageObject(s hubspot.PipelineStage, typ types.ObjectType) attr.Value {
	metadata := map[string]attr.Value{}
	for k, v := range s.Metadata {
		metadata[k] = types.StringValue(v)
	}
	return types.ObjectValueMust(typ.AttrTypes, map[string]attr.Value{"id": types.StringValue(s.ID), "label": types.StringValue(s.Label), "display_order": types.Int64Value(s.DisplayOrder), "metadata": types.MapValueMust(types.StringType, metadata), "write_permissions": types.StringValue(s.WritePermissions)})
}
