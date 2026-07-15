package provider

import (
	"context"
	"errors"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/jackemcpherson/terraform-provider-hubspot/internal/hubspot"
	"strings"
)

type CustomSchemaResource struct{ client *hubspot.SchemaClient }
type schemaPropertyModel struct {
	Label              types.String `tfsdk:"label"`
	Type               types.String `tfsdk:"type"`
	FieldType          types.String `tfsdk:"field_type"`
	Description        types.String `tfsdk:"description"`
	DisplayOrder       types.Int64  `tfsdk:"display_order"`
	FormField          types.Bool   `tfsdk:"form_field"`
	Hidden             types.Bool   `tfsdk:"hidden"`
	HasUniqueValue     types.Bool   `tfsdk:"has_unique_value"`
	ShowCurrencySymbol types.Bool   `tfsdk:"show_currency_symbol"`
	Options            types.Map    `tfsdk:"options"`
}
type customSchemaModel struct {
	ID                                types.String `tfsdk:"id"`
	ObjectTypeID                      types.String `tfsdk:"object_type_id"`
	FullyQualifiedName                types.String `tfsdk:"fully_qualified_name"`
	Name                              types.String `tfsdk:"name"`
	Labels                            types.Object `tfsdk:"labels"`
	PrimaryDisplayProperty            types.String `tfsdk:"primary_display_property"`
	Description                       types.String `tfsdk:"description"`
	AllowsSensitiveProperties         types.Bool   `tfsdk:"allows_sensitive_properties"`
	AssociatedObjects                 types.Set    `tfsdk:"associated_objects"`
	ShouldCreateSameObjectAssociation types.Bool   `tfsdk:"should_create_same_object_association"`
	RequiredProperties                types.Set    `tfsdk:"required_properties"`
	SearchableProperties              types.Set    `tfsdk:"searchable_properties"`
	SecondaryDisplayProperties        types.List   `tfsdk:"secondary_display_properties"`
	ExpectedExternalProperties        types.Set    `tfsdk:"expected_external_properties"`
	DeletionProtection                types.Bool   `tfsdk:"deletion_protection"`
	Properties                        types.Map    `tfsdk:"properties"`
}

func NewCustomSchemaResource() resource.Resource { return &CustomSchemaResource{} }
func (r *CustomSchemaResource) Metadata(_ context.Context, _ resource.MetadataRequest, res *resource.MetadataResponse) {
	res.TypeName = "hubspot_custom_object_schema"
}
func (r *CustomSchemaResource) Schema(_ context.Context, _ resource.SchemaRequest, res *resource.SchemaResponse) {
	res.Schema = schema.Schema{Version: 1, Description: "Manages a custom HubSpot object schema with continuously owned bootstrap properties.", Attributes: map[string]schema.Attribute{
		"id":                                    schema.StringAttribute{Computed: true, Description: "Canonical HubSpot object type ID."},
		"object_type_id":                        schema.StringAttribute{Computed: true, Description: "HubSpot object type ID returned after creation."},
		"fully_qualified_name":                  schema.StringAttribute{Computed: true, Description: "HubSpot fully qualified schema name."},
		"name":                                  schema.StringAttribute{Required: true, Description: "Immutable schema internal name.", PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
		"labels":                                schema.ObjectAttribute{Required: true, Description: "Singular and plural display labels.", AttributeTypes: map[string]attr.Type{"singular": types.StringType, "plural": types.StringType}},
		"primary_display_property":              schema.StringAttribute{Required: true, Description: "Owned property used as the primary display value."},
		"description":                           schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), Description: "Schema description."},
		"allows_sensitive_properties":           schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), Description: "Allows sensitive properties on eligible Enterprise accounts."},
		"associated_objects":                    schema.SetAttribute{Optional: true, Computed: true, ElementType: types.StringType, Description: "Exact object types associated at schema creation.", PlanModifiers: []planmodifier.Set{setRequiresReplace{}}},
		"should_create_same_object_association": schema.BoolAttribute{Optional: true, Computed: true, Description: "Requests a same-object association at creation."},
		"required_properties":                   schema.SetAttribute{Optional: true, Computed: true, ElementType: types.StringType, Description: "Owned properties required by the schema."},
		"searchable_properties":                 schema.SetAttribute{Optional: true, Computed: true, ElementType: types.StringType, Description: "Owned properties indexed for search."},
		"secondary_display_properties":          schema.ListAttribute{Optional: true, Computed: true, ElementType: types.StringType, Description: "Ordered owned properties shown as secondary values."},
		"expected_external_properties":          schema.SetAttribute{Optional: true, Computed: true, ElementType: types.StringType, Description: "Separately managed properties acknowledged without adoption."},
		"deletion_protection":                   schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true), Description: "Blocks schema deletion until disabled in a prior apply."},
		"properties": schema.MapNestedAttribute{Required: true, Description: "Nonempty map of continuously owned bootstrap properties.", NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
			"label":                schema.StringAttribute{Required: true, Description: "Property display label."},
			"type":                 schema.StringAttribute{Required: true, Description: "HubSpot property storage type."},
			"field_type":           schema.StringAttribute{Required: true, Description: "HubSpot editor field type."},
			"description":          schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), Description: "Property description."},
			"display_order":        schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(-1), Description: "HubSpot display order; defaults to -1."},
			"form_field":           schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), Description: "Whether the property can appear in forms."},
			"hidden":               schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), Description: "Whether HubSpot hides the property."},
			"has_unique_value":     schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), Description: "Whether property values must be unique."},
			"show_currency_symbol": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), Description: "Whether HubSpot displays a currency symbol."},
			"options":              schema.MapAttribute{Optional: true, Computed: true, ElementType: types.StringType, Description: "Reserved option metadata for supported property types."},
		}}},
	}}
}

func (r *CustomSchemaResource) UpgradeState(context.Context) map[int64]resource.StateUpgrader {
	return map[int64]resource.StateUpgrader{0: identityStateUpgrade()}
}
func (r *CustomSchemaResource) Configure(_ context.Context, req resource.ConfigureRequest, res *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	clients, ok := req.ProviderData.(*hubspot.ClientSet)
	if !ok || clients == nil || clients.Schemas == nil {
		res.Diagnostics.AddError("Provider is not configured", "The HubSpot schema client was not available.")
		return
	}
	r.client = clients.Schemas
}
func (r *CustomSchemaResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, res *resource.ModifyPlanResponse) {
	if req.Plan.Raw.IsNull() {
		return
	}
	var plan customSchemaModel
	res.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if res.Diagnostics.HasError() || plan.Properties.IsUnknown() {
		return
	}
	var props map[string]schemaPropertyModel
	if diags := plan.Properties.ElementsAs(ctx, &props, false); diags.HasError() {
		return
	}
	if _, ok := props[plan.PrimaryDisplayProperty.ValueString()]; !ok {
		res.Diagnostics.AddAttributeError(path.Root("primary_display_property"), "Unknown owned property", "Every role must reference a property in the owned properties map.")
	}
}
func (r *CustomSchemaResource) Create(ctx context.Context, req resource.CreateRequest, res *resource.CreateResponse) {
	var plan customSchemaModel
	res.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if res.Diagnostics.HasError() {
		return
	}
	in, err := schemaWriteFromModel(ctx, plan)
	if err != nil {
		res.Diagnostics.AddError("Invalid custom schema", err.Error())
		return
	}
	out, err := r.client.Create(ctx, in)
	if err != nil {
		appendHubSpotDiagnostic(&res.Diagnostics, "Custom schema creation failed", err)
		return
	}
	verified, err := r.client.Get(ctx, out.ObjectTypeID)
	if err != nil {
		appendHubSpotDiagnostic(&res.Diagnostics, "Custom schema creation verification failed", err)
		return
	}
	res.Diagnostics.Append(res.State.Set(ctx, modelFromSchema(verified))...)
}
func (r *CustomSchemaResource) Read(ctx context.Context, req resource.ReadRequest, res *resource.ReadResponse) {
	var state customSchemaModel
	res.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if res.Diagnostics.HasError() {
		return
	}
	out, err := r.client.Get(ctx, state.ObjectTypeID.ValueString())
	if err != nil {
		if isNotFound(err) {
			res.State.RemoveResource(ctx)
			return
		}
		appendHubSpotDiagnostic(&res.Diagnostics, "Custom schema refresh failed", err)
		return
	}
	remote := modelFromSchema(out)
	remote.ExpectedExternalProperties = state.ExpectedExternalProperties
	remote.DeletionProtection = state.DeletionProtection
	owned := map[string]struct{}{}
	var ownedProps map[string]schemaPropertyModel
	if !state.Properties.IsNull() && !state.Properties.IsUnknown() {
		_ = state.Properties.ElementsAs(ctx, &ownedProps, false)
		for name := range ownedProps {
			owned[name] = struct{}{}
		}
	}
	expected := map[string]struct{}{}
	var expectedList []string
	if !state.ExpectedExternalProperties.IsNull() && !state.ExpectedExternalProperties.IsUnknown() {
		_ = state.ExpectedExternalProperties.ElementsAs(ctx, &expectedList, false)
		for _, name := range expectedList {
			expected[name] = struct{}{}
		}
	}
	for _, prop := range out.Properties {
		if _, ok := owned[prop.Name]; !ok {
			if _, quiet := expected[prop.Name]; !quiet {
				res.Diagnostics.AddWarning("Unexpected external schema property", "A remote property is outside this schema resource ownership; it remains unmanaged and blocks safe destroy.")
			}
		}
	}
	res.Diagnostics.Append(res.State.Set(ctx, &remote)...)
}
func (r *CustomSchemaResource) Update(ctx context.Context, req resource.UpdateRequest, res *resource.UpdateResponse) {
	var plan customSchemaModel
	res.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if res.Diagnostics.HasError() {
		return
	}
	var state customSchemaModel
	res.Diagnostics.Append(req.State.Get(ctx, &state)...)
	in, err := schemaWriteFromModel(ctx, plan)
	if err != nil {
		res.Diagnostics.AddError("Invalid custom schema", err.Error())
		return
	}
	if _, err = r.client.Update(ctx, state.ObjectTypeID.ValueString(), in); err != nil {
		appendHubSpotDiagnostic(&res.Diagnostics, "Custom schema update failed", err)
		return
	}
	out, err := r.client.Get(ctx, state.ObjectTypeID.ValueString())
	if err != nil {
		appendHubSpotDiagnostic(&res.Diagnostics, "Custom schema update verification failed", err)
		return
	}
	res.Diagnostics.Append(res.State.Set(ctx, modelFromSchema(out))...)
}
func (r *CustomSchemaResource) Delete(ctx context.Context, req resource.DeleteRequest, res *resource.DeleteResponse) {
	var state customSchemaModel
	res.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if res.Diagnostics.HasError() {
		return
	}
	if state.DeletionProtection.IsNull() || state.DeletionProtection.IsUnknown() || state.DeletionProtection.ValueBool() {
		res.Diagnostics.AddError("Custom schema deletion protection is enabled", "Apply deletion_protection = false in a prior plan before destroying this schema.")
		return
	}
	current, readErr := r.client.Get(ctx, state.ObjectTypeID.ValueString())
	if readErr != nil {
		appendHubSpotDiagnostic(&res.Diagnostics, "Custom schema destroy preflight failed", readErr)
		return
	}
	var owned map[string]schemaPropertyModel
	_ = state.Properties.ElementsAs(ctx, &owned, false)
	for _, prop := range current.Properties {
		if _, ok := owned[prop.Name]; !ok {
			res.Diagnostics.AddError("External schema property blocks destroy", "A property outside this resource's owned property map remains on the schema; remove it through its owning resource first.")
			return
		}
	}
	if err := r.client.Archive(ctx, state.ObjectTypeID.ValueString()); err != nil {
		if isNotFound(err) {
			res.State.RemoveResource(ctx)
			return
		}
		appendHubSpotDiagnostic(&res.Diagnostics, "Custom schema archival failed", err)
		return
	}
	res.State.RemoveResource(ctx)
}
func (r *CustomSchemaResource) ImportState(ctx context.Context, req resource.ImportStateRequest, res *resource.ImportStateResponse) {
	if strings.TrimSpace(req.ID) == "" {
		res.Diagnostics.AddAttributeError(path.Root("id"), "Invalid schema import ID", "Use an objectTypeId or fully qualified name.")
		return
	}
	out, err := r.client.Get(ctx, req.ID)
	if err != nil {
		appendHubSpotDiagnostic(&res.Diagnostics, "Custom schema import failed", err)
		return
	}
	res.Diagnostics.Append(res.State.Set(ctx, modelFromSchema(out))...)
}
func schemaWriteFromModel(ctx context.Context, m customSchemaModel) (hubspot.SchemaWrite, error) {
	if m.Properties.IsNull() || m.Properties.IsUnknown() {
		return hubspot.SchemaWrite{}, errors.New("custom schema requires nonempty properties")
	}
	var props map[string]schemaPropertyModel
	if diags := m.Properties.ElementsAs(ctx, &props, false); diags.HasError() || len(props) == 0 {
		return hubspot.SchemaWrite{}, errors.New("custom schema requires nonempty properties")
	}
	labels := m.Labels.Attributes()
	singular, ok1 := labels["singular"].(types.String)
	plural, ok2 := labels["plural"].(types.String)
	if !ok1 || !ok2 {
		return hubspot.SchemaWrite{}, errors.New("invalid labels")
	}
	out := hubspot.SchemaWrite{Name: m.Name.ValueString(), Labels: map[string]string{"singular": singular.ValueString(), "plural": plural.ValueString()}, Description: m.Description.ValueString(), PrimaryDisplayProperty: m.PrimaryDisplayProperty.ValueString(), AllowsSensitiveProperties: m.AllowsSensitiveProperties.ValueBool(), ShouldCreateSameObjectAssociation: m.ShouldCreateSameObjectAssociation.ValueBool()}
	for k, p := range props {
		out.Properties = append(out.Properties, hubspot.SchemaProperty{Name: k, Label: p.Label.ValueString(), Type: p.Type.ValueString(), FieldType: p.FieldType.ValueString(), Description: p.Description.ValueString(), DisplayOrder: p.DisplayOrder.ValueInt64(), FormField: p.FormField.ValueBool(), Hidden: p.Hidden.ValueBool(), HasUniqueValue: p.HasUniqueValue.ValueBool(), ShowCurrencySymbol: p.ShowCurrencySymbol.ValueBool()})
	}
	return out, nil
}
func modelFromSchema(s hubspot.CustomSchema) customSchemaModel {
	labels := types.ObjectValueMust(map[string]attr.Type{"singular": types.StringType, "plural": types.StringType}, map[string]attr.Value{"singular": types.StringValue(s.Labels.Singular), "plural": types.StringValue(s.Labels.Plural)})
	vals := map[string]attr.Value{}
	for _, p := range s.Properties {
		vals[p.Name] = types.ObjectValueMust(map[string]attr.Type{"label": types.StringType, "type": types.StringType, "field_type": types.StringType, "description": types.StringType, "display_order": types.Int64Type, "form_field": types.BoolType, "hidden": types.BoolType, "has_unique_value": types.BoolType, "show_currency_symbol": types.BoolType, "options": types.MapType{ElemType: types.StringType}}, map[string]attr.Value{"label": types.StringValue(p.Label), "type": types.StringValue(p.Type), "field_type": types.StringValue(p.FieldType), "description": types.StringValue(p.Description), "display_order": types.Int64Value(p.DisplayOrder), "form_field": types.BoolValue(p.FormField), "hidden": types.BoolValue(p.Hidden), "has_unique_value": types.BoolValue(p.HasUniqueValue), "show_currency_symbol": types.BoolValue(p.ShowCurrencySymbol), "options": types.MapNull(types.StringType)})
	}
	return customSchemaModel{ID: types.StringValue(s.ObjectTypeID), ObjectTypeID: types.StringValue(s.ObjectTypeID), FullyQualifiedName: types.StringValue(s.FullyQualifiedName), Name: types.StringValue(s.Name), Labels: labels, PrimaryDisplayProperty: types.StringValue(s.PrimaryDisplayProperty), Description: types.StringValue(s.Description), Properties: types.MapValueMust(types.ObjectType{AttrTypes: map[string]attr.Type{"label": types.StringType, "type": types.StringType, "field_type": types.StringType, "description": types.StringType, "display_order": types.Int64Type, "form_field": types.BoolType, "hidden": types.BoolType, "has_unique_value": types.BoolType, "show_currency_symbol": types.BoolType, "options": types.MapType{ElemType: types.StringType}}}, vals)}
}

type setRequiresReplace struct{}

func (setRequiresReplace) Description(context.Context) string             { return "changes require replacement" }
func (v setRequiresReplace) MarkdownDescription(c context.Context) string { return v.Description(c) }
func (setRequiresReplace) PlanModifySet(_ context.Context, req planmodifier.SetRequest, res *planmodifier.SetResponse) {
	if !req.PlanValue.Equal(req.StateValue) {
		res.RequiresReplace = true
	}
}
