// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
)

func TestFileFolderSchemaMatchesFrozenContract(t *testing.T) {
	var response resource.SchemaResponse
	NewFileFolderResource().Schema(context.Background(), resource.SchemaRequest{}, &response)
	attributes := response.Schema.Attributes
	if response.Schema.Version != 0 || len(attributes) != 6 {
		t.Fatalf("folder schema version/shape = %d/%d", response.Schema.Version, len(attributes))
	}
	if !attributes["id"].IsComputed() || !attributes["name"].IsRequired() || !attributes["parent_folder_id"].IsOptional() || !attributes["path"].IsComputed() || !attributes["created_at"].IsComputed() || !attributes["updated_at"].IsComputed() {
		t.Fatal("folder schema did not preserve required, optional, and computed boundaries")
	}
}

func TestManagedFileSchemaMatchesFrozenContract(t *testing.T) {
	var response resource.SchemaResponse
	NewFileResource().Schema(context.Background(), resource.SchemaRequest{}, &response)
	attributes := response.Schema.Attributes
	if response.Schema.Version != 0 || len(attributes) != 18 {
		t.Fatalf("file schema version/shape = %d/%d", response.Schema.Version, len(attributes))
	}
	if !attributes["id"].IsComputed() || !attributes["name"].IsRequired() || !attributes["folder_id"].IsRequired() || !attributes["source_path"].IsRequired() || !attributes["source_path"].IsSensitive() || !attributes["source_sha256"].IsRequired() {
		t.Fatal("managed file input boundary is incorrect")
	}
	if !attributes["access"].IsOptional() || !attributes["access"].IsComputed() {
		t.Fatal("access must be optional and computed with a safe default")
	}
	for _, name := range []string{"path", "file_md5", "size", "extension", "type", "encoding", "height", "width", "url", "default_hosting_url", "created_at", "updated_at"} {
		if !attributes[name].IsComputed() {
			t.Fatalf("%s must be computed", name)
		}
	}
}
