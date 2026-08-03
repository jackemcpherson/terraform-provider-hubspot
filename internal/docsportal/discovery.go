// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

package docsportal

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

type providerRegistrar interface {
	Resources(context.Context) []func() resource.Resource
	DataSources(context.Context) []func() datasource.DataSource
}

type attributeDoc struct {
	Name        string
	Description string
	Mode        string
}

type providerType struct {
	Name        string
	Description string
	Attributes  []attributeDoc
	Example     string
}

type moduleDoc struct {
	Name      string
	Variables []string
	Resources []string
	Outputs   []string
	Sources   map[string]string
	Usage     map[string]string
	Guide     string
}

var (
	blockNamePattern = regexp.MustCompile(`(?m)^\s*(variable|output)\s+"([^"]+)"\s*\{`)
	resourcePattern  = regexp.MustCompile(`(?m)^\s*resource\s+"([^"]+)"\s+"[^"]+"\s*\{`)
)

func discoverResources(ctx context.Context, registrar providerRegistrar, providerRepo string) ([]providerType, error) {
	return discoverProviderTypes(registrar.Resources(ctx), func(entry resource.Resource) (providerType, error) {
		var metadata resource.MetadataResponse
		entry.Metadata(ctx, resource.MetadataRequest{}, &metadata)
		var response resource.SchemaResponse
		entry.Schema(ctx, resource.SchemaRequest{}, &response)
		if response.Diagnostics.HasError() {
			return providerType{}, fmt.Errorf("resource schema %s returned diagnostics", metadata.TypeName)
		}
		return providerType{Name: metadata.TypeName, Description: response.Schema.Description, Attributes: schemaAttributes(response.Schema.Attributes), Example: resourceExample(providerRepo, metadata.TypeName)}, nil
	})
}

func discoverDataSources(ctx context.Context, registrar providerRegistrar, providerRepo string) ([]providerType, error) {
	return discoverProviderTypes(registrar.DataSources(ctx), func(entry datasource.DataSource) (providerType, error) {
		var metadata datasource.MetadataResponse
		entry.Metadata(ctx, datasource.MetadataRequest{}, &metadata)
		var response datasource.SchemaResponse
		entry.Schema(ctx, datasource.SchemaRequest{}, &response)
		if response.Diagnostics.HasError() {
			return providerType{}, fmt.Errorf("data source schema %s returned diagnostics", metadata.TypeName)
		}
		return providerType{Name: metadata.TypeName, Description: response.Schema.Description, Attributes: schemaAttributes(response.Schema.Attributes), Example: dataSourceExample(providerRepo, metadata.TypeName)}, nil
	})
}

func discoverProviderTypes[T any](factories []func() T, inspect func(T) (providerType, error)) ([]providerType, error) {
	entries := make([]providerType, 0, len(factories))
	for _, factory := range factories {
		entry, err := inspect(factory())
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
	return entries, nil
}

type schemaAttribute interface {
	GetDescription() string
	IsRequired() bool
	IsOptional() bool
	IsComputed() bool
}

func schemaAttributes[T schemaAttribute](attributes map[string]T) []attributeDoc {
	result := make([]attributeDoc, 0, len(attributes))
	for name, attribute := range attributes {
		result = append(result, attributeDoc{Name: name, Description: attribute.GetDescription(), Mode: attributeMode(attribute.IsRequired(), attribute.IsOptional(), attribute.IsComputed())})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}

func attributeMode(required, optional, computed bool) string {
	parts := make([]string, 0, 2)
	if required {
		parts = append(parts, "required")
	}
	if optional {
		parts = append(parts, "optional")
	}
	if computed {
		parts = append(parts, "computed")
	}
	return strings.Join(parts, ", ")
}

func discoverModules(demoRepo string) ([]moduleDoc, error) {
	root := filepath.Join(demoRepo, "modules")
	directories, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("discover consumer modules: %w", err)
	}
	modules := make([]moduleDoc, 0)
	for _, directory := range directories {
		if !directory.IsDir() {
			continue
		}
		path := filepath.Join(root, directory.Name())
		files, err := filepath.Glob(filepath.Join(path, "*.tf"))
		if err != nil || len(files) == 0 {
			continue
		}
		module := moduleDoc{
			Name: directory.Name(), Sources: make(map[string]string), Usage: make(map[string]string),
			Guide: readOptional(filepath.Join(path, "README.md")),
		}
		for _, file := range files {
			contents, readErr := os.ReadFile(file)
			if readErr != nil {
				return nil, readErr
			}
			text := string(contents)
			module.Sources[filepath.Base(file)] = text
			for _, match := range blockNamePattern.FindAllStringSubmatch(text, -1) {
				if match[1] == "variable" {
					module.Variables = append(module.Variables, match[2])
				} else {
					module.Outputs = append(module.Outputs, match[2])
				}
			}
			for _, match := range resourcePattern.FindAllStringSubmatch(text, -1) {
				module.Resources = append(module.Resources, match[1])
			}
		}
		sort.Strings(module.Variables)
		sort.Strings(module.Resources)
		sort.Strings(module.Outputs)
		rootFiles, globErr := filepath.Glob(filepath.Join(demoRepo, "*.tf"))
		if globErr != nil {
			return nil, globErr
		}
		usesModule := false
		rootSources := make(map[string]string, len(rootFiles))
		for _, file := range rootFiles {
			contents, readErr := os.ReadFile(file)
			if readErr != nil {
				return nil, readErr
			}
			rootSources[filepath.Base(file)] = string(contents)
			usesModule = usesModule || strings.Contains(string(contents), "./modules/"+module.Name)
		}
		if usesModule {
			module.Usage = rootSources
		}
		exampleFiles, globErr := filepath.Glob(filepath.Join(demoRepo, "examples", module.Name, "*.tf"))
		if globErr != nil {
			return nil, globErr
		}
		if len(exampleFiles) > 0 {
			module.Usage = make(map[string]string, len(exampleFiles))
			for _, file := range exampleFiles {
				contents, readErr := os.ReadFile(file)
				if readErr != nil {
					return nil, readErr
				}
				module.Usage[filepath.Base(file)] = string(contents)
			}
		}
		modules = append(modules, module)
	}
	sort.Slice(modules, func(i, j int) bool { return modules[i].Name < modules[j].Name })
	return modules, nil
}

func resourceExample(repo, name string) string {
	short := strings.TrimPrefix(name, "hubspot_")
	return readOptional(filepath.Join(repo, "examples", "resources", name, "resource.tf"), filepath.Join(repo, "examples", short, "main.tf"))
}

func dataSourceExample(repo, name string) string {
	return readOptional(filepath.Join(repo, "examples", "data-sources", name, "data-source.tf"))
}

func readOptional(paths ...string) string {
	for _, path := range paths {
		if contents, err := os.ReadFile(path); err == nil {
			return string(contents)
		}
	}
	return ""
}

func containsProviderType(entries []providerType, name string) bool {
	for _, entry := range entries {
		if entry.Name == name {
			return true
		}
	}
	return false
}

func containsModule(entries []moduleDoc, name string) bool {
	for _, entry := range entries {
		if entry.Name == name {
			return true
		}
	}
	return false
}

func namesOfProviderTypes(entries []providerType) []string {
	names := make([]string, len(entries))
	for index, entry := range entries {
		names[index] = entry.Name
	}
	return names
}

func namesOfModules(entries []moduleDoc) []string {
	names := make([]string, len(entries))
	for index, entry := range entries {
		names[index] = entry.Name
	}
	return names
}
