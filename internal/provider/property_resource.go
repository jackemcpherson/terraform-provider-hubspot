package provider

import (
	"context"
	"errors"
	"sort"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/jackemcpherson/terraform-provider-hubspot/internal/hubspot"
)

type PropertyResource struct {
	client *hubspot.PropertyDefinitionClient
}
type propertyOptionModel struct {
	Label        types.String `tfsdk:"label"`
	Description  types.String `tfsdk:"description"`
	DisplayOrder types.Int64  `tfsdk:"display_order"`
	Hidden       types.Bool   `tfsdk:"hidden"`
}
type propertyResourceModel struct {
	ID                   types.String `tfsdk:"id"`
	ObjectType           types.String `tfsdk:"object_type"`
	Name                 types.String `tfsdk:"name"`
	Label                types.String `tfsdk:"label"`
	GroupName            types.String `tfsdk:"group_name"`
	Type                 types.String `tfsdk:"type"`
	FieldType            types.String `tfsdk:"field_type"`
	Description          types.String `tfsdk:"description"`
	DisplayOrder         types.Int64  `tfsdk:"display_order"`
	FormField            types.Bool   `tfsdk:"form_field"`
	Hidden               types.Bool   `tfsdk:"hidden"`
	HasUniqueValue       types.Bool   `tfsdk:"has_unique_value"`
	DataSensitivity      types.String `tfsdk:"data_sensitivity"`
	ExternalOptions      types.Bool   `tfsdk:"external_options"`
	ShowCurrencySymbol   types.Bool   `tfsdk:"show_currency_symbol"`
	CalculationFormula   types.String `tfsdk:"calculation_formula"`
	CurrencyPropertyName types.String `tfsdk:"currency_property_name"`
	NumberDisplayHint    types.String `tfsdk:"number_display_hint"`
	TextDisplayHint      types.String `tfsdk:"text_display_hint"`
	ReferencedObjectType types.String `tfsdk:"referenced_object_type"`
	Options              types.Map    `tfsdk:"options"`
}

func NewPropertyResource() resource.Resource { return &PropertyResource{} }
func (r *PropertyResource) Metadata(_ context.Context, _ resource.MetadataRequest, response *resource.MetadataResponse) {
	response.TypeName = "hubspot_property"
}
func (r *PropertyResource) Schema(_ context.Context, _ resource.SchemaRequest, response *resource.SchemaResponse) {
	response.Schema = schema.Schema{Version: 1, Description: "Manages one ordinary or enumeration HubSpot CRM property definition.", Attributes: map[string]schema.Attribute{
		"id":                     schema.StringAttribute{Computed: true, Description: "Canonical object_type/property_name identity.", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
		"object_type":            schema.StringAttribute{Required: true, Description: "Exact CRM object type; changes replace the definition.", Validators: []validator.String{identifierValidator{kind: "CRM object type"}}, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
		"name":                   schema.StringAttribute{Required: true, Description: "Immutable property name; changes replace the definition.", Validators: []validator.String{identifierValidator{kind: "property name"}}, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
		"label":                  schema.StringAttribute{Required: true, Description: "Property display label."},
		"group_name":             schema.StringAttribute{Required: true, Description: "Internal name of the owning property group."},
		"type":                   schema.StringAttribute{Required: true, Description: "HubSpot storage type. Type changes update in place and can affect existing record values.", Validators: []validator.String{propertyTypeValidator{}}},
		"field_type":             schema.StringAttribute{Required: true, Description: "HubSpot editor field type. Field-type changes update in place and can affect existing record values.", Validators: []validator.String{propertyFieldTypeValidator{}}},
		"description":            schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), Description: "Property description; defaults to an empty string."},
		"display_order":          schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(-1), Description: "HubSpot display order; defaults to -1."},
		"form_field":             schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), Description: "Whether the property can appear in forms; defaults to false."},
		"hidden":                 schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), Description: "Whether HubSpot hides the property; defaults to false."},
		"has_unique_value":       schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), Description: "Whether values must be unique; defaults to false and changes replace the definition.", PlanModifiers: []planmodifier.Bool{boolRequiresReplace{}}},
		"data_sensitivity":       schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("non_sensitive"), Description: "Must be non_sensitive; sensitive property definitions are deferred from v0.1.", Validators: []validator.String{freeTierSensitivityValidator{}}, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
		"external_options":       schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), Description: "Delegates option ownership to HubSpot; defaults to false and changes replace the definition.", PlanModifiers: []planmodifier.Bool{boolRequiresReplace{}}},
		"show_currency_symbol":   schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), Description: "Whether HubSpot shows a currency symbol; defaults to false."},
		"calculation_formula":    schema.StringAttribute{Optional: true, Computed: true, Description: "HubSpot calculation formula; omitted when null.", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
		"currency_property_name": schema.StringAttribute{Optional: true, Computed: true, Description: "Internal name of the currency source property; omitted when null.", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
		"number_display_hint":    schema.StringAttribute{Optional: true, Computed: true, Description: "HubSpot number display hint; omitted when null.", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
		"text_display_hint":      schema.StringAttribute{Optional: true, Computed: true, Description: "HubSpot text display hint; omitted when null.", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
		"referenced_object_type": schema.StringAttribute{Optional: true, Computed: true, Description: "Referenced CRM object type; changes replace the definition.", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown(), stringplanmodifier.RequiresReplace()}},
		"options": schema.MapNestedAttribute{Optional: true, Computed: true, Description: "Complete option set keyed by immutable CRM record value.", PlanModifiers: []planmodifier.Map{mapplanmodifier.UseStateForUnknown()}, NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
			"label":         schema.StringAttribute{Required: true, Description: "Option display label."},
			"description":   schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), Description: "Option description; defaults to an empty string."},
			"display_order": schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(-1), Description: "HubSpot display order; defaults to -1."},
			"hidden":        schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), Description: "Whether HubSpot hides the option; defaults to false."},
		}}},
	}}
}

func (r *PropertyResource) UpgradeState(context.Context) map[int64]resource.StateUpgrader {
	return map[int64]resource.StateUpgrader{0: identityStateUpgrade()}
}
func (r *PropertyResource) Configure(_ context.Context, request resource.ConfigureRequest, response *resource.ConfigureResponse) {
	if request.ProviderData == nil {
		return
	}
	clients, ok := request.ProviderData.(*hubspot.ClientSet)
	if !ok || clients == nil || clients.Properties == nil {
		response.Diagnostics.AddError("Provider is not configured", "The HubSpot property client was not available.")
		return
	}
	r.client = clients.Properties
}
func (r *PropertyResource) ModifyPlan(ctx context.Context, request resource.ModifyPlanRequest, response *resource.ModifyPlanResponse) {
	if request.Plan.Raw.IsNull() {
		return
	}
	var plan propertyResourceModel
	response.Diagnostics.Append(request.Plan.Get(ctx, &plan)...)
	if response.Diagnostics.HasError() {
		return
	}
	if request.State.Raw.IsNull() {
		return
	}
	var old propertyResourceModel
	response.Diagnostics.Append(request.State.Get(ctx, &old)...)
	if response.Diagnostics.HasError() {
		return
	}
	if knownStringChanged(plan.Type, old.Type) || knownStringChanged(plan.FieldType, old.FieldType) {
		response.Diagnostics.AddWarning("Property type transition", "HubSpot may retain CRM record values that no longer match the new definition.")
	}
	if optionKeysChanged(plan.Options, old.Options) {
		response.Diagnostics.AddWarning("Property option values changed", "HubSpot does not migrate existing CRM record values when option keys are removed or renamed.")
	}
}
func (r *PropertyResource) Create(ctx context.Context, request resource.CreateRequest, response *resource.CreateResponse) {
	var plan propertyResourceModel
	response.Diagnostics.Append(request.Plan.Get(ctx, &plan)...)
	if response.Diagnostics.HasError() {
		return
	}
	input, err := propertyWriteFromModel(ctx, plan)
	if err != nil {
		response.Diagnostics.AddError("Invalid property definition", err.Error())
		return
	}
	def, err := r.client.Create(ctx, plan.ObjectType.ValueString(), input)
	if err != nil {
		if isAmbiguous(err) {
			if recovered, recoveryErr := r.client.Get(ctx, plan.ObjectType.ValueString(), plan.Name.ValueString(), false, sensitivityValue(plan.DataSensitivity), ""); recoveryErr == nil && propertyMatchesPlan(recovered, plan) {
				response.Diagnostics.AddWarning("Create response was ambiguous", "An exact active property definition matched the requested state and was adopted after read-back.")
				model, modelErr := r.modelFromPropertyDefinitionWithOrders(ctx, plan.ObjectType.ValueString(), recovered, plan)
				if modelErr != nil {
					appendHubSpotDiagnostic(&response.Diagnostics, "Property append-order verification failed", modelErr)
					return
				}
				response.Diagnostics.Append(response.State.Set(ctx, model)...)
				return
			}
		}
		appendHubSpotDiagnostic(&response.Diagnostics, "Property creation failed", err)
		return
	}
	if def.Name != plan.Name.ValueString() {
		response.Diagnostics.AddError("Property identity mismatch", "HubSpot returned a different immutable property name.")
		return
	}
	verified, verifyErr := r.client.Get(ctx, plan.ObjectType.ValueString(), plan.Name.ValueString(), false, sensitivityValue(plan.DataSensitivity), "")
	if verifyErr != nil {
		appendHubSpotDiagnostic(&response.Diagnostics, "Property creation verification failed", verifyErr)
		return
	}
	if !propertyMatchesPlan(verified, plan) {
		response.Diagnostics.AddError("Property creation was not verified", "HubSpot returned a definition that does not match the requested state.")
		return
	}
	model, modelErr := r.modelFromPropertyDefinitionWithOrders(ctx, plan.ObjectType.ValueString(), verified, plan)
	if modelErr != nil {
		appendHubSpotDiagnostic(&response.Diagnostics, "Property append-order verification failed", modelErr)
		return
	}
	response.Diagnostics.Append(response.State.Set(ctx, model)...)
}
func (r *PropertyResource) Read(ctx context.Context, request resource.ReadRequest, response *resource.ReadResponse) {
	var state propertyResourceModel
	response.Diagnostics.Append(request.State.Get(ctx, &state)...)
	if response.Diagnostics.HasError() {
		return
	}
	def, err := r.client.Get(ctx, state.ObjectType.ValueString(), state.Name.ValueString(), false, sensitivityValue(state.DataSensitivity), "")
	if err != nil {
		if isNotFound(err) {
			response.State.RemoveResource(ctx)
			return
		}
		appendHubSpotDiagnostic(&response.Diagnostics, "Property refresh failed", err)
		return
	}
	if def.Archived != nil && *def.Archived {
		response.State.RemoveResource(ctx)
		return
	}
	if def.HubSpotDefined != nil && *def.HubSpotDefined {
		response.Diagnostics.AddError("HubSpot-defined property is not manageable", "Use hubspot_property_definition for discovery-only definitions.")
		return
	}
	if readOnlyDefinition(def) {
		response.Diagnostics.AddError("Read-only property is not manageable", "Use hubspot_property_definition for discovery-only definitions.")
		return
	}
	model, modelErr := r.modelFromPropertyDefinitionWithOrders(ctx, state.ObjectType.ValueString(), def, state)
	if modelErr != nil {
		appendHubSpotDiagnostic(&response.Diagnostics, "Property append-order refresh failed", modelErr)
		return
	}
	response.Diagnostics.Append(response.State.Set(ctx, model)...)
}
func (r *PropertyResource) Update(ctx context.Context, request resource.UpdateRequest, response *resource.UpdateResponse) {
	var plan propertyResourceModel
	response.Diagnostics.Append(request.Plan.Get(ctx, &plan)...)
	if response.Diagnostics.HasError() {
		return
	}
	input, err := propertyWriteFromModel(ctx, plan)
	if err != nil {
		response.Diagnostics.AddError("Invalid property definition", err.Error())
		return
	}
	_, err = r.client.Update(ctx, plan.ObjectType.ValueString(), plan.Name.ValueString(), input)
	if err != nil {
		if isAmbiguous(err) {
			if recovered, recoveryErr := r.client.Get(ctx, plan.ObjectType.ValueString(), plan.Name.ValueString(), false, sensitivityValue(plan.DataSensitivity), ""); recoveryErr == nil && propertyMatchesPlan(recovered, plan) {
				response.Diagnostics.AddWarning("Update response was ambiguous", "A verified read-back matched the requested property state.")
				model, modelErr := r.modelFromPropertyDefinitionWithOrders(ctx, plan.ObjectType.ValueString(), recovered, plan)
				if modelErr != nil {
					appendHubSpotDiagnostic(&response.Diagnostics, "Property append-order verification failed", modelErr)
					return
				}
				response.Diagnostics.Append(response.State.Set(ctx, model)...)
				return
			}
		}
		appendHubSpotDiagnostic(&response.Diagnostics, "Property update failed", err)
		return
	}
	verified, verifyErr := r.client.Get(ctx, plan.ObjectType.ValueString(), plan.Name.ValueString(), false, sensitivityValue(plan.DataSensitivity), "")
	if verifyErr != nil {
		appendHubSpotDiagnostic(&response.Diagnostics, "Property update verification failed", verifyErr)
		return
	}
	if !propertyMatchesPlan(verified, plan) {
		response.Diagnostics.AddError("Property update was not verified", "HubSpot returned a definition that does not match the requested scalar or option state.")
		return
	}
	model, modelErr := r.modelFromPropertyDefinitionWithOrders(ctx, plan.ObjectType.ValueString(), verified, plan)
	if modelErr != nil {
		appendHubSpotDiagnostic(&response.Diagnostics, "Property append-order verification failed", modelErr)
		return
	}
	response.Diagnostics.Append(response.State.Set(ctx, model)...)
}
func (r *PropertyResource) Delete(ctx context.Context, request resource.DeleteRequest, response *resource.DeleteResponse) {
	var state propertyResourceModel
	response.Diagnostics.Append(request.State.Get(ctx, &state)...)
	if response.Diagnostics.HasError() {
		return
	}
	if err := r.client.Archive(ctx, state.ObjectType.ValueString(), state.Name.ValueString()); err != nil {
		if isNotFound(err) {
			response.State.RemoveResource(ctx)
			return
		}
		appendHubSpotDiagnostic(&response.Diagnostics, "Property archival failed", err)
		return
	}
	def, err := r.client.Get(ctx, state.ObjectType.ValueString(), state.Name.ValueString(), true, sensitivityValue(state.DataSensitivity), "")
	if err != nil {
		if isNotFound(err) {
			response.State.RemoveResource(ctx)
			return
		}
		appendHubSpotDiagnostic(&response.Diagnostics, "Property archival verification failed", err)
		return
	}
	if def.Archived == nil || !*def.Archived {
		response.Diagnostics.AddError("Property archival was not verified", "The property remains active after the archive request; state was retained.")
		return
	}
	response.State.RemoveResource(ctx)
}
func (r *PropertyResource) ImportState(ctx context.Context, request resource.ImportStateRequest, response *resource.ImportStateResponse) {
	parts := strings.Split(request.ID, "/")
	if len(parts) != 2 || !validImportPart(parts[0]) || !validImportPart(parts[1]) {
		response.Diagnostics.AddAttributeError(path.Root("id"), "Invalid property import ID", "Use exact object_type/property_name form with exactly one slash.")
		return
	}
	def, err := r.client.Get(ctx, parts[0], parts[1], false, "non_sensitive", "")
	if err != nil {
		appendHubSpotDiagnostic(&response.Diagnostics, "Property import failed", err)
		return
	}
	if (def.HubSpotDefined != nil && *def.HubSpotDefined) || readOnlyDefinition(def) {
		response.Diagnostics.AddError("Property is discovery-only", "Use hubspot_property_definition for HubSpot-defined or read-only definitions.")
		return
	}
	response.Diagnostics.Append(response.State.Set(ctx, modelFromPropertyDefinition(parts[0], def))...)
}

func propertyWriteFromModel(ctx context.Context, model propertyResourceModel) (hubspot.PropertyWrite, error) {
	var options map[string]propertyOptionModel
	if !model.Options.IsNull() && !model.Options.IsUnknown() {
		if diags := model.Options.ElementsAs(ctx, &options, false); diags.HasError() {
			return hubspot.PropertyWrite{}, errors.New("invalid property options")
		}
	}
	keys := make([]string, 0, len(options))
	for value := range options {
		keys = append(keys, value)
	}
	sort.Strings(keys)
	list := make([]hubspot.PropertyOption, 0, len(options))
	for _, value := range keys {
		opt := options[value]
		list = append(list, hubspot.PropertyOption{Value: value, Label: opt.Label.ValueString(), Description: stringPointer(opt.Description), DisplayOrder: displayOrderPointer(opt.DisplayOrder), Hidden: boolPointer(opt.Hidden)})
	}
	return hubspot.PropertyWrite{Name: model.Name.ValueString(), Label: model.Label.ValueString(), GroupName: model.GroupName.ValueString(), Type: model.Type.ValueString(), FieldType: model.FieldType.ValueString(), Description: stringPointer(model.Description), DisplayOrder: displayOrderPointer(model.DisplayOrder), FormField: boolPointer(model.FormField), Hidden: boolPointer(model.Hidden), HasUniqueValue: boolPointer(model.HasUniqueValue), DataSensitivity: stringPointer(model.DataSensitivity), ExternalOptions: boolPointer(model.ExternalOptions), ShowCurrencySymbol: boolPointer(model.ShowCurrencySymbol), CalculationFormula: stringPointer(model.CalculationFormula), CurrencyPropertyName: stringPointer(model.CurrencyPropertyName), NumberDisplayHint: stringPointer(model.NumberDisplayHint), TextDisplayHint: stringPointer(model.TextDisplayHint), ReferencedObjectType: stringPointer(model.ReferencedObjectType), Options: list}, nil
}
func stringPointer(v types.String) *string {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	s := v.ValueString()
	return &s
}
func displayOrderPointer(v types.Int64) *int64 {
	if v.IsNull() {
		return nil
	}
	n := int64(-1)
	if !v.IsUnknown() {
		n = v.ValueInt64()
	}
	return &n
}
func boolPointer(v types.Bool) *bool {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	b := v.ValueBool()
	return &b
}
func sensitivityValue(v types.String) string {
	if v.IsNull() || v.IsUnknown() || v.ValueString() == "" {
		return "non_sensitive"
	}
	return v.ValueString()
}
func modelFromPropertyDefinition(objectType string, d hubspot.PropertyDefinition) propertyResourceModel {
	m := propertyResourceModel{ID: types.StringValue(objectType + "/" + d.Name), ObjectType: types.StringValue(objectType), Name: types.StringValue(d.Name), Label: types.StringValue(d.Label), GroupName: types.StringValue(d.GroupName), Type: types.StringValue(d.Type), FieldType: types.StringValue(d.FieldType), Description: optionalString(d.Description), DisplayOrder: optionalInt(d.DisplayOrder), FormField: optionalBool(d.FormField), Hidden: optionalBool(d.Hidden), HasUniqueValue: optionalBool(d.HasUniqueValue), DataSensitivity: optionalString(d.DataSensitivity), ExternalOptions: optionalBool(d.ExternalOptions), ShowCurrencySymbol: optionalBool(d.ShowCurrencySymbol), CalculationFormula: optionalString(d.CalculationFormula), CurrencyPropertyName: optionalString(d.CurrencyPropertyName), NumberDisplayHint: optionalString(d.NumberDisplayHint), TextDisplayHint: optionalString(d.TextDisplayHint), ReferencedObjectType: optionalString(d.ReferencedObjectType)}
	vals := map[string]attr.Value{}
	for _, o := range d.Options {
		vals[o.Value] = types.ObjectValueMust(optionAttrTypes(), map[string]attr.Value{"label": types.StringValue(o.Label), "description": optionalString(o.Description), "display_order": optionalInt(o.DisplayOrder), "hidden": optionalBool(o.Hidden)})
	}
	if d.Options != nil {
		m.Options = types.MapValueMust(types.ObjectType{AttrTypes: optionAttrTypes()}, vals)
	} else {
		m.Options = types.MapNull(types.ObjectType{AttrTypes: optionAttrTypes()})
	}
	return m
}

func (r *PropertyResource) modelFromPropertyDefinitionWithOrders(ctx context.Context, objectType string, definition hubspot.PropertyDefinition, configured propertyResourceModel) (propertyResourceModel, error) {
	model := modelFromPropertyDefinition(objectType, definition)
	if !configured.DisplayOrder.IsNull() && !configured.DisplayOrder.IsUnknown() && configured.DisplayOrder.ValueInt64() == -1 {
		definitions, err := r.client.List(ctx, objectType, false, sensitivityValue(configured.DataSensitivity), "")
		if err != nil {
			return propertyResourceModel{}, err
		}
		if propertyIsLastInGroup(definition, definitions) {
			model.DisplayOrder = types.Int64Value(-1)
		}
	}
	preserveConfiguredOptionAppendOrders(ctx, &model, configured.Options)
	return model, nil
}

func propertyIsLastInGroup(definition hubspot.PropertyDefinition, definitions []hubspot.PropertyDefinition) bool {
	if definition.DisplayOrder == nil {
		return false
	}
	last := *definition.DisplayOrder
	for _, candidate := range definitions {
		if candidate.GroupName == definition.GroupName && candidate.DisplayOrder != nil && *candidate.DisplayOrder > last {
			last = *candidate.DisplayOrder
		}
	}
	return *definition.DisplayOrder == last
}

func preserveConfiguredOptionAppendOrders(ctx context.Context, model *propertyResourceModel, configured types.Map) {
	if model.Options.IsNull() || model.Options.IsUnknown() || configured.IsNull() || configured.IsUnknown() {
		return
	}
	var remote, planned map[string]propertyOptionModel
	if diagnostics := model.Options.ElementsAs(ctx, &remote, false); diagnostics.HasError() {
		return
	}
	if diagnostics := configured.ElementsAs(ctx, &planned, false); diagnostics.HasError() {
		return
	}
	sentinelKeys := make([]string, 0, len(planned))
	for key, option := range planned {
		if !option.DisplayOrder.IsNull() && !option.DisplayOrder.IsUnknown() && option.DisplayOrder.ValueInt64() == -1 {
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
		leftOrder := remote[remoteKeys[left]].DisplayOrder
		rightOrder := remote[remoteKeys[right]].DisplayOrder
		if leftOrder.IsNull() || leftOrder.IsUnknown() || rightOrder.IsNull() || rightOrder.IsUnknown() || leftOrder.ValueInt64() == rightOrder.ValueInt64() {
			return remoteKeys[left] < remoteKeys[right]
		}
		return leftOrder.ValueInt64() < rightOrder.ValueInt64()
	})
	if len(sentinelKeys) > len(remoteKeys) || strings.Join(remoteKeys[len(remoteKeys)-len(sentinelKeys):], "\x00") != strings.Join(sentinelKeys, "\x00") {
		return
	}
	for key, option := range remote {
		if hint, ok := planned[key]; ok && !hint.DisplayOrder.IsNull() && !hint.DisplayOrder.IsUnknown() && hint.DisplayOrder.ValueInt64() == -1 {
			option.DisplayOrder = types.Int64Value(-1)
			remote[key] = option
		}
	}
	model.Options = propertyOptionMap(remote)
}

func propertyOptionMap(options map[string]propertyOptionModel) types.Map {
	values := make(map[string]attr.Value, len(options))
	for key, option := range options {
		values[key] = types.ObjectValueMust(optionAttrTypes(), map[string]attr.Value{
			"label":         option.Label,
			"description":   option.Description,
			"display_order": option.DisplayOrder,
			"hidden":        option.Hidden,
		})
	}
	return types.MapValueMust(types.ObjectType{AttrTypes: optionAttrTypes()}, values)
}
func propertyMatchesPlan(d hubspot.PropertyDefinition, p propertyResourceModel) bool {
	if d.Name != p.Name.ValueString() || d.Label != p.Label.ValueString() || d.GroupName != p.GroupName.ValueString() || d.Type != p.Type.ValueString() || d.FieldType != p.FieldType.ValueString() {
		return false
	}
	if !stringMatches(p.Description, d.Description) || !intMatches(p.DisplayOrder, d.DisplayOrder) || !boolMatches(p.FormField, d.FormField) || !boolMatches(p.Hidden, d.Hidden) || !boolMatches(p.HasUniqueValue, d.HasUniqueValue) || !stringMatches(p.DataSensitivity, d.DataSensitivity) || !boolMatches(p.ExternalOptions, d.ExternalOptions) || !boolMatches(p.ShowCurrencySymbol, d.ShowCurrencySymbol) || !stringMatches(p.CalculationFormula, d.CalculationFormula) || !stringMatches(p.CurrencyPropertyName, d.CurrencyPropertyName) || !stringMatches(p.NumberDisplayHint, d.NumberDisplayHint) || !stringMatches(p.TextDisplayHint, d.TextDisplayHint) || !stringMatches(p.ReferencedObjectType, d.ReferencedObjectType) {
		return false
	}
	if p.Options.IsNull() || p.Options.IsUnknown() {
		return true
	}
	var options map[string]propertyOptionModel
	if diags := p.Options.ElementsAs(context.Background(), &options, false); diags.HasError() {
		return false
	}
	if len(options) != len(d.Options) {
		return false
	}
	for _, option := range d.Options {
		planned, ok := options[option.Value]
		if !ok || planned.Label.ValueString() != option.Label || !stringMatches(planned.Description, option.Description) || !intMatches(planned.DisplayOrder, option.DisplayOrder) || !boolMatches(planned.Hidden, option.Hidden) {
			return false
		}
	}
	return true
}
func stringMatches(v types.String, remote *string) bool {
	if v.IsNull() || v.IsUnknown() {
		return remote == nil
	}
	return remote != nil && *remote == v.ValueString()
}
func intMatches(v types.Int64, remote *int64) bool {
	if v.IsNull() {
		return remote == nil
	}
	if v.IsUnknown() || v.ValueInt64() == -1 {
		return remote != nil
	}
	return remote != nil && *remote == v.ValueInt64()
}
func boolMatches(v types.Bool, remote *bool) bool {
	if v.IsNull() || v.IsUnknown() {
		return remote == nil
	}
	return remote != nil && *remote == v.ValueBool()
}
func readOnlyDefinition(d hubspot.PropertyDefinition) bool {
	return d.ModificationMetadata != nil && d.ModificationMetadata.ReadOnlyDefinition != nil && *d.ModificationMetadata.ReadOnlyDefinition
}

func isAmbiguous(err error) bool {
	var apiErr *hubspot.Error
	return errors.As(err, &apiErr) && apiErr.Ambiguous
}
func optionKeysChanged(a, b types.Map) bool {
	if a.IsUnknown() || b.IsUnknown() {
		return false
	}
	if a.IsNull() || b.IsNull() {
		return a.IsNull() != b.IsNull()
	}
	aElements := a.Elements()
	bElements := b.Elements()
	if len(aElements) != len(bElements) {
		return true
	}
	for key := range aElements {
		if _, ok := bElements[key]; !ok {
			return true
		}
	}
	return false
}

func knownStringChanged(a, b types.String) bool {
	return !a.IsNull() && !a.IsUnknown() && !b.IsNull() && !b.IsUnknown() && a.ValueString() != b.ValueString()
}

type propertyTypeValidator struct{}

func (propertyTypeValidator) Description(context.Context) string {
	return "must be a supported HubSpot property type"
}
func (v propertyTypeValidator) MarkdownDescription(c context.Context) string { return v.Description(c) }
func (propertyTypeValidator) ValidateString(_ context.Context, r validator.StringRequest, res *validator.StringResponse) {
	switch r.ConfigValue.ValueString() {
	case "bool", "enumeration", "date", "datetime", "string", "number":
	default:
		res.Diagnostics.AddAttributeError(r.Path, "Invalid property type", "Use bool, enumeration, date, datetime, string, or number.")
	}
}

type propertyFieldTypeValidator struct{}

func (propertyFieldTypeValidator) Description(context.Context) string {
	return "must be a supported HubSpot property field type"
}
func (v propertyFieldTypeValidator) MarkdownDescription(c context.Context) string {
	return v.Description(c)
}
func (propertyFieldTypeValidator) ValidateString(_ context.Context, r validator.StringRequest, res *validator.StringResponse) {
	if r.ConfigValue.IsNull() || r.ConfigValue.IsUnknown() {
		return
	}
	if r.ConfigValue.ValueString() == "" {
		res.Diagnostics.AddAttributeError(r.Path, "Invalid field type", "Field type must not be empty.")
	}
}

type boolRequiresReplace struct{}

func (boolRequiresReplace) Description(context.Context) string             { return "changes require replacement" }
func (v boolRequiresReplace) MarkdownDescription(c context.Context) string { return v.Description(c) }
func (boolRequiresReplace) PlanModifyBool(_ context.Context, req planmodifier.BoolRequest, res *planmodifier.BoolResponse) {
	if !req.PlanValue.Equal(req.StateValue) {
		res.RequiresReplace = true
	}
}
