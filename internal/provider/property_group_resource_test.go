// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
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
