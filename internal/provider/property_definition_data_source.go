package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/jackemcpherson/terraform-provider-hubspot/internal/hubspot"
)

type PropertyDefinitionDataSource struct {
	client *hubspot.PropertyDefinitionClient
}
type PropertyDefinitionsDataSource struct {
	client *hubspot.PropertyDefinitionClient
}

type propertyDefinitionModel struct {
	ID                   types.String `tfsdk:"id"`
	ObjectType           types.String `tfsdk:"object_type"`
	Name                 types.String `tfsdk:"name"`
	Archived             types.Bool   `tfsdk:"archived"`
	Locale               types.String `tfsdk:"locale"`
	Label                types.String `tfsdk:"label"`
	GroupName            types.String `tfsdk:"group_name"`
	Type                 types.String `tfsdk:"type"`
	FieldType            types.String `tfsdk:"field_type"`
	Description          types.String `tfsdk:"description"`
	DisplayOrder         types.Int64  `tfsdk:"display_order"`
	FormField            types.Bool   `tfsdk:"form_field"`
	Hidden               types.Bool   `tfsdk:"hidden"`
	HasUniqueValue       types.Bool   `tfsdk:"has_unique_value"`
	ExternalOptions      types.Bool   `tfsdk:"external_options"`
	ReferencedObjectType types.String `tfsdk:"referenced_object_type"`
	ShowCurrencySymbol   types.Bool   `tfsdk:"show_currency_symbol"`
	CalculationFormula   types.String `tfsdk:"calculation_formula"`
	CurrencyPropertyName types.String `tfsdk:"currency_property_name"`
	NumberDisplayHint    types.String `tfsdk:"number_display_hint"`
	TextDisplayHint      types.String `tfsdk:"text_display_hint"`
	DateDisplayHint      types.String `tfsdk:"date_display_hint"`
	DataSensitivity      types.String `tfsdk:"data_sensitivity"`
	SensitivityCategory  types.String `tfsdk:"sensitivity_category"`
	Calculated           types.Bool   `tfsdk:"calculated"`
	HubSpotDefined       types.Bool   `tfsdk:"hubspot_defined"`
	ArchivedValue        types.Bool   `tfsdk:"is_archived"`
	ArchivedAt           types.String `tfsdk:"archived_at"`
	CreatedAt            types.String `tfsdk:"created_at"`
	UpdatedAt            types.String `tfsdk:"updated_at"`
	CreatedUserID        types.String `tfsdk:"created_user_id"`
	UpdatedUserID        types.String `tfsdk:"updated_user_id"`
	Options              types.Map    `tfsdk:"options"`
	ModificationMetadata types.Object `tfsdk:"modification_metadata"`
}

type propertyDefinitionsModel struct {
	ObjectType      types.String `tfsdk:"object_type"`
	Archived        types.Bool   `tfsdk:"archived"`
	DataSensitivity types.String `tfsdk:"data_sensitivity"`
	Locale          types.String `tfsdk:"locale"`
	Definitions     types.Map    `tfsdk:"definitions"`
}

func NewPropertyDefinitionDataSource() datasource.DataSource { return &PropertyDefinitionDataSource{} }
func NewPropertyDefinitionsDataSource() datasource.DataSource {
	return &PropertyDefinitionsDataSource{}
}

func propertyDefinitionAttrs(includeName bool) map[string]schema.Attribute {
	attrs := map[string]schema.Attribute{
		"id": schema.StringAttribute{Computed: true, Description: "Canonical object_type/property_name identity."}, "object_type": schema.StringAttribute{Required: true, Description: "Exact CRM object type API identifier.", Validators: []validator.String{identifierValidator{kind: "CRM object type"}}}, "archived": schema.BoolAttribute{Optional: true, Computed: true, Description: "Select archived definitions instead of active definitions; defaults to false."}, "data_sensitivity": schema.StringAttribute{Optional: true, Computed: true, Description: "Sensitivity selector; defaults to non_sensitive.", Validators: []validator.String{sensitivityValidator{}}}, "locale": schema.StringAttribute{Optional: true, Description: "Optional HubSpot locale selector."},
		"label": schema.StringAttribute{Computed: true}, "group_name": schema.StringAttribute{Computed: true}, "type": schema.StringAttribute{Computed: true}, "field_type": schema.StringAttribute{Computed: true}, "description": schema.StringAttribute{Computed: true}, "display_order": schema.Int64Attribute{Computed: true}, "form_field": schema.BoolAttribute{Computed: true}, "hidden": schema.BoolAttribute{Computed: true}, "has_unique_value": schema.BoolAttribute{Computed: true}, "external_options": schema.BoolAttribute{Computed: true}, "referenced_object_type": schema.StringAttribute{Computed: true}, "show_currency_symbol": schema.BoolAttribute{Computed: true}, "calculation_formula": schema.StringAttribute{Computed: true}, "currency_property_name": schema.StringAttribute{Computed: true}, "number_display_hint": schema.StringAttribute{Computed: true}, "text_display_hint": schema.StringAttribute{Computed: true}, "date_display_hint": schema.StringAttribute{Computed: true}, "sensitivity_category": schema.StringAttribute{Computed: true}, "calculated": schema.BoolAttribute{Computed: true}, "hubspot_defined": schema.BoolAttribute{Computed: true}, "is_archived": schema.BoolAttribute{Computed: true}, "archived_at": schema.StringAttribute{Computed: true}, "created_at": schema.StringAttribute{Computed: true}, "updated_at": schema.StringAttribute{Computed: true}, "created_user_id": schema.StringAttribute{Computed: true}, "updated_user_id": schema.StringAttribute{Computed: true},
		"options": schema.MapAttribute{Computed: true, ElementType: types.ObjectType{AttrTypes: optionAttrTypes()}}, "modification_metadata": schema.ObjectAttribute{Computed: true, AttributeTypes: metadataAttrTypes()},
	}
	if includeName {
		attrs["name"] = schema.StringAttribute{Required: true, Description: "Exact immutable property name.", Validators: []validator.String{identifierValidator{kind: "property name"}}}
	}
	return attrs
}
func optionAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{"label": types.StringType, "description": types.StringType, "display_order": types.Int64Type, "hidden": types.BoolType}
}
func metadataAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{"archivable": types.BoolType, "read_only_definition": types.BoolType, "read_only_value": types.BoolType, "read_only_options": types.BoolType}
}

func (d *PropertyDefinitionDataSource) Metadata(_ context.Context, _ datasource.MetadataRequest, r *datasource.MetadataResponse) {
	r.TypeName = "hubspot_property_definition"
}
func (d *PropertyDefinitionDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, r *datasource.SchemaResponse) {
	r.Schema = schema.Schema{Description: "Reads one HubSpot CRM property definition without reading CRM records.", Attributes: propertyDefinitionAttrs(true)}
}
func (d *PropertyDefinitionDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, r *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	clients, ok := req.ProviderData.(*hubspot.ClientSet)
	if !ok || clients == nil {
		r.Diagnostics.AddError("Provider is not configured", "The HubSpot client set was not available.")
		return
	}
	d.client = clients.Properties
}
func (d *PropertyDefinitionDataSource) Read(ctx context.Context, req datasource.ReadRequest, r *datasource.ReadResponse) {
	var plan propertyDefinitionModel
	r.Diagnostics.Append(req.Config.Get(ctx, &plan)...)
	if r.Diagnostics.HasError() {
		return
	}
	def, err := d.client.Get(ctx, plan.ObjectType.ValueString(), plan.Name.ValueString(), boolValue(plan.Archived, false), stringValue(plan.DataSensitivity, "non_sensitive"), stringValue(plan.Locale, ""))
	if err != nil {
		if isNotFound(err) {
			r.Diagnostics.AddAttributeError(path.Root("name"), "Property definition not found", "No property definition matched the exact object type, name, archive selector, sensitivity selector, and locale.")
			return
		}
		appendHubSpotDiagnostic(&r.Diagnostics, "Property definition read failed", err)
		return
	}
	if def.Name != plan.Name.ValueString() {
		r.Diagnostics.AddAttributeError(path.Root("name"), "Property definition identity mismatch", "HubSpot returned a different immutable property name.")
		return
	}
	for _, option := range def.Options {
		if option.Value == "" {
			r.Diagnostics.AddError("Malformed property definition response", "HubSpot returned an option without its immutable value.")
			return
		}
	}
	state := modelFromDefinition(plan.ObjectType.ValueString(), def)
	state.Archived = types.BoolValue(boolValue(plan.Archived, false))
	state.Locale = plan.Locale
	state.DataSensitivity = types.StringValue(stringValue(plan.DataSensitivity, "non_sensitive"))
	r.Diagnostics.Append(r.State.Set(ctx, &state)...)
}

func (d *PropertyDefinitionsDataSource) Metadata(_ context.Context, _ datasource.MetadataRequest, r *datasource.MetadataResponse) {
	r.TypeName = "hubspot_property_definitions"
}
func (d *PropertyDefinitionsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, r *datasource.SchemaResponse) {
	r.Schema = schema.Schema{Description: "Reads HubSpot CRM property definitions without reading CRM records.", Attributes: map[string]schema.Attribute{"object_type": schema.StringAttribute{Required: true, Description: "Exact CRM object type API identifier.", Validators: []validator.String{identifierValidator{kind: "CRM object type"}}}, "archived": schema.BoolAttribute{Optional: true, Computed: true, Description: "Select archived definitions instead of active definitions; defaults to false."}, "data_sensitivity": schema.StringAttribute{Optional: true, Computed: true, Description: "Sensitivity selector; defaults to non_sensitive.", Validators: []validator.String{sensitivityValidator{}}}, "locale": schema.StringAttribute{Optional: true, Description: "Optional HubSpot locale selector."}, "definitions": schema.MapAttribute{Computed: true, Description: "Definitions keyed by immutable property name; an empty map is valid.", ElementType: types.ObjectType{AttrTypes: definitionAttrTypes()}}}}
}
func definitionAttrTypes() map[string]attr.Type {
	a := propertyDefinitionAttrs(false)
	out := make(map[string]attr.Type, len(a))
	for k, v := range a {
		if k == "id" || k == "object_type" || k == "archived" || k == "locale" {
			continue
		}
		out[k] = v.GetType()
	}
	out["name"] = types.StringType
	return out
}
func (d *PropertyDefinitionsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, r *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	clients, ok := req.ProviderData.(*hubspot.ClientSet)
	if !ok || clients == nil {
		r.Diagnostics.AddError("Provider is not configured", "The HubSpot client set was not available.")
		return
	}
	d.client = clients.Properties
}
func (d *PropertyDefinitionsDataSource) Read(ctx context.Context, req datasource.ReadRequest, r *datasource.ReadResponse) {
	var plan propertyDefinitionsModel
	r.Diagnostics.Append(req.Config.Get(ctx, &plan)...)
	if r.Diagnostics.HasError() {
		return
	}
	defs, err := d.client.List(ctx, plan.ObjectType.ValueString(), boolValue(plan.Archived, false), stringValue(plan.DataSensitivity, "non_sensitive"), stringValue(plan.Locale, ""))
	if err != nil {
		appendHubSpotDiagnostic(&r.Diagnostics, "Property definition collection read failed", err)
		return
	}
	result := make(map[string]attr.Value, len(defs))
	for _, def := range defs {
		if def.Name == "" {
			r.Diagnostics.AddError("Malformed property definition response", "HubSpot returned a definition without its immutable name.")
			return
		}
		for _, option := range def.Options {
			if option.Value == "" {
				r.Diagnostics.AddError("Malformed property definition response", "HubSpot returned an option without its immutable value.")
				return
			}
		}
		result[def.Name] = definitionObject(modelFromDefinition(plan.ObjectType.ValueString(), def))
	}
	value := types.MapValueMust(types.ObjectType{AttrTypes: definitionAttrTypes()}, result)
	state := propertyDefinitionsModel{ObjectType: plan.ObjectType, Archived: types.BoolValue(boolValue(plan.Archived, false)), DataSensitivity: types.StringValue(stringValue(plan.DataSensitivity, "non_sensitive")), Locale: plan.Locale, Definitions: value}
	r.Diagnostics.Append(r.State.Set(ctx, &state)...)
}

func boolValue(v types.Bool, fallback bool) bool {
	if v.IsNull() || v.IsUnknown() {
		return fallback
	}
	return v.ValueBool()
}
func stringValue(v types.String, fallback string) string {
	if v.IsNull() || v.IsUnknown() {
		return fallback
	}
	return v.ValueString()
}
func modelFromDefinition(objectType string, d hubspot.PropertyDefinition) propertyDefinitionModel {
	m := propertyDefinitionModel{ID: types.StringValue(objectType + "/" + d.Name), ObjectType: types.StringValue(objectType), Name: types.StringValue(d.Name), Label: types.StringValue(d.Label), GroupName: types.StringValue(d.GroupName), Type: types.StringValue(d.Type), FieldType: types.StringValue(d.FieldType), DataSensitivity: optionalString(d.DataSensitivity), SensitivityCategory: optionalString(d.SensitivityCategory)}
	m.Description = optionalString(d.Description)
	m.DisplayOrder = optionalInt(d.DisplayOrder)
	m.FormField = optionalBool(d.FormField)
	m.Hidden = optionalBool(d.Hidden)
	m.HasUniqueValue = optionalBool(d.HasUniqueValue)
	m.ExternalOptions = optionalBool(d.ExternalOptions)
	m.ReferencedObjectType = optionalString(d.ReferencedObjectType)
	m.ShowCurrencySymbol = optionalBool(d.ShowCurrencySymbol)
	m.CalculationFormula = optionalString(d.CalculationFormula)
	m.CurrencyPropertyName = optionalString(d.CurrencyPropertyName)
	m.NumberDisplayHint = optionalString(d.NumberDisplayHint)
	m.TextDisplayHint = optionalString(d.TextDisplayHint)
	m.DateDisplayHint = optionalString(d.DateDisplayHint)
	m.Calculated = optionalBool(d.Calculated)
	m.HubSpotDefined = optionalBool(d.HubSpotDefined)
	m.ArchivedValue = optionalBool(d.Archived)
	m.ArchivedAt = optionalString(d.ArchivedAt)
	m.CreatedAt = optionalString(d.CreatedAt)
	m.UpdatedAt = optionalString(d.UpdatedAt)
	m.CreatedUserID = optionalString(d.CreatedUserID)
	m.UpdatedUserID = optionalString(d.UpdatedUserID)
	m.Options = optionsValue(d.Options)
	m.ModificationMetadata = metadataValue(d.ModificationMetadata)
	return m
}
func optionalString(v *string) types.String {
	if v == nil {
		return types.StringNull()
	}
	return types.StringValue(*v)
}
func optionalInt(v *int64) types.Int64 {
	if v == nil {
		return types.Int64Null()
	}
	return types.Int64Value(*v)
}
func optionalBool(v *bool) types.Bool {
	if v == nil {
		return types.BoolNull()
	}
	return types.BoolValue(*v)
}
func optionsValue(options []hubspot.PropertyOption) types.Map {
	if options == nil {
		return types.MapNull(types.ObjectType{AttrTypes: optionAttrTypes()})
	}
	vals := map[string]attr.Value{}
	for _, o := range options {
		if o.Value == "" {
			continue
		}
		vals[o.Value] = types.ObjectValueMust(optionAttrTypes(), map[string]attr.Value{"label": types.StringValue(o.Label), "description": optionalString(o.Description), "display_order": optionalInt(o.DisplayOrder), "hidden": optionalBool(o.Hidden)})
	}
	return types.MapValueMust(types.ObjectType{AttrTypes: optionAttrTypes()}, vals)
}
func metadataValue(m *hubspot.ModificationMetadata) types.Object {
	if m == nil {
		return types.ObjectNull(metadataAttrTypes())
	}
	return types.ObjectValueMust(metadataAttrTypes(), map[string]attr.Value{"archivable": optionalBool(m.Archivable), "read_only_definition": optionalBool(m.ReadOnlyDefinition), "read_only_value": optionalBool(m.ReadOnlyValue), "read_only_options": optionalBool(m.ReadOnlyOptions)})
}
func definitionObject(m propertyDefinitionModel) attr.Value {
	return types.ObjectValueMust(definitionAttrTypes(), map[string]attr.Value{"name": m.Name, "label": m.Label, "group_name": m.GroupName, "type": m.Type, "field_type": m.FieldType, "description": m.Description, "display_order": m.DisplayOrder, "form_field": m.FormField, "hidden": m.Hidden, "has_unique_value": m.HasUniqueValue, "external_options": m.ExternalOptions, "referenced_object_type": m.ReferencedObjectType, "show_currency_symbol": m.ShowCurrencySymbol, "calculation_formula": m.CalculationFormula, "currency_property_name": m.CurrencyPropertyName, "number_display_hint": m.NumberDisplayHint, "text_display_hint": m.TextDisplayHint, "date_display_hint": m.DateDisplayHint, "data_sensitivity": m.DataSensitivity, "sensitivity_category": m.SensitivityCategory, "calculated": m.Calculated, "hubspot_defined": m.HubSpotDefined, "is_archived": m.ArchivedValue, "archived_at": m.ArchivedAt, "created_at": m.CreatedAt, "updated_at": m.UpdatedAt, "created_user_id": m.CreatedUserID, "updated_user_id": m.UpdatedUserID, "options": m.Options, "modification_metadata": m.ModificationMetadata})
}

type sensitivityValidator struct{}

func (sensitivityValidator) Description(context.Context) string {
	return "must be non_sensitive, sensitive, or highly_sensitive"
}
func (s sensitivityValidator) MarkdownDescription(ctx context.Context) string {
	return s.Description(ctx)
}
func (sensitivityValidator) ValidateString(_ context.Context, request validator.StringRequest, response *validator.StringResponse) {
	if request.ConfigValue.IsNull() || request.ConfigValue.IsUnknown() {
		return
	}
	value := request.ConfigValue.ValueString()
	if value != "non_sensitive" && value != "sensitive" && value != "highly_sensitive" {
		response.Diagnostics.AddAttributeError(request.Path, "Invalid data sensitivity", "Use non_sensitive, sensitive, or highly_sensitive.")
	}
}
