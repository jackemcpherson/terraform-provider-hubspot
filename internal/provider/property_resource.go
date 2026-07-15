package provider

import (
	"context"
	"errors"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
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
	response.Schema = schema.Schema{Description: "Manages one ordinary or enumeration HubSpot CRM property definition.", Attributes: map[string]schema.Attribute{
		"id":          schema.StringAttribute{Computed: true},
		"object_type": schema.StringAttribute{Required: true, Validators: []validator.String{identifierValidator{kind: "CRM object type"}}, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
		"name":        schema.StringAttribute{Required: true, Validators: []validator.String{identifierValidator{kind: "property name"}}, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
		"label":       schema.StringAttribute{Required: true}, "group_name": schema.StringAttribute{Required: true},
		"type": schema.StringAttribute{Required: true, Validators: []validator.String{propertyTypeValidator{}}}, "field_type": schema.StringAttribute{Required: true, Validators: []validator.String{propertyFieldTypeValidator{}}},
		"description": schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("")}, "display_order": schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(-1)},
		"form_field": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false)}, "hidden": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false)},
		"has_unique_value": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), PlanModifiers: []planmodifier.Bool{boolRequiresReplace{}}},
		"data_sensitivity": schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("non_sensitive"), Validators: []validator.String{sensitivityValidator{}}, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
		"external_options": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), PlanModifiers: []planmodifier.Bool{boolRequiresReplace{}}}, "show_currency_symbol": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false)},
		"calculation_formula": schema.StringAttribute{Optional: true, Computed: true}, "currency_property_name": schema.StringAttribute{Optional: true, Computed: true}, "number_display_hint": schema.StringAttribute{Optional: true, Computed: true}, "text_display_hint": schema.StringAttribute{Optional: true, Computed: true}, "referenced_object_type": schema.StringAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
		"options": schema.MapAttribute{Optional: true, Computed: true, ElementType: types.ObjectType{AttrTypes: map[string]attr.Type{"label": types.StringType, "description": types.StringType, "display_order": types.Int64Type, "hidden": types.BoolType}}},
	}}
}
func (r *PropertyResource) Configure(_ context.Context, request resource.ConfigureRequest, response *resource.ConfigureResponse) {
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
	if response.Diagnostics.HasError() || plan.DataSensitivity.IsNull() || plan.DataSensitivity.IsUnknown() || plan.DataSensitivity.ValueString() == "non_sensitive" {
		return
	}
	response.Diagnostics.AddWarning("Sensitive property tier and retention risk", "Sensitive and highly_sensitive properties require Enterprise eligibility and object-specific sensitive write scopes. Classification is immutable, and archived sensitive properties are permanently deleted after 90 days; verify account tier, scopes, and cleanup before apply.")
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
				response.Diagnostics.Append(response.State.Set(ctx, modelFromPropertyDefinition(plan.ObjectType.ValueString(), recovered))...)
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
	response.Diagnostics.Append(response.State.Set(ctx, modelFromPropertyDefinition(plan.ObjectType.ValueString(), verified))...)
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
	response.Diagnostics.Append(response.State.Set(ctx, modelFromPropertyDefinition(state.ObjectType.ValueString(), def))...)
}
func (r *PropertyResource) Update(ctx context.Context, request resource.UpdateRequest, response *resource.UpdateResponse) {
	var plan, old propertyResourceModel
	response.Diagnostics.Append(request.Plan.Get(ctx, &plan)...)
	response.Diagnostics.Append(request.State.Get(ctx, &old)...)
	if response.Diagnostics.HasError() {
		return
	}
	if plan.Type.ValueString() != old.Type.ValueString() || plan.FieldType.ValueString() != old.FieldType.ValueString() {
		response.Diagnostics.AddWarning("Property type transition", "HubSpot may retain CRM record values that no longer match the new definition.")
	}
	if optionKeysChanged(plan.Options, old.Options) {
		response.Diagnostics.AddWarning("Property option values changed", "HubSpot does not migrate existing CRM record values when option keys are removed or renamed.")
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
				response.Diagnostics.Append(response.State.Set(ctx, modelFromPropertyDefinition(plan.ObjectType.ValueString(), recovered))...)
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
	response.Diagnostics.Append(response.State.Set(ctx, modelFromPropertyDefinition(plan.ObjectType.ValueString(), verified))...)
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
	list := make([]hubspot.PropertyOption, 0, len(options))
	for value, opt := range options {
		list = append(list, hubspot.PropertyOption{Value: value, Label: opt.Label.ValueString(), Description: stringPointer(opt.Description), DisplayOrder: intPointer(opt.DisplayOrder), Hidden: boolPointer(opt.Hidden)})
	}
	return hubspot.PropertyWrite{Name: model.Name.ValueString(), Label: model.Label.ValueString(), GroupName: model.GroupName.ValueString(), Type: model.Type.ValueString(), FieldType: model.FieldType.ValueString(), Description: stringPointer(model.Description), DisplayOrder: intPointer(model.DisplayOrder), FormField: boolPointer(model.FormField), Hidden: boolPointer(model.Hidden), HasUniqueValue: boolPointer(model.HasUniqueValue), DataSensitivity: stringPointer(model.DataSensitivity), ExternalOptions: boolPointer(model.ExternalOptions), ShowCurrencySymbol: boolPointer(model.ShowCurrencySymbol), CalculationFormula: stringPointer(model.CalculationFormula), CurrencyPropertyName: stringPointer(model.CurrencyPropertyName), NumberDisplayHint: stringPointer(model.NumberDisplayHint), TextDisplayHint: stringPointer(model.TextDisplayHint), ReferencedObjectType: stringPointer(model.ReferencedObjectType), Options: list}, nil
}
func stringPointer(v types.String) *string {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	s := v.ValueString()
	return &s
}
func intPointer(v types.Int64) *int64 {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	n := v.ValueInt64()
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
	if v.IsNull() || v.IsUnknown() {
		return remote == nil
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
	if a.IsNull() || b.IsNull() {
		return a.IsNull() != b.IsNull()
	}
	return a.String() != b.String()
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
