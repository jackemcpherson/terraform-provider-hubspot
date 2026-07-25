// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"reflect"
	"sort"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	fwprovider "github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

// TestSchemaAttributesHaveMarkdownDescriptions is the consumer-facing
// contract gate: attribute descriptions are what the Terraform Registry and
// editor hovers show, so every provider, resource, and data-source
// attribute (including attributes and blocks nested arbitrarily deep) must
// carry a non-empty MarkdownDescription. A missing MarkdownDescription is
// not covered by a plain Description fallback: the framework's
// GetMarkdownDescription() returns the MarkdownDescription field verbatim.
func TestSchemaAttributesHaveMarkdownDescriptions(t *testing.T) {
	ctx := context.Background()
	var gaps []string

	p := New("test")().(*Provider)

	var providerResponse fwprovider.SchemaResponse
	p.Schema(ctx, fwprovider.SchemaRequest{}, &providerResponse)
	gaps = append(gaps, describeGaps("provider", providerResponse.Schema.Attributes, providerResponse.Schema.Blocks)...)

	for _, newResource := range p.Resources(ctx) {
		r := newResource()
		var metadata resource.MetadataResponse
		r.Metadata(ctx, resource.MetadataRequest{ProviderTypeName: "hubspot"}, &metadata)
		var schemaResponse resource.SchemaResponse
		r.Schema(ctx, resource.SchemaRequest{}, &schemaResponse)
		gaps = append(gaps, describeGaps("resource "+metadata.TypeName, schemaResponse.Schema.Attributes, schemaResponse.Schema.Blocks)...)
	}

	for _, newDataSource := range p.DataSources(ctx) {
		d := newDataSource()
		var metadata datasource.MetadataResponse
		d.Metadata(ctx, datasource.MetadataRequest{ProviderTypeName: "hubspot"}, &metadata)
		var schemaResponse datasource.SchemaResponse
		d.Schema(ctx, datasource.SchemaRequest{}, &schemaResponse)
		gaps = append(gaps, describeGaps("data source "+metadata.TypeName, schemaResponse.Schema.Attributes, schemaResponse.Schema.Blocks)...)
	}

	sort.Strings(gaps)
	if len(gaps) > 0 {
		t.Fatalf("attributes missing a MarkdownDescription:\n%s", joinLines(gaps))
	}
}

// markdownDescriber is satisfied structurally by every framework Attribute
// and Block implementation across the provider, resource, and data-source
// schema packages, which each define their own concrete types with this
// exact method. Declaring it locally avoids depending on the framework's
// internal fwschema package while still using the framework's own accessor
// semantics rather than re-deriving them.
type markdownDescriber interface {
	GetMarkdownDescription() string
}

// describeGaps walks a top-level attribute/block map pair and returns the
// dotted attribute paths whose MarkdownDescription is empty, recursing into
// every nested attribute and block shape (list/map/set/single nested
// attributes and blocks) regardless of which of the three schema packages
// (provider, resource, data source) they came from.
func describeGaps(root string, attributes, blocks any) []string {
	var gaps []string
	gaps = append(gaps, walkContainer(root+".", attributes)...)
	gaps = append(gaps, walkContainer(root+".", blocks)...)
	return gaps
}

func walkContainer(prefix string, container any) []string {
	var gaps []string
	if container == nil {
		return gaps
	}
	value := reflect.ValueOf(container)
	if value.Kind() != reflect.Map {
		return gaps
	}
	names := make([]string, 0, value.Len())
	for _, key := range value.MapKeys() {
		names = append(names, key.String())
	}
	sort.Strings(names)
	for _, name := range names {
		entry := value.MapIndex(reflect.ValueOf(name)).Interface()
		gaps = append(gaps, walkEntry(prefix+name, entry)...)
	}
	return gaps
}

func walkEntry(path string, entry any) []string {
	var gaps []string
	if describer, ok := entry.(markdownDescriber); ok {
		if describer.GetMarkdownDescription() == "" {
			gaps = append(gaps, path)
		}
	}

	rv := reflect.ValueOf(entry)
	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return gaps
	}

	holder := rv
	if nested := rv.FieldByName("NestedObject"); nested.IsValid() && nested.Kind() == reflect.Struct {
		holder = nested
	}
	if attrs := holder.FieldByName("Attributes"); attrs.IsValid() && attrs.Kind() == reflect.Map {
		gaps = append(gaps, walkContainer(path+".", attrs.Interface())...)
	}
	if childBlocks := holder.FieldByName("Blocks"); childBlocks.IsValid() && childBlocks.Kind() == reflect.Map {
		gaps = append(gaps, walkContainer(path+".", childBlocks.Interface())...)
	}
	return gaps
}

func joinLines(lines []string) string {
	joined := ""
	for _, line := range lines {
		joined += "  " + line + "\n"
	}
	return joined
}
