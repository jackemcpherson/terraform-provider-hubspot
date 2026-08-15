// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/jackemcpherson/terraform-provider-hubspot/internal/hubspot"
)

func TestProductSchemaMatchesFrozenContract(t *testing.T) {
	var response resource.SchemaResponse
	NewProductResource().Schema(context.Background(), resource.SchemaRequest{}, &response)
	attributes := response.Schema.Attributes
	if response.Schema.Version != 0 || len(attributes) != 7 {
		t.Fatalf("Product schema version/shape = %d/%d", response.Schema.Version, len(attributes))
	}
	if !attributes["id"].IsComputed() {
		t.Fatal("id must be computed")
	}
	for _, name := range []string{"name", "sku", "description", "price"} {
		if !attributes[name].IsRequired() {
			t.Fatalf("%s must be required", name)
		}
	}
	for _, name := range []string{"cost", "recurring_billing_period"} {
		if !attributes[name].IsOptional() || attributes[name].IsComputed() {
			t.Fatalf("%s must be optional and not computed", name)
		}
	}
}

func TestProductPatchSendsOnlyChangedManagedProperties(t *testing.T) {
	state := productResourceModel{
		ID: types.StringValue("701"), Name: types.StringValue("Support"),
		SKU: types.StringValue("S-1"), Description: types.StringValue("Annual"),
		Price: types.StringValue("1200.00"), Cost: types.StringNull(),
		RecurringBillingPeriod: types.StringValue("P12M"),
	}
	plan := state
	plan.Price = types.StringValue("1200")
	plan.Cost = types.StringValue("")
	plan.RecurringBillingPeriod = types.StringValue("")
	patch := productPatchFromModels(state, plan)
	want := map[string]string{
		"hs_cost_of_goods_sold":       "",
		"hs_recurring_billing_period": "",
	}
	if !reflect.DeepEqual(patch, want) {
		t.Fatalf("patch = %#v, want %#v", patch, want)
	}
}

func TestProductImportRejectsMissingRequiredRuntimeProperties(t *testing.T) {
	_, err := productModelFromRemote(hubspot.Product{
		ID: "701", Name: "", SKU: "S-1", Description: "Annual", Price: "1200",
	}, nil)
	if err == nil {
		t.Fatal("Product import model accepted an empty required name")
	}
}

func TestProductRefreshRejectsUnsupportedManagedOptionalRuntimeValues(t *testing.T) {
	prior := productResourceModel{
		ID: types.StringValue("701"), Name: types.StringValue("Support"), SKU: types.StringValue("S-1"),
		Description: types.StringValue("Annual"), Price: types.StringValue("1200.00"),
		Cost: types.StringValue("300.00"), RecurringBillingPeriod: types.StringValue("P12M"),
	}
	remote := hubspot.Product{
		ID: "701", Name: "Support", SKU: "S-1", Description: "Annual", Price: "1200",
		Cost: "not-a-decimal", RecurringBillingPeriod: "P12M",
	}
	if _, err := productModelFromRemote(remote, &prior); err == nil {
		t.Fatal("Product refresh accepted an unsupported managed cost")
	}
	remote.Cost = "300"
	remote.RecurringBillingPeriod = "P1Y"
	if _, err := productModelFromRemote(remote, &prior); err == nil {
		t.Fatal("Product refresh accepted an unsupported managed recurrence")
	}
}

func TestProductDecimalAndRecurrenceValidation(t *testing.T) {
	decimalTests := []struct {
		value string
		valid bool
	}{{"0", true}, {"1200.00", true}, {"01", false}, {"1.", false}, {".5", false}, {"-1", false}, {"1e3", false}, {"", false}}
	for _, test := range decimalTests {
		response := validator.StringResponse{}
		productDecimalValidator{allowEmpty: false}.ValidateString(context.Background(), validator.StringRequest{
			Path: path.Root("price"), ConfigValue: types.StringValue(test.value),
		}, &response)
		if got := !response.Diagnostics.HasError(); got != test.valid {
			t.Errorf("decimal %q valid = %v, want %v", test.value, got, test.valid)
		}
	}
	for _, test := range []struct {
		value string
		valid bool
	}{{"", true}, {"P1M", true}, {"P12M", true}, {"P0M", false}, {"P1Y", false}, {"p1m", false}} {
		response := validator.StringResponse{}
		productRecurrenceValidator{}.ValidateString(context.Background(), validator.StringRequest{
			Path: path.Root("recurring_billing_period"), ConfigValue: types.StringValue(test.value),
		}, &response)
		if got := !response.Diagnostics.HasError(); got != test.valid {
			t.Errorf("recurrence %q valid = %v, want %v", test.value, got, test.valid)
		}
	}
}

func TestProductDecimalEqualityIsSemantic(t *testing.T) {
	if !productDecimalsEqual("1200.00", "1200") {
		t.Fatal("equivalent decimal strings did not compare equal")
	}
	if productDecimalsEqual("1200.01", "1200") {
		t.Fatal("different decimal strings compared equal")
	}
	if productDecimalsEqual("", "0") || !productDecimalsEqual("", "") {
		t.Fatal("empty decimal clearing semantics are incorrect")
	}
}
