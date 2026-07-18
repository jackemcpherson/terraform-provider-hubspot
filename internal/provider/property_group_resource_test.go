// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/jackemcpherson/terraform-provider-hubspot/internal/hubspot"
)

func TestPropertyGroupResourceSchema(t *testing.T) {
	resourceUnderTest := NewPropertyGroupResource()
	var response resource.SchemaResponse
	resourceUnderTest.Schema(context.Background(), resource.SchemaRequest{}, &response)

	for _, name := range []string{"id", "object_type", "name", "label", "display_order"} {
		if _, ok := response.Schema.Attributes[name]; !ok {
			t.Fatalf("missing attribute %q", name)
		}
	}
	if !response.Schema.Attributes["id"].IsComputed() {
		t.Fatal("id must be computed")
	}
	if !response.Schema.Attributes["object_type"].IsRequired() || !response.Schema.Attributes["name"].IsRequired() {
		t.Fatal("identity attributes must be required")
	}
	if !response.Schema.Attributes["display_order"].IsOptional() {
		t.Fatal("display_order must be optional")
	}
}

func TestPropertyDefinitionDataSourceSchemas(t *testing.T) {
	for _, source := range []datasource.DataSource{NewPropertyDefinitionDataSource(), NewPropertyDefinitionsDataSource()} {
		var response datasource.SchemaResponse
		source.Schema(context.Background(), datasource.SchemaRequest{}, &response)
		for _, name := range []string{"object_type", "archived", "data_sensitivity", "locale"} {
			if _, ok := response.Schema.Attributes[name]; !ok {
				t.Fatalf("missing data source attribute %q", name)
			}
		}
	}
	var singular datasource.SchemaResponse
	NewPropertyDefinitionDataSource().Schema(context.Background(), datasource.SchemaRequest{}, &singular)
	if !singular.Schema.Attributes["date_display_hint"].IsComputed() {
		t.Fatal("date_display_hint must remain discovery-only computed metadata")
	}
}

func TestPropertyResourceSchema(t *testing.T) {
	var response resource.SchemaResponse
	NewPropertyResource().Schema(context.Background(), resource.SchemaRequest{}, &response)
	for _, name := range []string{"id", "object_type", "name", "label", "group_name", "type", "field_type", "options"} {
		if _, ok := response.Schema.Attributes[name]; !ok {
			t.Fatalf("missing property attribute %q", name)
		}
	}
	if _, ok := response.Schema.Attributes["date_display_hint"]; ok {
		t.Fatal("date_display_hint must not be configurable on managed properties")
	}
}

func TestPropertyWriteSortsOptionValuesDeterministically(t *testing.T) {
	options := map[string]attr.Value{
		"zeta": types.ObjectValueMust(optionAttrTypes(), map[string]attr.Value{
			"label": types.StringValue("Zeta"), "description": types.StringValue(""), "display_order": types.Int64Value(-1), "hidden": types.BoolValue(false),
		}),
		"alpha": types.ObjectValueMust(optionAttrTypes(), map[string]attr.Value{
			"label": types.StringValue("Alpha"), "description": types.StringValue(""), "display_order": types.Int64Value(-1), "hidden": types.BoolValue(false),
		}),
	}
	write, err := propertyWriteFromModel(context.Background(), propertyResourceModel{
		Name: types.StringValue("test"), Label: types.StringValue("Test"), GroupName: types.StringValue("contactinformation"),
		Type: types.StringValue("enumeration"), FieldType: types.StringValue("select"),
		Options: types.MapValueMust(types.ObjectType{AttrTypes: optionAttrTypes()}, options),
	})
	if err != nil {
		t.Fatalf("propertyWriteFromModel: %v", err)
	}
	if len(write.Options) != 2 || write.Options[0].Value != "alpha" || write.Options[1].Value != "zeta" {
		t.Fatalf("option order = %#v", write.Options)
	}
}

func TestAppendOrderNormalizationExposesRemoteDrift(t *testing.T) {
	orderTen := int64(10)
	orderTwenty := int64(20)
	definition := hubspot.PropertyDefinition{Name: "managed", GroupName: "group", DisplayOrder: &orderTen}
	if propertyIsLastInGroup(definition, []hubspot.PropertyDefinition{definition, {Name: "other", GroupName: "group", DisplayOrder: &orderTwenty}}) {
		t.Fatal("property append sentinel must not hide a remote move away from the end of its group")
	}

	configured := propertyOptionMap(map[string]propertyOptionModel{
		"alpha": {Label: types.StringValue("Alpha"), Description: types.StringValue(""), DisplayOrder: types.Int64Value(-1), Hidden: types.BoolValue(false)},
		"beta":  {Label: types.StringValue("Beta"), Description: types.StringValue(""), DisplayOrder: types.Int64Value(-1), Hidden: types.BoolValue(false)},
	})
	model := propertyResourceModel{Options: propertyOptionMap(map[string]propertyOptionModel{
		"alpha": {Label: types.StringValue("Alpha"), Description: types.StringValue(""), DisplayOrder: types.Int64Value(20), Hidden: types.BoolValue(false)},
		"beta":  {Label: types.StringValue("Beta"), Description: types.StringValue(""), DisplayOrder: types.Int64Value(10), Hidden: types.BoolValue(false)},
	})}
	preserveConfiguredOptionAppendOrders(context.Background(), &model, configured)
	var observed map[string]propertyOptionModel
	if diagnostics := model.Options.ElementsAs(context.Background(), &observed, false); diagnostics.HasError() {
		t.Fatal("decode normalized option state")
	}
	if observed["alpha"].DisplayOrder.ValueInt64() != 20 || observed["beta"].DisplayOrder.ValueInt64() != 10 {
		t.Fatal("option append sentinels hid remote option-order drift")
	}
}

func TestPropertyOptionsPlanClearsStaleEnumerationOptionsOnStorageChange(t *testing.T) {
	prior := propertyOptionMap(map[string]propertyOptionModel{
		"alpha": {Label: types.StringValue("Alpha"), Description: types.StringValue(""), DisplayOrder: types.Int64Value(-1), Hidden: types.BoolValue(false)},
	})
	cleared := propertyOptionsPlanValue(types.StringValue("string"), types.StringValue("enumeration"), prior)
	if cleared.IsNull() || len(cleared.Elements()) != 0 {
		t.Fatal("enumeration options remained planned after changing to scalar storage")
	}
	preserved := propertyOptionsPlanValue(types.StringValue("enumeration"), types.StringValue("enumeration"), prior)
	if !preserved.Equal(prior) {
		t.Fatal("unchanged enumeration options were not preserved for an unrelated update")
	}
}
