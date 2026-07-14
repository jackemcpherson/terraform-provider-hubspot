// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestProviderMetadata(t *testing.T) {
	p := New("1.2.3")().(*Provider)
	var response provider.MetadataResponse
	p.Metadata(context.Background(), provider.MetadataRequest{}, &response)

	if response.TypeName != "hubspot" {
		t.Fatalf("type name = %q, want hubspot", response.TypeName)
	}
	if response.Version != "1.2.3" {
		t.Fatalf("version = %q, want 1.2.3", response.Version)
	}
}

func TestProviderSchema(t *testing.T) {
	p := New("test")()
	var response provider.SchemaResponse
	p.Schema(context.Background(), provider.SchemaRequest{}, &response)

	if response.Schema.Attributes["access_token"].IsSensitive() != true {
		t.Fatal("access_token must be sensitive")
	}
	if response.Schema.Attributes["api_base_url"].IsOptional() != true {
		t.Fatal("api_base_url must be optional")
	}
}

func TestDefaultAPIBaseURL(t *testing.T) {
	if types.StringValue(defaultAPIBaseURL).ValueString() != "https://api.hubapi.com" {
		t.Fatal("unexpected default API base URL")
	}
}

func TestProviderServesProtocol6(t *testing.T) {
	if _, err := providerserver.NewProtocol6WithError(New("test")())(); err != nil {
		t.Fatalf("protocol 6 server construction failed: %v", err)
	}
}

func TestAPIBaseURLValidator(t *testing.T) {
	tests := []struct {
		name  string
		value string
		valid bool
	}{
		{name: "default HTTPS", value: "https://api.hubapi.com", valid: true},
		{name: "path prefix", value: "https://localhost:8443/test", valid: true},
		{name: "loopback HTTP", value: "http://127.0.0.1:8080", valid: true},
		{name: "plain host", value: "api.hubapi.com", valid: false},
		{name: "public HTTP", value: "http://api.hubapi.com", valid: false},
		{name: "userinfo", value: "https://user@example.com", valid: false},
		{name: "query", value: "https://api.hubapi.com?token=bad", valid: false},
		{name: "surrounding whitespace", value: " https://api.hubapi.com ", valid: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := validator.StringResponse{}
			apiBaseURLValidator{}.ValidateString(context.Background(), validator.StringRequest{
				Path:        path.Root("api_base_url"),
				ConfigValue: types.StringValue(test.value),
			}, &response)
			if got := !response.Diagnostics.HasError(); got != test.valid {
				t.Fatalf("valid = %v, want %v; diagnostics = %#v", got, test.valid, response.Diagnostics)
			}
		})
	}
}
