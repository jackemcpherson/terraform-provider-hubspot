// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
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

type PropertyGroupResource struct {
	client *hubspot.PropertyGroupClient
}

type propertyGroupModel struct {
	ID           types.String `tfsdk:"id"`
	ObjectType   types.String `tfsdk:"object_type"`
	Name         types.String `tfsdk:"name"`
	Label        types.String `tfsdk:"label"`
	DisplayOrder types.Int64  `tfsdk:"display_order"`
}

func NewPropertyGroupResource() resource.Resource {
	return &PropertyGroupResource{}
}

func (r *PropertyGroupResource) Metadata(_ context.Context, _ resource.MetadataRequest, response *resource.MetadataResponse) {
	response.TypeName = "hubspot_property_group"
}

func (r *PropertyGroupResource) Schema(_ context.Context, _ resource.SchemaRequest, response *resource.SchemaResponse) {
	response.Schema = schema.Schema{
		Version:     1,
		Description: "Manages one HubSpot CRM property group for an exact CRM object type.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				Description:         "Canonical object_type/name identity.",
				MarkdownDescription: "Canonical `object_type/name` identity.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"object_type": schema.StringAttribute{
				Required:            true,
				Description:         "Exact HubSpot CRM object type API identifier.",
				MarkdownDescription: "Exact HubSpot CRM object type API identifier, such as `contacts`.",
				Validators:          []validator.String{identifierValidator{kind: "CRM object type"}},
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"name": schema.StringAttribute{
				Required:            true,
				Description:         "Immutable property group internal name.",
				MarkdownDescription: "Immutable property group internal name.",
				Validators:          []validator.String{identifierValidator{kind: "property group name"}},
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"label": schema.StringAttribute{
				Required:            true,
				Description:         "Display label for the property group.",
				MarkdownDescription: "Display label for the property group.",
			},
			"display_order": schema.Int64Attribute{
				Optional:            true,
				Computed:            true,
				Description:         "HubSpot display order; -1 places the group after positive values.",
				MarkdownDescription: "HubSpot display order; `-1` places the group after positive values.",
				Default:             int64default.StaticInt64(-1),
			},
		},
	}
}

func (r *PropertyGroupResource) UpgradeState(context.Context) map[int64]resource.StateUpgrader {
	return map[int64]resource.StateUpgrader{0: identityStateUpgrade()}
}

func (r *PropertyGroupResource) Configure(_ context.Context, request resource.ConfigureRequest, response *resource.ConfigureResponse) {
	if request.ProviderData == nil {
		return
	}
	clients, ok := request.ProviderData.(*hubspot.ClientSet)
	if !ok || clients == nil || clients.PropertyGroups == nil {
		response.Diagnostics.AddError("Provider is not configured", "The HubSpot client set was not available to hubspot_property_group.")
		return
	}
	r.client = clients.PropertyGroups
}

func (r *PropertyGroupResource) Create(ctx context.Context, request resource.CreateRequest, response *resource.CreateResponse) {
	var plan propertyGroupModel
	response.Diagnostics.Append(request.Plan.Get(ctx, &plan)...)
	if response.Diagnostics.HasError() {
		return
	}

	_, err := r.client.Create(ctx, plan.ObjectType.ValueString(), hubspot.PropertyGroupCreate{
		Name:         plan.Name.ValueString(),
		Label:        plan.Label.ValueString(),
		DisplayOrder: requestedDisplayOrder(plan.DisplayOrder),
	})
	if err != nil {
		if isAmbiguous(err) {
			if recovered, recoveryErr := r.client.Get(ctx, plan.ObjectType.ValueString(), plan.Name.ValueString()); recoveryErr == nil && propertyGroupMatchesPlan(recovered, plan) {
				response.Diagnostics.AddWarning("Create response was ambiguous", "HubSpot did not confirm creation, but a property group with the exact configured identity was found and adopted after read-back.")
				model, modelErr := r.modelFromGroupWithOrder(ctx, plan.ObjectType.ValueString(), recovered, plan.DisplayOrder)
				if modelErr != nil {
					appendHubSpotDiagnostic(&response.Diagnostics, "Property group append-order verification failed", modelErr)
					return
				}
				response.Diagnostics.Append(response.State.Set(ctx, model)...)
				return
			}
		}
		appendHubSpotDiagnostic(&response.Diagnostics, "Property group creation failed", err)
		return
	}
	verified, verifyErr := r.client.Get(ctx, plan.ObjectType.ValueString(), plan.Name.ValueString())
	if verifyErr != nil {
		appendHubSpotDiagnostic(&response.Diagnostics, "Property group creation verification failed", verifyErr)
		return
	}
	if verified.Name != plan.Name.ValueString() || !propertyGroupMatchesPlan(verified, plan) {
		response.Diagnostics.AddError("Property group identity mismatch", "HubSpot returned a property group identity different from the configured immutable name.")
		return
	}
	model, modelErr := r.modelFromGroupWithOrder(ctx, plan.ObjectType.ValueString(), verified, plan.DisplayOrder)
	if modelErr != nil {
		appendHubSpotDiagnostic(&response.Diagnostics, "Property group append-order verification failed", modelErr)
		return
	}
	response.Diagnostics.Append(response.State.Set(ctx, model)...)
}

func (r *PropertyGroupResource) Read(ctx context.Context, request resource.ReadRequest, response *resource.ReadResponse) {
	var state propertyGroupModel
	response.Diagnostics.Append(request.State.Get(ctx, &state)...)
	if response.Diagnostics.HasError() {
		return
	}
	group, err := r.client.Get(ctx, state.ObjectType.ValueString(), state.Name.ValueString())
	if err != nil {
		if isNotFound(err) {
			response.State.RemoveResource(ctx)
			return
		}
		appendHubSpotDiagnostic(&response.Diagnostics, "Property group refresh failed", err)
		return
	}
	if group.Archived {
		response.State.RemoveResource(ctx)
		return
	}
	model, modelErr := r.modelFromGroupWithOrder(ctx, state.ObjectType.ValueString(), group, state.DisplayOrder)
	if modelErr != nil {
		appendHubSpotDiagnostic(&response.Diagnostics, "Property group append-order refresh failed", modelErr)
		return
	}
	response.Diagnostics.Append(response.State.Set(ctx, model)...)
}

func (r *PropertyGroupResource) Update(ctx context.Context, request resource.UpdateRequest, response *resource.UpdateResponse) {
	var plan propertyGroupModel
	response.Diagnostics.Append(request.Plan.Get(ctx, &plan)...)
	if response.Diagnostics.HasError() {
		return
	}
	group, err := r.client.Update(ctx, plan.ObjectType.ValueString(), plan.Name.ValueString(), hubspot.PropertyGroupUpdate{
		Label:        plan.Label.ValueString(),
		DisplayOrder: requestedDisplayOrder(plan.DisplayOrder),
	})
	if err != nil {
		if recovered, recoveryErr := r.client.Get(ctx, plan.ObjectType.ValueString(), plan.Name.ValueString()); recoveryErr == nil && propertyGroupMatchesPlan(recovered, plan) {
			response.Diagnostics.AddWarning("Update response was ambiguous", "HubSpot did not confirm the update, but a verified read-back supplied the current property group state.")
			model, modelErr := r.modelFromGroupWithOrder(ctx, plan.ObjectType.ValueString(), recovered, plan.DisplayOrder)
			if modelErr != nil {
				appendHubSpotDiagnostic(&response.Diagnostics, "Property group append-order verification failed", modelErr)
				return
			}
			response.Diagnostics.Append(response.State.Set(ctx, model)...)
			return
		}
		appendHubSpotDiagnostic(&response.Diagnostics, "Property group update failed", err)
		return
	}
	verified, verifyErr := r.client.Get(ctx, plan.ObjectType.ValueString(), plan.Name.ValueString())
	if verifyErr != nil {
		appendHubSpotDiagnostic(&response.Diagnostics, "Property group update verification failed", verifyErr)
		return
	}
	if group.Name != verified.Name || !propertyGroupMatchesPlan(verified, plan) {
		response.Diagnostics.AddError("Property group identity mismatch", "HubSpot returned an update identity different from the configured immutable name.")
		return
	}
	model, modelErr := r.modelFromGroupWithOrder(ctx, plan.ObjectType.ValueString(), verified, plan.DisplayOrder)
	if modelErr != nil {
		appendHubSpotDiagnostic(&response.Diagnostics, "Property group append-order verification failed", modelErr)
		return
	}
	response.Diagnostics.Append(response.State.Set(ctx, model)...)
}

func (r *PropertyGroupResource) Delete(ctx context.Context, request resource.DeleteRequest, response *resource.DeleteResponse) {
	var state propertyGroupModel
	response.Diagnostics.Append(request.State.Get(ctx, &state)...)
	if response.Diagnostics.HasError() {
		return
	}
	if err := r.client.Archive(ctx, state.ObjectType.ValueString(), state.Name.ValueString()); err != nil {
		if isNotFound(err) {
			response.State.RemoveResource(ctx)
			return
		}
		if verified, verifyErr := r.client.Get(ctx, state.ObjectType.ValueString(), state.Name.ValueString()); verifyErr == nil && verified.Archived {
			response.Diagnostics.AddWarning("Archive response was ambiguous", "HubSpot returned an archive error, but a verified read-back found the property group archived.")
			response.State.RemoveResource(ctx)
			return
		}
		appendHubSpotDiagnostic(&response.Diagnostics, "Property group archival failed", err)
		return
	}
	archived, verifyErr := r.client.Get(ctx, state.ObjectType.ValueString(), state.Name.ValueString())
	if verifyErr != nil {
		if isNotFound(verifyErr) {
			response.State.RemoveResource(ctx)
			return
		}
		appendHubSpotDiagnostic(&response.Diagnostics, "Property group archival verification failed", verifyErr)
		return
	}
	if !archived.Archived {
		response.Diagnostics.AddError("Property group archival was not verified", "HubSpot returned the group as active after the archive request; state was retained for retry.")
		return
	}
	response.State.RemoveResource(ctx)
}

func (r *PropertyGroupResource) ImportState(ctx context.Context, request resource.ImportStateRequest, response *resource.ImportStateResponse) {
	parts := strings.Split(request.ID, "/")
	if len(parts) != 2 || !validImportPart(parts[0]) || !validImportPart(parts[1]) {
		response.Diagnostics.AddAttributeError(path.Root("id"), "Invalid property group import ID", "Use the exact object_type/group_name form with exactly one slash.")
		return
	}
	model := propertyGroupModel{
		ID:           types.StringValue(request.ID),
		ObjectType:   types.StringValue(parts[0]),
		Name:         types.StringValue(parts[1]),
		Label:        types.StringNull(),
		DisplayOrder: types.Int64Null(),
	}
	response.Diagnostics.Append(response.State.Set(ctx, &model)...)
}

func validImportPart(value string) bool {
	return value != "" && value == strings.TrimSpace(value) && !strings.ContainsAny(value, "/?#")
}

type identifierValidator struct {
	kind string
}

func (v identifierValidator) Description(context.Context) string {
	return fmt.Sprintf("must be a non-empty %s without whitespace padding or path separators", v.kind)
}

func (v identifierValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v identifierValidator) ValidateString(_ context.Context, request validator.StringRequest, response *validator.StringResponse) {
	if request.ConfigValue.IsNull() || request.ConfigValue.IsUnknown() {
		return
	}
	value := request.ConfigValue.ValueString()
	if value == "" || value != strings.TrimSpace(value) || strings.ContainsAny(value, "/?#") {
		response.Diagnostics.AddAttributeError(request.Path, "Invalid "+v.kind, "Use a non-empty exact identifier without surrounding whitespace or path separators.")
	}
}

func modelFromGroup(objectType string, group hubspot.PropertyGroup) propertyGroupModel {
	return propertyGroupModel{
		ID:           types.StringValue(objectType + "/" + group.Name),
		ObjectType:   types.StringValue(objectType),
		Name:         types.StringValue(group.Name),
		Label:        types.StringValue(group.Label),
		DisplayOrder: types.Int64Value(group.DisplayOrder),
	}
}

func (r *PropertyGroupResource) modelFromGroupWithOrder(ctx context.Context, objectType string, group hubspot.PropertyGroup, configured types.Int64) (propertyGroupModel, error) {
	model := modelFromGroup(objectType, group)
	if !configured.IsNull() && !configured.IsUnknown() && configured.ValueInt64() == -1 {
		groups, err := r.client.List(ctx, objectType)
		if err != nil {
			return propertyGroupModel{}, err
		}
		lastOrder := group.DisplayOrder
		for _, candidate := range groups {
			if !candidate.Archived && candidate.DisplayOrder > lastOrder {
				lastOrder = candidate.DisplayOrder
			}
		}
		if group.DisplayOrder == lastOrder {
			model.DisplayOrder = types.Int64Value(-1)
		}
	}
	return model, nil
}

func propertyGroupMatchesPlan(group hubspot.PropertyGroup, plan propertyGroupModel) bool {
	displayOrderMatches := plan.DisplayOrder.IsNull() || plan.DisplayOrder.IsUnknown() || plan.DisplayOrder.ValueInt64() == -1 || group.DisplayOrder == plan.DisplayOrder.ValueInt64()
	return !group.Archived && group.Name == plan.Name.ValueString() && group.Label == plan.Label.ValueString() && displayOrderMatches
}

func requestedDisplayOrder(value types.Int64) int64 {
	if value.IsNull() || value.IsUnknown() {
		return -1
	}
	return value.ValueInt64()
}

func appendHubSpotDiagnostic(diagnostics *diag.Diagnostics, summary string, err error) {
	var apiError *hubspot.Error
	if errors.As(err, &apiError) {
		detail := fmt.Sprintf("HubSpot returned HTTP %d", apiError.Status)
		if apiError.Category != "" {
			detail += " (" + apiError.Category + ")"
		}
		if apiError.SubCategory != "" {
			detail += " [" + apiError.SubCategory + "]"
		}
		diagnostics.AddError(summary, detail+". State was retained; inspect scopes, account access, and the next plan.")
		return
	}
	diagnostics.AddError(summary, "The provider could not verify the HubSpot operation. State was retained; retry after correcting the reported configuration or account access.")
}

func isNotFound(err error) bool {
	var apiError *hubspot.Error
	return errors.As(err, &apiError) && apiError.Status == http.StatusNotFound
}
