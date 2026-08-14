// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/jackemcpherson/terraform-provider-hubspot/internal/hubspot"
)

func TestAccountMembershipSchemaMatchesFrozenContract(t *testing.T) {
	var response resource.SchemaResponse
	NewAccountMembershipResource().Schema(context.Background(), resource.SchemaRequest{}, &response)
	attributes := response.Schema.Attributes
	if response.Schema.Version != 0 || len(attributes) != 7 {
		t.Fatalf("account membership schema version/shape = %d/%d", response.Schema.Version, len(attributes))
	}
	if !attributes["id"].IsComputed() || !attributes["email"].IsRequired() {
		t.Fatal("account membership identity boundary is incorrect")
	}
	for _, name := range []string{"first_name", "last_name"} {
		if !attributes[name].IsOptional() || !attributes[name].IsComputed() {
			t.Fatalf("%s must be optional and computed", name)
		}
	}
	if !attributes["send_welcome_email"].IsRequired() {
		t.Fatal("send_welcome_email must require an explicit creation choice")
	}
	if !attributes["allow_removal"].IsOptional() || !attributes["allow_removal"].IsComputed() {
		t.Fatal("allow_removal must be an optional local guard with a default")
	}
	if !attributes["super_admin"].IsComputed() {
		t.Fatal("super_admin must be a computed safety observation")
	}
}

func TestConfiguredAccountMembershipNamesMustBeNonblank(t *testing.T) {
	for _, test := range []struct {
		value string
		valid bool
	}{{"First", true}, {" A ", true}, {"", false}, {" ", false}, {"\t\n", false}} {
		response := validator.StringResponse{}
		accountMembershipNameValidator{}.ValidateString(context.Background(), validator.StringRequest{
			Path: path.Root("first_name"), ConfigValue: types.StringValue(test.value),
		}, &response)
		if got := !response.Diagnostics.HasError(); got != test.valid {
			t.Errorf("name %q valid = %v, want %v", test.value, got, test.valid)
		}
	}
}

func TestAccountMembershipCreateAcceptsComputedSuperAdminObservation(t *testing.T) {
	config := accountMembershipResourceModel{
		FirstName: types.StringValue("First"), LastName: types.StringValue("Member"),
	}
	plan := config
	remote := hubspot.AccountMembership{
		ID: "101", Email: "member@example.com", FirstName: "First", LastName: "Member", SuperAdmin: true,
	}
	if !membershipMatchesCreate(remote, "101", "member@example.com", config, plan) {
		t.Fatal("computed Super Admin observation rejected an otherwise exact create recovery")
	}
}
