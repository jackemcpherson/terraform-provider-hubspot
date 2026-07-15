package provider

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/jackemcpherson/terraform-provider-hubspot/internal/hubspot"
)

type PipelineResource struct{ client *hubspot.PipelineClient }

const pipelineArchivedPrivateKey = "pipeline_archived"

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
	res.Schema = schema.Schema{Version: 1, Description: "Manages a pipeline and its exclusively owned writable stages.", Attributes: map[string]schema.Attribute{
		"id":            schema.StringAttribute{Computed: true, Description: "Canonical object_type/pipeline_id identity.", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
		"object_type":   schema.StringAttribute{Required: true, Description: "Exact HubSpot CRM object type API identifier.", Validators: []validator.String{identifierValidator{kind: "CRM object type"}}, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
		"label":         schema.StringAttribute{Required: true, Description: "Pipeline display label."},
		"display_order": schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(-1), Description: "HubSpot display order; defaults to -1."},
		"stages": schema.MapNestedAttribute{Required: true, Description: "Complete set of writable stages, keyed by stable local identity.", NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
			"id":                schema.StringAttribute{Computed: true, Description: "HubSpot-generated stage ID.", PlanModifiers: []planmodifier.String{computedStringStateOrUnknown{}}},
			"label":             schema.StringAttribute{Required: true, Description: "Stage display label."},
			"display_order":     schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(-1), Description: "HubSpot display order; defaults to -1."},
			"metadata":          schema.MapAttribute{Optional: true, Computed: true, ElementType: types.StringType, Default: mapdefault.StaticValue(types.MapValueMust(types.StringType, map[string]attr.Value{})), Description: "Object-specific stage metadata."},
			"write_permissions": schema.StringAttribute{Computed: true, Description: "HubSpot stage write-permission classification.", PlanModifiers: []planmodifier.String{computedStringStateOrUnknown{}}},
		}}},
	}}
}

type computedStringStateOrUnknown struct{}

func (computedStringStateOrUnknown) Description(context.Context) string {
	return "Preserves a known computed value and plans newly computed values as unknown."
}

func (m computedStringStateOrUnknown) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (computedStringStateOrUnknown) PlanModifyString(_ context.Context, req planmodifier.StringRequest, res *planmodifier.StringResponse) {
	if !req.PlanValue.IsNull() && !req.PlanValue.IsUnknown() {
		return
	}
	if !req.StateValue.IsNull() && !req.StateValue.IsUnknown() {
		res.PlanValue = req.StateValue
		return
	}
	res.PlanValue = types.StringUnknown()
}

func (r *PipelineResource) UpgradeState(context.Context) map[int64]resource.StateUpgrader {
	return map[int64]resource.StateUpgrader{0: pipelineStateUpgrade()}
}
func (r *PipelineResource) Configure(_ context.Context, req resource.ConfigureRequest, res *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
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
	archived, diagnostics := req.Private.GetKey(ctx, pipelineArchivedPrivateKey)
	res.Diagnostics.Append(diagnostics...)
	if !res.Diagnostics.HasError() && string(archived) == "true" {
		res.Diagnostics.Append(res.Plan.SetAttribute(ctx, path.Root("id"), types.StringUnknown())...)
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
	model, modelErr := r.verifiedPipelineModel(ctx, plan.ObjectType.ValueString(), out.ID, plan, verified)
	if modelErr != nil {
		appendPipelineVerificationDiagnostic(&res.Diagnostics, "Pipeline creation was not verified", modelErr)
		return
	}
	res.Diagnostics.Append(res.State.Set(ctx, model)...)
}
func (r *PipelineResource) Read(ctx context.Context, req resource.ReadRequest, res *resource.ReadResponse) {
	var state pipelineModel
	res.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if res.Diagnostics.HasError() {
		return
	}
	remoteID := pipelineRemoteID(state.ObjectType.ValueString(), state.ID.ValueString())
	out, err := r.getPipelineIncludingArchived(ctx, state.ObjectType.ValueString(), remoteID)
	if err != nil {
		if isNotFound(err) {
			res.State.RemoveResource(ctx)
			return
		}
		appendHubSpotDiagnostic(&res.Diagnostics, "Pipeline refresh failed", err)
		return
	}
	if out.Archived {
		res.Diagnostics.Append(res.Private.SetKey(ctx, pipelineArchivedPrivateKey, []byte("true"))...)
	} else {
		res.Diagnostics.Append(res.Private.SetKey(ctx, pipelineArchivedPrivateKey, nil)...)
	}
	model := modelFromPipeline(state.ObjectType.ValueString(), state, out)
	if modelErr := r.preservePipelineAppendOrders(ctx, state.ObjectType.ValueString(), out, state, &model); modelErr != nil {
		appendHubSpotDiagnostic(&res.Diagnostics, "Pipeline append-order refresh failed", modelErr)
		return
	}
	res.Diagnostics.Append(res.State.Set(ctx, model)...)
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
	remoteID := pipelineRemoteID(old.ObjectType.ValueString(), old.ID.ValueString())
	archived, privateDiagnostics := req.Private.GetKey(ctx, pipelineArchivedPrivateKey)
	res.Diagnostics.Append(privateDiagnostics...)
	if res.Diagnostics.HasError() {
		return
	}
	if string(archived) == "true" {
		_, restoreErr := r.client.Restore(ctx, old.ObjectType.ValueString(), remoteID)
		restored, verifyErr := r.client.Get(ctx, old.ObjectType.ValueString(), remoteID)
		if verifyErr != nil || restored.Archived || restored.ID != remoteID {
			if restoreErr != nil {
				appendHubSpotDiagnostic(&res.Diagnostics, "Pipeline restore failed", restoreErr)
				return
			}
			if verifyErr != nil {
				appendHubSpotDiagnostic(&res.Diagnostics, "Pipeline restore verification failed", verifyErr)
				return
			}
			res.Diagnostics.AddError("Pipeline restore was not verified", "HubSpot did not return the same active pipeline identity after restore; state was retained.")
			return
		}
		res.Diagnostics.Append(res.Private.SetKey(ctx, pipelineArchivedPrivateKey, nil)...)
	}
	_, updateErr := r.client.Update(ctx, old.ObjectType.ValueString(), remoteID, input)
	out, verifyErr := r.client.Get(ctx, old.ObjectType.ValueString(), remoteID)
	if verifyErr != nil {
		if updateErr != nil {
			appendHubSpotDiagnostic(&res.Diagnostics, "Pipeline update failed", updateErr)
			return
		}
		appendHubSpotDiagnostic(&res.Diagnostics, "Pipeline update verification failed", verifyErr)
		return
	}
	model, modelErr := r.verifiedPipelineModel(ctx, plan.ObjectType.ValueString(), remoteID, plan, out)
	if modelErr != nil {
		if updateErr != nil {
			appendHubSpotDiagnostic(&res.Diagnostics, "Pipeline update failed", updateErr)
			return
		}
		appendPipelineVerificationDiagnostic(&res.Diagnostics, "Pipeline update was not verified", modelErr)
		return
	}
	res.Diagnostics.Append(res.State.Set(ctx, model)...)
}
func (r *PipelineResource) Delete(ctx context.Context, req resource.DeleteRequest, res *resource.DeleteResponse) {
	var state pipelineModel
	res.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if res.Diagnostics.HasError() {
		return
	}
	remoteID := pipelineRemoteID(state.ObjectType.ValueString(), state.ID.ValueString())
	current, currentErr := r.getPipelineIncludingArchived(ctx, state.ObjectType.ValueString(), remoteID)
	if currentErr != nil {
		if isNotFound(currentErr) {
			res.Diagnostics.AddError("Pipeline archival was not verified", "The canonical pipeline was absent from the readable API, so its required archived terminal state could not be verified; state was retained.")
			return
		}
		appendHubSpotDiagnostic(&res.Diagnostics, "Pipeline archival preflight failed", currentErr)
		return
	}
	if current.ID != remoteID {
		res.Diagnostics.AddError("Pipeline archival was not verified", "HubSpot returned a different pipeline identity; state was retained.")
		return
	}
	if current.Archived {
		res.State.RemoveResource(ctx)
		return
	}
	archiveErr := r.client.Archive(ctx, state.ObjectType.ValueString(), remoteID)
	out, verifyErr := r.client.GetArchived(ctx, state.ObjectType.ValueString(), remoteID)
	if verifyErr != nil {
		if archiveErr != nil {
			appendHubSpotDiagnostic(&res.Diagnostics, "Pipeline archival failed", archiveErr)
			return
		}
		appendHubSpotDiagnostic(&res.Diagnostics, "Pipeline archival verification failed", verifyErr)
		return
	}
	if out.ID != remoteID || !out.Archived {
		res.Diagnostics.AddError("Pipeline archival was not verified", "HubSpot did not return the same archived pipeline identity; state was retained.")
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
	out, err := r.getPipelineIncludingArchived(ctx, parts[0], parts[1])
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
	state := pipelineModel{ID: types.StringValue(parts[0] + "/" + out.ID), ObjectType: types.StringValue(parts[0]), Label: types.StringValue(out.Label), DisplayOrder: types.Int64Value(out.DisplayOrder), Stages: stageMap(out.Stages)}
	res.Diagnostics.Append(res.State.Set(ctx, &state)...)
	if out.Archived {
		res.Diagnostics.Append(res.Private.SetKey(ctx, pipelineArchivedPrivateKey, []byte("true"))...)
	}
}

func (r *PipelineResource) getPipelineIncludingArchived(ctx context.Context, objectType, id string) (hubspot.Pipeline, error) {
	active, err := r.client.Get(ctx, objectType, id)
	if err == nil || !isNotFound(err) {
		return active, err
	}
	return r.client.GetArchived(ctx, objectType, id)
}

func (r *PipelineResource) verifiedPipelineModel(ctx context.Context, objectType, remoteID string, plan pipelineModel, out hubspot.Pipeline) (pipelineModel, error) {
	if out.ID != remoteID || out.Archived {
		return pipelineModel{}, errors.New("HubSpot did not return the same active pipeline identity")
	}
	model := modelFromPipeline(objectType, plan, out)
	if err := r.preservePipelineAppendOrders(ctx, objectType, out, plan, &model); err != nil {
		return pipelineModel{}, fmt.Errorf("append-order verification failed: %w", err)
	}
	if err := pipelineModelMatchesPlan(ctx, plan, model); err != nil {
		return pipelineModel{}, err
	}
	return model, nil
}

func appendPipelineVerificationDiagnostic(diagnostics *diag.Diagnostics, summary string, err error) {
	var apiError *hubspot.Error
	if errors.As(err, &apiError) {
		appendHubSpotDiagnostic(diagnostics, summary, err)
		return
	}
	diagnostics.AddError(summary, err.Error()+". State was retained.")
}

func pipelineModelMatchesPlan(ctx context.Context, plan, actual pipelineModel) error {
	if plan.Label.IsUnknown() || actual.Label.ValueString() != plan.Label.ValueString() ||
		plan.DisplayOrder.IsUnknown() || actual.DisplayOrder.ValueInt64() != plan.DisplayOrder.ValueInt64() {
		return errors.New("HubSpot returned pipeline fields that differ from the configured state")
	}
	var planned, observed map[string]pipelineStageModel
	if diagnostics := plan.Stages.ElementsAs(ctx, &planned, false); diagnostics.HasError() {
		return errors.New("configured pipeline stages could not be decoded")
	}
	if diagnostics := actual.Stages.ElementsAs(ctx, &observed, false); diagnostics.HasError() {
		return errors.New("HubSpot pipeline stages could not be decoded")
	}
	if len(planned) != len(observed) {
		return errors.New("HubSpot returned a different number of pipeline stages than configured")
	}
	for key, expected := range planned {
		got, ok := observed[key]
		if !ok {
			return errors.New("HubSpot returned an unexpected pipeline stage identity")
		}
		if !expected.ID.IsNull() && !expected.ID.IsUnknown() && expected.ID.ValueString() != got.ID.ValueString() {
			return errors.New("HubSpot changed an existing pipeline stage identity")
		}
		if expected.Label.IsUnknown() || expected.Label.ValueString() != got.Label.ValueString() ||
			expected.DisplayOrder.IsUnknown() || expected.DisplayOrder.ValueInt64() != got.DisplayOrder.ValueInt64() ||
			!expected.Metadata.Equal(got.Metadata) {
			return errors.New("HubSpot returned pipeline stage fields that differ from the configured state")
		}
	}
	return nil
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
	labels := make(map[string]string, len(keys))
	for _, key := range keys {
		s := stages[key]
		label := s.Label.ValueString()
		if previousKey, exists := labels[label]; exists {
			_ = previousKey
			return hubspot.PipelineWrite{}, errors.New("two stages use the same label")
		}
		labels[label] = key
		metadata := map[string]string{}
		if !s.Metadata.IsNull() && !s.Metadata.IsUnknown() {
			if diags := s.Metadata.ElementsAs(ctx, &metadata, false); diags.HasError() {
				return hubspot.PipelineWrite{}, errors.New("invalid stage metadata")
			}
		}
		if m.ObjectType.ValueString() == "deals" {
			probability, ok := metadata["probability"]
			if !ok {
				return hubspot.PipelineWrite{}, errors.New("every deal stage requires probability metadata")
			}
			value, err := strconv.ParseFloat(probability, 64)
			if err != nil || value < 0 || value > 1 || value*10 != float64(int(value*10)) {
				return hubspot.PipelineWrite{}, errors.New("deal stage probability must be 0.0 through 1.0 in 0.1 increments")
			}
		}
		if m.ObjectType.ValueString() == "tickets" {
			if ticketState, ok := metadata["ticketState"]; ok && ticketState != "OPEN" && ticketState != "CLOSED" {
				return hubspot.PipelineWrite{}, errors.New("ticket stage ticketState must be OPEN or CLOSED")
			}
		}
		stageID := ""
		if !s.ID.IsNull() && !s.ID.IsUnknown() {
			stageID = s.ID.ValueString()
		}
		out.Stages = append(out.Stages, hubspot.PipelineStageWrite{StageID: stageID, Label: label, DisplayOrder: s.DisplayOrder.ValueInt64(), Metadata: metadata})
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
	pending := map[string]string{}
	var old map[string]pipelineStageModel
	if !prior.Stages.IsNull() && !prior.Stages.IsUnknown() && !prior.Stages.ElementsAs(context.Background(), &old, false).HasError() {
		for key, stage := range old {
			if !stage.ID.IsNull() && !stage.ID.IsUnknown() && stage.ID.ValueString() != "" {
				known[stage.ID.ValueString()] = key
			} else if !stage.Label.IsNull() && !stage.Label.IsUnknown() {
				pending[stage.Label.ValueString()] = key
			}
		}
	}
	vals := map[string]attr.Value{}
	stageType := types.ObjectType{AttrTypes: map[string]attr.Type{"id": types.StringType, "label": types.StringType, "display_order": types.Int64Type, "metadata": types.MapType{ElemType: types.StringType}, "write_permissions": types.StringType}}
	for _, stage := range p.Stages {
		key := stage.ID
		if priorKey, ok := known[stage.ID]; ok {
			key = priorKey
		} else if plannedKey, ok := pending[stage.Label]; ok {
			key = plannedKey
		}
		vals[key] = stageObject(stage, stageType)
	}
	return pipelineModel{ID: types.StringValue(objectType + "/" + p.ID), ObjectType: types.StringValue(objectType), Label: types.StringValue(p.Label), DisplayOrder: types.Int64Value(p.DisplayOrder), Stages: types.MapValueMust(types.ObjectType{AttrTypes: stageType.AttrTypes}, vals)}
}

func (r *PipelineResource) preservePipelineAppendOrders(ctx context.Context, objectType string, pipeline hubspot.Pipeline, configured pipelineModel, model *pipelineModel) error {
	if !configured.DisplayOrder.IsNull() && !configured.DisplayOrder.IsUnknown() && configured.DisplayOrder.ValueInt64() == -1 {
		pipelines, err := r.client.List(ctx, objectType)
		if err != nil {
			return err
		}
		lastOrder := pipeline.DisplayOrder
		for _, candidate := range pipelines {
			if !candidate.Archived && candidate.DisplayOrder > lastOrder {
				lastOrder = candidate.DisplayOrder
			}
		}
		if pipeline.DisplayOrder == lastOrder {
			model.DisplayOrder = types.Int64Value(-1)
		}
	}
	preservePipelineStageAppendOrders(ctx, model, configured.Stages)
	return nil
}

func preservePipelineStageAppendOrders(ctx context.Context, model *pipelineModel, configured types.Map) {
	if model.Stages.IsNull() || model.Stages.IsUnknown() || configured.IsNull() || configured.IsUnknown() {
		return
	}
	var remote, planned map[string]pipelineStageModel
	if diagnostics := model.Stages.ElementsAs(ctx, &remote, false); diagnostics.HasError() {
		return
	}
	if diagnostics := configured.ElementsAs(ctx, &planned, false); diagnostics.HasError() {
		return
	}
	sentinelKeys := make([]string, 0, len(planned))
	for key, stage := range planned {
		if !stage.DisplayOrder.IsNull() && !stage.DisplayOrder.IsUnknown() && stage.DisplayOrder.ValueInt64() == -1 {
			sentinelKeys = append(sentinelKeys, key)
		}
	}
	if len(sentinelKeys) == 0 {
		return
	}
	sort.Strings(sentinelKeys)
	remoteKeys := make([]string, 0, len(remote))
	for key := range remote {
		remoteKeys = append(remoteKeys, key)
	}
	sort.Slice(remoteKeys, func(left, right int) bool {
		leftOrder := remote[remoteKeys[left]].DisplayOrder.ValueInt64()
		rightOrder := remote[remoteKeys[right]].DisplayOrder.ValueInt64()
		if leftOrder == rightOrder {
			return remoteKeys[left] < remoteKeys[right]
		}
		return leftOrder < rightOrder
	})
	if len(sentinelKeys) > len(remoteKeys) || strings.Join(remoteKeys[len(remoteKeys)-len(sentinelKeys):], "\x00") != strings.Join(sentinelKeys, "\x00") {
		return
	}
	for key, stage := range remote {
		if plannedStage, ok := planned[key]; ok && !plannedStage.DisplayOrder.IsNull() && !plannedStage.DisplayOrder.IsUnknown() && plannedStage.DisplayOrder.ValueInt64() == -1 {
			stage.DisplayOrder = types.Int64Value(-1)
			remote[key] = stage
		}
	}
	model.Stages = pipelineStageMap(remote)
}

func pipelineStageMap(stages map[string]pipelineStageModel) types.Map {
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

func pipelineRemoteID(objectType, stateID string) string {
	prefix := objectType + "/"
	if strings.HasPrefix(stateID, prefix) {
		return strings.TrimPrefix(stateID, prefix)
	}
	return stateID
}

func stageObject(s hubspot.PipelineStage, typ types.ObjectType) attr.Value {
	metadata := map[string]attr.Value{}
	for k, v := range s.Metadata {
		metadata[k] = types.StringValue(v)
	}
	return types.ObjectValueMust(typ.AttrTypes, map[string]attr.Value{"id": types.StringValue(s.ID), "label": types.StringValue(s.Label), "display_order": types.Int64Value(s.DisplayOrder), "metadata": types.MapValueMust(types.StringType, metadata), "write_permissions": types.StringValue(s.WritePermissions)})
}
