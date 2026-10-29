// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/jackemcpherson/terraform-provider-hubspot/internal/hubspot"
)

const contactSegmentDeletedPrivateKey = "contact_segment_deleted"

var (
	contactSegmentDefaultPollInterval = 2 * time.Second
	contactSegmentDefaultPollTimeout  = 5 * time.Minute
)

var (
	_ resource.ResourceWithImportState    = (*ContactSegmentResource)(nil)
	_ resource.ResourceWithModifyPlan     = (*ContactSegmentResource)(nil)
	_ resource.ResourceWithValidateConfig = (*ContactSegmentResource)(nil)
)

var contactSegmentFilterObjectType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"property":      types.StringType,
	"operator":      types.StringType,
	"property_kind": types.StringType,
	"value":         types.StringType,
}}

var contactSegmentGroupObjectType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"filters": types.SetType{ElemType: contactSegmentFilterObjectType},
}}

// ContactSegmentResource manages one Contact segment definition by its ILS
// list ID. Contact memberships and CRM record values are deliberately outside
// this resource boundary.
type ContactSegmentResource struct {
	client       *hubspot.ContactSegmentClient
	pollInterval time.Duration
	pollTimeout  time.Duration
}

type contactSegmentResourceModel struct {
	ID             types.String `tfsdk:"id"`
	Name           types.String `tfsdk:"name"`
	ProcessingType types.String `tfsdk:"processing_type"`
	FilterGroups   types.Set    `tfsdk:"filter_groups"`
}

type contactSegmentGroupModel struct {
	Filters types.Set `tfsdk:"filters"`
}

type contactSegmentFilterModel struct {
	Property     types.String `tfsdk:"property"`
	Operator     types.String `tfsdk:"operator"`
	PropertyKind types.String `tfsdk:"property_kind"`
	Value        types.String `tfsdk:"value"`
}

func NewContactSegmentResource() resource.Resource {
	return &ContactSegmentResource{pollInterval: contactSegmentDefaultPollInterval, pollTimeout: contactSegmentDefaultPollTimeout}
}

func (r *ContactSegmentResource) Metadata(_ context.Context, _ resource.MetadataRequest, response *resource.MetadataResponse) {
	response.TypeName = "hubspot_contact_segment"
}

func (r *ContactSegmentResource) Schema(_ context.Context, _ resource.SchemaRequest, response *resource.SchemaResponse) {
	response.Schema = schema.Schema{
		Version:             0,
		Description:         "Manages one HubSpot Contact segment definition by generated ILS list ID.",
		MarkdownDescription: "Manages one HubSpot Contact segment definition by generated ILS list `id`. Contact memberships are not managed.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				Description:         "Canonical positive-decimal ILS list ID and sole state and import identity.",
				MarkdownDescription: "Canonical positive-decimal ILS list `id` and sole state and import identity.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Required:            true,
				Description:         "Nonblank mutable Contact segment name. HubSpot enforces remote uniqueness.",
				MarkdownDescription: "Nonblank mutable Contact segment name. HubSpot enforces remote uniqueness; the provider never searches or adopts by name.",
			},
			"processing_type": schema.StringAttribute{
				Required:            true,
				Description:         "Processing type: manual, dynamic, or snapshot. Changes replace the definition.",
				MarkdownDescription: "Processing type: `manual`, `dynamic`, or `snapshot`. Changes replace the definition.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"filter_groups": schema.SetNestedAttribute{
				Optional:            true,
				Description:         "Unordered OR branches, each containing an unordered AND set of Contact property filters.",
				MarkdownDescription: "Unordered OR branches, each containing an unordered AND set of Contact property filters. Required for `dynamic` and `snapshot`; forbidden for `manual`.",
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"filters": schema.SetNestedAttribute{
						Required:            true,
						Description:         "Unordered AND predicates in this OR branch.",
						MarkdownDescription: "Unordered AND predicates in this OR branch.",
						NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
							"property": schema.StringAttribute{Required: true, Description: "Exact internal Contact property name.", MarkdownDescription: "Exact internal Contact property name."},
							"operator": schema.StringAttribute{Required: true, Description: "Supported equality or presence operator.", MarkdownDescription: "Supported equality or presence operator."},
							"property_kind": schema.StringAttribute{
								Optional:            true,
								Description:         "Declared property kind, text or select, for equality operators.",
								MarkdownDescription: "Declared property kind, `text` or `select`, required for equality operators and forbidden for presence operators.",
							},
							"value": schema.StringAttribute{
								Optional:            true,
								Description:         "One nonblank equality value; forbidden for presence operators.",
								MarkdownDescription: "One nonblank equality value; forbidden for presence operators.",
							},
						}},
					},
				}},
			},
		},
	}
}

func (r *ContactSegmentResource) Configure(_ context.Context, request resource.ConfigureRequest, response *resource.ConfigureResponse) {
	if request.ProviderData == nil {
		return
	}
	clients, ok := request.ProviderData.(*hubspot.ClientSet)
	if !ok || clients == nil || clients.ContactSegments == nil {
		response.Diagnostics.AddError("Provider is not configured", "The HubSpot Lists 2026-03 client was not available to hubspot_contact_segment.")
		return
	}
	r.client = clients.ContactSegments
}

func (r *ContactSegmentResource) ValidateConfig(ctx context.Context, request resource.ValidateConfigRequest, response *resource.ValidateConfigResponse) {
	var config contactSegmentResourceModel
	response.Diagnostics.Append(request.Config.Get(ctx, &config)...)
	if response.Diagnostics.HasError() {
		return
	}
	_, diagnostics := contactSegmentWriteFromModel(ctx, config, true)
	response.Diagnostics.Append(diagnostics...)
}

func (r *ContactSegmentResource) ModifyPlan(ctx context.Context, request resource.ModifyPlanRequest, response *resource.ModifyPlanResponse) {
	if request.Plan.Raw.IsNull() {
		return
	}
	var plan contactSegmentResourceModel
	response.Diagnostics.Append(request.Plan.Get(ctx, &plan)...)
	if response.Diagnostics.HasError() {
		return
	}
	if !plan.ProcessingType.IsUnknown() && plan.ProcessingType.ValueString() == "snapshot" && !request.State.Raw.IsNull() {
		var state contactSegmentResourceModel
		response.Diagnostics.Append(request.State.Get(ctx, &state)...)
		if !response.Diagnostics.HasError() && !plan.FilterGroups.IsUnknown() && !state.FilterGroups.IsUnknown() && !plan.FilterGroups.Equal(state.FilterGroups) {
			response.RequiresReplace = append(response.RequiresReplace, path.Root("filter_groups"))
		}
	}
	deleted, diagnostics := request.Private.GetKey(ctx, contactSegmentDeletedPrivateKey)
	response.Diagnostics.Append(diagnostics...)
	if !response.Diagnostics.HasError() && string(deleted) == "true" {
		response.Diagnostics.Append(response.Plan.SetAttribute(ctx, path.Root("id"), types.StringUnknown())...)
	}
}

func (r *ContactSegmentResource) Create(ctx context.Context, request resource.CreateRequest, response *resource.CreateResponse) {
	var plan contactSegmentResourceModel
	response.Diagnostics.Append(request.Plan.Get(ctx, &plan)...)
	if response.Diagnostics.HasError() {
		return
	}
	input, diagnostics := contactSegmentWriteFromModel(ctx, plan, false)
	response.Diagnostics.Append(diagnostics...)
	if response.Diagnostics.HasError() {
		return
	}
	created, createErr := r.client.Create(ctx, input)
	if !validContactSegmentImportID(created.ID) {
		if created.ID != "" {
			response.Diagnostics.AddError("Contact segment creation identity is invalid", "HubSpot returned a non-canonical ILS list ID. The provider did not search by name. Inspect Lists in HubSpot and import the exact positive-decimal ID if creation succeeded.")
		} else if createErr != nil && !isAmbiguous(createErr) {
			appendHubSpotDiagnostic(&response.Diagnostics, "Contact segment creation failed", createErr)
		} else {
			response.Diagnostics.AddError("Contact segment creation outcome is unknown", "HubSpot did not return an ILS list ID. Inspect Lists in HubSpot and import the exact positive-decimal ID if creation succeeded. The provider did not search by name.")
		}
		return
	}
	recovery := plan
	recovery.ID = types.StringValue(created.ID)
	response.Diagnostics.Append(response.State.Set(ctx, &recovery)...)
	if response.Diagnostics.HasError() {
		return
	}
	if createErr != nil && !isAmbiguous(createErr) {
		appendHubSpotDiagnostic(&response.Diagnostics, "Contact segment creation failed", createErr)
		return
	}
	verified, verifyErr := r.waitForActiveDefinition(ctx, created.ID, created.Version, input)
	if verifyErr != nil {
		if createErr != nil {
			appendHubSpotDiagnostic(&response.Diagnostics, "Contact segment creation failed", createErr)
		} else {
			appendHubSpotDiagnostic(&response.Diagnostics, "Contact segment creation verification failed", verifyErr)
		}
		return
	}
	if createErr != nil {
		response.Diagnostics.AddWarning("Contact segment create response was ambiguous", "Exact-ID read-back verified the generated identity and complete desired definition.")
	}
	model := contactSegmentModelFromRemote(ctx, verified, &response.Diagnostics)
	if response.Diagnostics.HasError() {
		return
	}
	response.Diagnostics.Append(response.State.Set(ctx, &model)...)
}

func (r *ContactSegmentResource) Read(ctx context.Context, request resource.ReadRequest, response *resource.ReadResponse) {
	var state contactSegmentResourceModel
	response.Diagnostics.Append(request.State.Get(ctx, &state)...)
	if response.Diagnostics.HasError() {
		return
	}
	out, err := r.client.Get(ctx, state.ID.ValueString())
	if err != nil {
		if isNotFound(err) {
			response.State.RemoveResource(ctx)
			return
		}
		appendHubSpotDiagnostic(&response.Diagnostics, "Contact segment refresh failed", err)
		return
	}
	model := contactSegmentModelFromRemote(ctx, out, &response.Diagnostics)
	if response.Diagnostics.HasError() {
		return
	}
	response.Diagnostics.Append(response.State.Set(ctx, &model)...)
	if out.Deleted {
		response.Diagnostics.Append(response.Private.SetKey(ctx, contactSegmentDeletedPrivateKey, []byte("true"))...)
	} else {
		response.Diagnostics.Append(response.Private.SetKey(ctx, contactSegmentDeletedPrivateKey, nil)...)
	}
}

func (r *ContactSegmentResource) Update(ctx context.Context, request resource.UpdateRequest, response *resource.UpdateResponse) {
	var state, plan contactSegmentResourceModel
	response.Diagnostics.Append(request.State.Get(ctx, &state)...)
	response.Diagnostics.Append(request.Plan.Get(ctx, &plan)...)
	if response.Diagnostics.HasError() {
		return
	}
	input, diagnostics := contactSegmentWriteFromModel(ctx, plan, false)
	response.Diagnostics.Append(diagnostics...)
	if response.Diagnostics.HasError() {
		return
	}
	id := state.ID.ValueString()
	if !validContactSegmentImportID(id) {
		response.Diagnostics.AddError("Contact segment identity is missing", "The canonical positive-decimal ILS list ID was absent from state. No write was sent.")
		return
	}
	current, err := r.client.Get(ctx, id)
	if err != nil {
		appendHubSpotDiagnostic(&response.Diagnostics, "Contact segment update preflight failed", err)
		return
	}
	if current.ID != id || current.ProcessingType != input.ProcessingType {
		response.Diagnostics.AddError("Contact segment update identity changed", "HubSpot did not return the exact ILS list identity and immutable processing type. Prior state was retained.")
		return
	}
	deleted, privateDiagnostics := request.Private.GetKey(ctx, contactSegmentDeletedPrivateKey)
	response.Diagnostics.Append(privateDiagnostics...)
	if response.Diagnostics.HasError() {
		return
	}
	ambiguous := false
	if current.Deleted || string(deleted) == "true" {
		restoreErr := r.client.Restore(ctx, id)
		if restoreErr != nil && !isAmbiguous(restoreErr) {
			appendHubSpotDiagnostic(&response.Diagnostics, "Contact segment restoration failed", restoreErr)
			return
		}
		restored, verifyErr := r.waitForActive(ctx, id, current.Version+1)
		if verifyErr != nil {
			if restoreErr != nil {
				appendHubSpotDiagnostic(&response.Diagnostics, "Contact segment restoration failed", restoreErr)
			} else {
				appendHubSpotDiagnostic(&response.Diagnostics, "Contact segment restoration verification failed", verifyErr)
			}
			return
		}
		if restoreErr != nil {
			ambiguous = true
		}
		current = restored
		response.Diagnostics.Append(response.Private.SetKey(ctx, contactSegmentDeletedPrivateKey, nil)...)
	}
	if current.Name != input.Name {
		_, updateErr := r.client.UpdateName(ctx, id, input.Name)
		if updateErr != nil && !isAmbiguous(updateErr) {
			appendHubSpotDiagnostic(&response.Diagnostics, "Contact segment name update failed", updateErr)
			return
		}
		verified, verifyErr := r.readMatchingDefinition(ctx, id, input, false)
		if verifyErr != nil {
			if updateErr != nil {
				appendHubSpotDiagnostic(&response.Diagnostics, "Contact segment name update failed", updateErr)
			} else {
				appendHubSpotDiagnostic(&response.Diagnostics, "Contact segment name update verification failed", verifyErr)
			}
			return
		}
		if updateErr != nil {
			ambiguous = true
		}
		current = verified
	}
	if input.ProcessingType == "dynamic" && !contactSegmentGroupsEqual(current.FilterGroups, input.FilterGroups) {
		minimumVersion := current.Version + 1
		_, updateErr := r.client.UpdateFilters(ctx, id, input.FilterGroups)
		if updateErr != nil && !isAmbiguous(updateErr) {
			appendHubSpotDiagnostic(&response.Diagnostics, "Contact segment filter update failed", updateErr)
			return
		}
		verified, verifyErr := r.waitForActiveDefinition(ctx, id, minimumVersion, input)
		if verifyErr != nil {
			if updateErr != nil {
				appendHubSpotDiagnostic(&response.Diagnostics, "Contact segment filter update failed", updateErr)
			} else {
				appendHubSpotDiagnostic(&response.Diagnostics, "Contact segment filter update verification failed", verifyErr)
			}
			return
		}
		if updateErr != nil {
			ambiguous = true
		}
		current = verified
	}
	if !contactSegmentMatchesWrite(current, input) || current.Deleted {
		response.Diagnostics.AddError("Contact segment update was not verified", "Exact-ID read-back did not match every desired definition field. Prior state was retained.")
		return
	}
	if ambiguous {
		response.Diagnostics.AddWarning("Contact segment write response was ambiguous", "Exact-ID read-back verified the same active identity, current listVersion, and complete desired definition.")
	}
	model := contactSegmentModelFromRemote(ctx, current, &response.Diagnostics)
	if response.Diagnostics.HasError() {
		return
	}
	response.Diagnostics.Append(response.State.Set(ctx, &model)...)
}

func (r *ContactSegmentResource) Delete(ctx context.Context, request resource.DeleteRequest, response *resource.DeleteResponse) {
	var state contactSegmentResourceModel
	response.Diagnostics.Append(request.State.Get(ctx, &state)...)
	if response.Diagnostics.HasError() {
		return
	}
	id := state.ID.ValueString()
	current, err := r.client.Get(ctx, id)
	if err != nil {
		if isNotFound(err) {
			response.State.RemoveResource(ctx)
			return
		}
		appendHubSpotDiagnostic(&response.Diagnostics, "Contact segment deletion preflight failed", err)
		return
	}
	if current.ID != id {
		response.Diagnostics.AddError("Contact segment deletion was not verified", "HubSpot returned a different ILS list identity. Prior state was retained.")
		return
	}
	if current.Deleted {
		response.State.RemoveResource(ctx)
		return
	}
	deleteErr := r.client.Delete(ctx, id)
	if deleteErr != nil && !isNotFound(deleteErr) && !isAmbiguous(deleteErr) {
		appendHubSpotDiagnostic(&response.Diagnostics, "Contact segment deletion failed", deleteErr)
		return
	}
	tombstone, verifyErr := r.waitForTombstone(ctx, id)
	if verifyErr != nil {
		if deleteErr != nil {
			appendHubSpotDiagnostic(&response.Diagnostics, "Contact segment deletion failed", deleteErr)
		} else {
			appendHubSpotDiagnostic(&response.Diagnostics, "Contact segment deletion verification failed", verifyErr)
		}
		return
	}
	if tombstone.ID != id || !tombstone.Deleted {
		response.Diagnostics.AddError("Contact segment deletion was not verified", "HubSpot did not return the same tombstoned ILS list identity. Prior state was retained.")
		return
	}
	if deleteErr != nil {
		response.Diagnostics.AddWarning("Contact segment delete response was ambiguous", "Exact-ID read-back verified the same tombstoned identity.")
	}
	response.State.RemoveResource(ctx)
}

func (r *ContactSegmentResource) ImportState(ctx context.Context, request resource.ImportStateRequest, response *resource.ImportStateResponse) {
	if !validContactSegmentImportID(request.ID) {
		response.Diagnostics.AddAttributeError(path.Root("id"), "Invalid Contact segment import ID", "Use one exact canonical positive-decimal HubSpot ILS list ID. Names and composite identifiers are not accepted.")
		return
	}
	out, err := r.client.Get(ctx, request.ID)
	if err != nil {
		appendHubSpotDiagnostic(&response.Diagnostics, "Contact segment import failed", err)
		return
	}
	model := contactSegmentModelFromRemote(ctx, out, &response.Diagnostics)
	if response.Diagnostics.HasError() {
		return
	}
	response.Diagnostics.Append(response.State.Set(ctx, &model)...)
	if out.Deleted {
		response.Diagnostics.Append(response.Private.SetKey(ctx, contactSegmentDeletedPrivateKey, []byte("true"))...)
	}
}

func contactSegmentWriteFromModel(ctx context.Context, model contactSegmentResourceModel, allowUnknown bool) (hubspot.ContactSegmentWrite, diag.Diagnostics) {
	var diagnostics diag.Diagnostics
	write := hubspot.ContactSegmentWrite{}
	if model.Name.IsUnknown() || model.ProcessingType.IsUnknown() || model.FilterGroups.IsUnknown() {
		if !allowUnknown {
			diagnostics.AddError("Contact segment configuration is unknown", "Name, processing_type, and the complete filter tree must be known before a Contact segment write.")
		}
		return write, diagnostics
	}
	if model.Name.IsNull() || strings.TrimSpace(model.Name.ValueString()) == "" {
		diagnostics.AddAttributeError(path.Root("name"), "Invalid Contact segment name", "Name must contain at least one non-whitespace character.")
	} else {
		write.Name = model.Name.ValueString()
	}
	if model.ProcessingType.IsNull() {
		diagnostics.AddAttributeError(path.Root("processing_type"), "Invalid Contact segment processing type", "Choose manual, dynamic, or snapshot.")
		return write, diagnostics
	}
	write.ProcessingType = model.ProcessingType.ValueString()
	if write.ProcessingType != "manual" && write.ProcessingType != "dynamic" && write.ProcessingType != "snapshot" {
		diagnostics.AddAttributeError(path.Root("processing_type"), "Invalid Contact segment processing type", "Choose manual, dynamic, or snapshot.")
	}
	if model.FilterGroups.IsNull() {
		if write.ProcessingType != "manual" && write.ProcessingType != "" {
			diagnostics.AddAttributeError(path.Root("filter_groups"), "Contact segment filters are required", "Dynamic and snapshot definitions require at least one nonempty filter group.")
		}
		return write, diagnostics
	}
	if write.ProcessingType == "manual" {
		diagnostics.AddAttributeError(path.Root("filter_groups"), "Manual Contact segment cannot have filters", "Omit filter_groups when processing_type is manual.")
		return write, diagnostics
	}
	var groups []contactSegmentGroupModel
	diagnostics.Append(model.FilterGroups.ElementsAs(ctx, &groups, false)...)
	if diagnostics.HasError() {
		return write, diagnostics
	}
	if write.ProcessingType != "manual" && write.ProcessingType != "" && len(groups) == 0 {
		diagnostics.AddAttributeError(path.Root("filter_groups"), "Contact segment filters are required", "Dynamic and snapshot definitions require at least one nonempty filter group.")
	}
	write.FilterGroups = make([]hubspot.ContactSegmentFilterGroup, 0, len(groups))
	for groupIndex, group := range groups {
		if group.Filters.IsNull() || group.Filters.IsUnknown() {
			if group.Filters.IsUnknown() && allowUnknown {
				continue
			}
			diagnostics.AddAttributeError(path.Root("filter_groups"), "Contact segment filter group is incomplete", fmt.Sprintf("Filter group %d must contain at least one filter.", groupIndex+1))
			continue
		}
		var filters []contactSegmentFilterModel
		diagnostics.Append(group.Filters.ElementsAs(ctx, &filters, false)...)
		if len(filters) == 0 {
			diagnostics.AddAttributeError(path.Root("filter_groups"), "Contact segment filter group is empty", fmt.Sprintf("Filter group %d must contain at least one filter.", groupIndex+1))
			continue
		}
		writeGroup := hubspot.ContactSegmentFilterGroup{Filters: make([]hubspot.ContactSegmentFilter, 0, len(filters))}
		for filterIndex, filter := range filters {
			filterPath := path.Root("filter_groups")
			if filter.Property.IsUnknown() || filter.Operator.IsUnknown() || filter.PropertyKind.IsUnknown() || filter.Value.IsUnknown() {
				if !allowUnknown {
					diagnostics.AddAttributeError(filterPath, "Contact segment filter is unknown", fmt.Sprintf("Filter %d in group %d must be completely known before apply.", filterIndex+1, groupIndex+1))
				}
				continue
			}
			if filter.Property.IsNull() || strings.TrimSpace(filter.Property.ValueString()) == "" {
				diagnostics.AddAttributeError(filterPath, "Invalid Contact segment filter property", fmt.Sprintf("Filter %d in group %d requires a nonblank Contact property name.", filterIndex+1, groupIndex+1))
				continue
			}
			semantic := hubspot.ContactSegmentFilter{Property: filter.Property.ValueString()}
			if !filter.Operator.IsNull() {
				semantic.Operator = filter.Operator.ValueString()
			}
			if !filter.PropertyKind.IsNull() {
				semantic.PropertyKind = filter.PropertyKind.ValueString()
			}
			if !filter.Value.IsNull() {
				semantic.Value = filter.Value.ValueString()
			}
			switch semantic.Operator {
			case "is_equal_to", "is_not_equal_to":
				if semantic.PropertyKind != "text" && semantic.PropertyKind != "select" {
					diagnostics.AddAttributeError(filterPath, "Invalid Contact segment equality filter", fmt.Sprintf("Filter %d in group %d requires property_kind text or select.", filterIndex+1, groupIndex+1))
				}
				if strings.TrimSpace(semantic.Value) == "" {
					diagnostics.AddAttributeError(filterPath, "Invalid Contact segment equality filter", fmt.Sprintf("Filter %d in group %d requires one nonblank value.", filterIndex+1, groupIndex+1))
				}
			case "is_known", "is_not_known":
				if semantic.PropertyKind != "" || semantic.Value != "" || !filter.PropertyKind.IsNull() || !filter.Value.IsNull() {
					diagnostics.AddAttributeError(filterPath, "Invalid Contact segment presence filter", fmt.Sprintf("Filter %d in group %d forbids property_kind and value.", filterIndex+1, groupIndex+1))
				}
			default:
				diagnostics.AddAttributeError(filterPath, "Invalid Contact segment filter operator", fmt.Sprintf("Filter %d in group %d must use is_equal_to, is_not_equal_to, is_known, or is_not_known.", filterIndex+1, groupIndex+1))
			}
			writeGroup.Filters = append(writeGroup.Filters, semantic)
		}
		write.FilterGroups = append(write.FilterGroups, writeGroup)
	}
	return write, diagnostics
}

func contactSegmentModelFromRemote(ctx context.Context, segment hubspot.ContactSegment, diagnostics *diag.Diagnostics) contactSegmentResourceModel {
	model := contactSegmentResourceModel{
		ID:             types.StringValue(segment.ID),
		Name:           types.StringValue(segment.Name),
		ProcessingType: types.StringValue(segment.ProcessingType),
		FilterGroups:   types.SetNull(contactSegmentGroupObjectType),
	}
	if len(segment.FilterGroups) == 0 {
		return model
	}
	groups := make([]contactSegmentGroupModel, 0, len(segment.FilterGroups))
	for _, remoteGroup := range segment.FilterGroups {
		filters := make([]contactSegmentFilterModel, 0, len(remoteGroup.Filters))
		for _, remoteFilter := range remoteGroup.Filters {
			kind := types.StringNull()
			value := types.StringNull()
			if remoteFilter.PropertyKind != "" {
				kind = types.StringValue(remoteFilter.PropertyKind)
			}
			if remoteFilter.Value != "" {
				value = types.StringValue(remoteFilter.Value)
			}
			filters = append(filters, contactSegmentFilterModel{
				Property: types.StringValue(remoteFilter.Property), Operator: types.StringValue(remoteFilter.Operator),
				PropertyKind: kind, Value: value,
			})
		}
		filterSet, filterDiagnostics := types.SetValueFrom(ctx, contactSegmentFilterObjectType, filters)
		diagnostics.Append(filterDiagnostics...)
		groups = append(groups, contactSegmentGroupModel{Filters: filterSet})
	}
	groupSet, groupDiagnostics := types.SetValueFrom(ctx, contactSegmentGroupObjectType, groups)
	diagnostics.Append(groupDiagnostics...)
	model.FilterGroups = groupSet
	return model
}

func (r *ContactSegmentResource) readMatchingDefinition(ctx context.Context, id string, desired hubspot.ContactSegmentWrite, includeFilters bool) (hubspot.ContactSegment, error) {
	out, err := r.client.Get(ctx, id)
	if err != nil {
		return hubspot.ContactSegment{}, err
	}
	if out.Deleted || out.ID != id || out.Name != desired.Name || out.ProcessingType != desired.ProcessingType {
		return hubspot.ContactSegment{}, errors.New("exact Contact segment identity or desired fields did not match")
	}
	if includeFilters && !contactSegmentGroupsEqual(out.FilterGroups, desired.FilterGroups) {
		return hubspot.ContactSegment{}, errors.New("contact segment filters did not match")
	}
	return out, nil
}

func (r *ContactSegmentResource) waitForActiveDefinition(ctx context.Context, id string, minimumVersion int64, desired hubspot.ContactSegmentWrite) (hubspot.ContactSegment, error) {
	return r.waitForSegment(ctx, id, func(segment hubspot.ContactSegment) (bool, error) {
		if segment.Deleted || segment.Version < minimumVersion || !contactSegmentMatchesWrite(segment, desired) {
			return false, nil
		}
		return contactSegmentProcessingComplete(segment.ProcessingStatus)
	})
}

func (r *ContactSegmentResource) waitForActive(ctx context.Context, id string, minimumVersion int64) (hubspot.ContactSegment, error) {
	return r.waitForSegment(ctx, id, func(segment hubspot.ContactSegment) (bool, error) {
		if segment.Deleted || segment.Version < minimumVersion {
			return false, nil
		}
		return contactSegmentProcessingComplete(segment.ProcessingStatus)
	})
}

func (r *ContactSegmentResource) waitForTombstone(ctx context.Context, id string) (hubspot.ContactSegment, error) {
	return r.waitForSegment(ctx, id, func(segment hubspot.ContactSegment) (bool, error) {
		return segment.Deleted, nil
	})
}

func (r *ContactSegmentResource) waitForSegment(ctx context.Context, id string, ready func(hubspot.ContactSegment) (bool, error)) (hubspot.ContactSegment, error) {
	interval := r.pollInterval
	if interval <= 0 {
		interval = contactSegmentDefaultPollInterval
	}
	timeout := r.pollTimeout
	if timeout <= 0 {
		timeout = contactSegmentDefaultPollTimeout
	}
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	for {
		segment, err := r.client.Get(ctx, id)
		if err == nil {
			complete, readyErr := ready(segment)
			if readyErr != nil {
				return hubspot.ContactSegment{}, readyErr
			}
			if complete {
				return segment, nil
			}
			if failed, failure := contactSegmentProcessingFailed(segment.ProcessingStatus); failed {
				return hubspot.ContactSegment{}, failure
			}
		} else if !isNotFound(err) {
			return hubspot.ContactSegment{}, err
		}
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return hubspot.ContactSegment{}, ctx.Err()
		case <-deadline.C:
			timer.Stop()
			return hubspot.ContactSegment{}, errors.New("timed out after five minutes waiting for the exact Contact segment identity and current listVersion")
		case <-timer.C:
		}
	}
}

func contactSegmentProcessingComplete(status string) (bool, error) {
	if strings.EqualFold(status, "COMPLETE") {
		return true, nil
	}
	if failed, err := contactSegmentProcessingFailed(status); failed {
		return false, err
	}
	return false, nil
}

func contactSegmentProcessingFailed(status string) (bool, error) {
	switch strings.ToUpper(status) {
	case "FAILED", "ERROR", "CANCELED", "CANCELLED", "REJECTED":
		return true, fmt.Errorf("HubSpot contact segment processing ended with terminal status %s", status)
	default:
		return false, nil
	}
}

func contactSegmentMatchesWrite(segment hubspot.ContactSegment, desired hubspot.ContactSegmentWrite) bool {
	return segment.Name == desired.Name && segment.ProcessingType == desired.ProcessingType &&
		contactSegmentGroupsEqual(segment.FilterGroups, desired.FilterGroups)
}

func contactSegmentGroupsEqual(left, right []hubspot.ContactSegmentFilterGroup) bool {
	if len(left) != len(right) {
		return false
	}
	counts := make(map[string]int, len(left))
	for _, group := range left {
		counts[contactSegmentGroupKey(group)]++
	}
	for _, group := range right {
		key := contactSegmentGroupKey(group)
		if counts[key] == 0 {
			return false
		}
		counts[key]--
	}
	return true
}

func contactSegmentGroupKey(group hubspot.ContactSegmentFilterGroup) string {
	keys := make([]string, 0, len(group.Filters))
	for _, filter := range group.Filters {
		keys = append(keys, strings.Join([]string{filter.Property, filter.Operator, filter.PropertyKind, filter.Value}, "\x00"))
	}
	// Set nesting means order is never semantic. A tiny insertion sort avoids
	// exposing canonicalization concerns outside this resource.
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j] < keys[j-1]; j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
	return strings.Join(keys, "\x01")
}

func validContactSegmentImportID(id string) bool {
	value, ok := new(big.Int).SetString(id, 10)
	return ok && value.Sign() > 0 && value.String() == id
}
