// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

package docsportal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"

	frameworkprovider "github.com/hashicorp/terraform-plugin-framework/provider"
)

type Config struct {
	Provider               frameworkprovider.Provider
	ProviderRepo           string
	DemoRepo               string
	OutputDir              string
	Version                string
	Released               bool
	RequireClean           bool
	ExpectedProviderCommit string
	ExpectedDemoCommit     string
	ProviderProvenance     Provenance
	DemoProvenance         Provenance
}

type Provenance struct {
	Commit    string `json:"commit"`
	Timestamp string `json:"timestamp"`
	Dirty     bool   `json:"dirty"`
}

type manifest struct {
	Version     string     `json:"version"`
	State       string     `json:"state"`
	GeneratedAt string     `json:"generated_at"`
	Provider    Provenance `json:"provider"`
	Demo        Provenance `json:"demo"`
	Resources   []string   `json:"resources"`
	DataSources []string   `json:"data_sources"`
	Modules     []string   `json:"modules"`
}

// Generate discovers the provider and consumer-module interfaces, then renders
// and validates a deterministic documentation portal for those exact sources.
func Generate(ctx context.Context, config Config) error {
	if config.Provider == nil || config.ProviderRepo == "" || config.DemoRepo == "" || config.OutputDir == "" || config.Version == "" {
		return errors.New("provider, repositories, output directory, and version are required")
	}
	absOutput, err := filepath.Abs(config.OutputDir)
	if err != nil {
		return err
	}
	absProvider, _ := filepath.Abs(config.ProviderRepo)
	absDemo, _ := filepath.Abs(config.DemoRepo)
	if filepath.Dir(absOutput) == absOutput || absOutput == absProvider || absOutput == absDemo {
		return errors.New("portal output must be a dedicated non-root directory")
	}
	config.OutputDir = absOutput
	registrar, ok := config.Provider.(providerRegistrar)
	if !ok {
		return errors.New("provider does not expose registered resources and data sources")
	}
	if err := resolveProvenance(&config); err != nil {
		return err
	}

	resources, err := discoverResources(ctx, registrar, config.ProviderRepo)
	if err != nil {
		return err
	}
	dataSources, err := discoverDataSources(ctx, registrar, config.ProviderRepo)
	if err != nil {
		return err
	}
	modules, err := discoverModules(config.DemoRepo)
	if err != nil {
		return err
	}
	if err := requireSurface(resources, dataSources, modules); err != nil {
		return err
	}

	if err := resetOutput(config.OutputDir); err != nil {
		return err
	}
	metadata := portalManifest(config, resources, dataSources, modules)
	encoded, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return err
	}
	if err := writeFile(filepath.Join(config.OutputDir, "provenance.json"), string(encoded)+"\n"); err != nil {
		return err
	}
	if err := renderPortal(config, metadata, resources, dataSources, modules); err != nil {
		return err
	}
	if err := ValidateLinks(config.OutputDir); err != nil {
		return err
	}
	return ValidateRenderedHTML(config.OutputDir)
}

func requireSurface(resources, dataSources []providerType, modules []moduleDoc) error {
	if !containsModule(modules, "crm-schema") {
		return errors.New("required consumer module crm-schema was not discovered from HCL")
	}
	for _, required := range []string{"hubspot_property_group", "hubspot_property"} {
		if !containsProviderType(resources, required) {
			return fmt.Errorf("required registered resource %s was not discovered", required)
		}
	}
	for _, required := range []string{"hubspot_property_definition", "hubspot_property_definitions"} {
		if !containsProviderType(dataSources, required) {
			return fmt.Errorf("required registered data source %s was not discovered", required)
		}
	}
	return nil
}

func portalManifest(config Config, resources, dataSources []providerType, modules []moduleDoc) manifest {
	state := "unreleased"
	if config.Released {
		state = "released"
	}
	return manifest{
		Version: config.Version, State: state, GeneratedAt: config.ProviderProvenance.Timestamp,
		Provider: config.ProviderProvenance, Demo: config.DemoProvenance,
		Resources: namesOfProviderTypes(resources), DataSources: namesOfProviderTypes(dataSources), Modules: namesOfModules(modules),
	}
}
