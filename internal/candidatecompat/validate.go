// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

// Package candidatecompat validates a requested provider version against a
// cumulative consumer checkout before release or live qualification begins.
package candidatecompat

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	version "github.com/hashicorp/go-version"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
)

var packageHashPattern = regexp.MustCompile(`^(h1:[A-Za-z0-9+/]{43}=|zh:[0-9a-f]{64})$`)

type providerConstraint struct {
	file string
	text string
}

type localModuleCall struct {
	file   string
	source string
}

// Validate proves that the cumulative root, its local module graph, and both
// committed engine locks admit and select candidateVersion.
func Validate(candidateVersion, demoRoot string) error {
	releaseVersion := strings.TrimPrefix(candidateVersion, "v")
	candidate, err := version.NewVersion(releaseVersion)
	if err != nil {
		return fmt.Errorf("requested version %q is invalid: %w", candidateVersion, err)
	}

	root, err := filepath.Abs(demoRoot)
	if err != nil {
		return fmt.Errorf("resolve cumulative demo checkout %q: %w", demoRoot, err)
	}
	info, err := os.Stat(root)
	if err != nil {
		return fmt.Errorf("cumulative demo checkout %q is unavailable: %w", demoRoot, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("cumulative demo checkout %q is not a directory", demoRoot)
	}

	if err := validateModuleGraph(root, candidateVersion, candidate); err != nil {
		return err
	}
	for _, lock := range []struct {
		engine  string
		path    string
		address string
	}{
		{engine: "OpenTofu", path: filepath.Join("locks", "tofu", ".terraform.lock.hcl"), address: "registry.opentofu.org/jackemcpherson/hubspot"},
		{engine: "Terraform", path: filepath.Join("locks", "terraform", ".terraform.lock.hcl"), address: "registry.terraform.io/jackemcpherson/hubspot"},
	} {
		if err := validateLock(root, lock.engine, lock.path, lock.address, candidateVersion, releaseVersion, candidate); err != nil {
			return err
		}
	}
	return nil
}

func validateModuleGraph(root, candidateVersion string, candidate *version.Version) error {
	visited := make(map[string]bool)
	queue := []string{root}
	for len(queue) > 0 {
		directory := queue[0]
		queue = queue[1:]
		if visited[directory] {
			continue
		}
		visited[directory] = true

		constraints, calls, err := parseModule(root, directory)
		if err != nil {
			return err
		}
		if len(constraints) == 0 {
			name := "cumulative root"
			if directory != root {
				name = "module " + relativePath(root, directory)
			}
			return fmt.Errorf("%s: HubSpot provider constraint is missing", name)
		}
		for _, constraint := range constraints {
			parsed, err := version.NewConstraint(constraint.text)
			if err != nil {
				return fmt.Errorf("%s: malformed HubSpot provider constraint %q for requested version %s: %w", constraint.file, constraint.text, candidateVersion, err)
			}
			if !parsed.Check(candidate) {
				return fmt.Errorf("%s: HubSpot provider constraint %q excludes requested version %s", constraint.file, constraint.text, candidateVersion)
			}
		}

		for _, call := range calls {
			if !strings.HasPrefix(call.source, "./") && !strings.HasPrefix(call.source, "../") {
				continue
			}
			target := filepath.Clean(filepath.Join(directory, filepath.FromSlash(call.source)))
			withinRoot, err := filepath.Rel(root, target)
			if err != nil || withinRoot == ".." || strings.HasPrefix(withinRoot, ".."+string(filepath.Separator)) {
				return fmt.Errorf("%s: local module %q resolves outside the cumulative demo checkout", call.file, call.source)
			}
			info, err := os.Stat(target)
			if err != nil {
				return fmt.Errorf("%s: local module %q is missing at %s", call.file, call.source, relativePath(root, target))
			}
			if !info.IsDir() {
				return fmt.Errorf("%s: local module %q is not a directory at %s", call.file, call.source, relativePath(root, target))
			}
			queue = append(queue, target)
		}
	}
	return nil
}

func parseModule(root, directory string) ([]providerConstraint, []localModuleCall, error) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, nil, fmt.Errorf("read module %s: %w", relativePath(root, directory), err)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	var constraints []providerConstraint
	var calls []localModuleCall
	foundConfiguration := false
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".tf") {
			continue
		}
		foundConfiguration = true
		path := filepath.Join(directory, entry.Name())
		fileName := relativePath(root, path)
		parser := hclparse.NewParser()
		file, diagnostics := parser.ParseHCLFile(path)
		if diagnostics.HasErrors() {
			return nil, nil, fmt.Errorf("%s: malformed Terraform configuration: %s", fileName, diagnostics.Error())
		}
		body, ok := file.Body.(*hclsyntax.Body)
		if !ok {
			return nil, nil, fmt.Errorf("%s: unsupported Terraform configuration syntax", fileName)
		}
		for _, block := range body.Blocks {
			switch block.Type {
			case "terraform":
				for _, nested := range block.Body.Blocks {
					if nested.Type != "required_providers" {
						continue
					}
					found, err := constraintsFromRequiredProviders(nested.Body, fileName)
					if err != nil {
						return nil, nil, err
					}
					constraints = append(constraints, found...)
				}
			case "module":
				attribute, exists := block.Body.Attributes["source"]
				if !exists {
					return nil, nil, fmt.Errorf("%s: module %q has no source", fileName, firstLabel(block.Labels))
				}
				source, err := constantString(attribute.Expr, fileName, "module source")
				if err != nil {
					return nil, nil, err
				}
				calls = append(calls, localModuleCall{file: fileName, source: source})
			}
		}
	}
	if !foundConfiguration {
		return nil, nil, fmt.Errorf("module %s has no Terraform configuration", relativePath(root, directory))
	}
	return constraints, calls, nil
}

func constraintsFromRequiredProviders(body *hclsyntax.Body, fileName string) ([]providerConstraint, error) {
	var constraints []providerConstraint
	localNames := make([]string, 0, len(body.Attributes))
	for localName := range body.Attributes {
		localNames = append(localNames, localName)
	}
	sort.Strings(localNames)
	for _, localName := range localNames {
		attribute := body.Attributes[localName]
		declaration, diagnostics := attribute.Expr.Value(nil)
		if diagnostics.HasErrors() {
			return nil, fmt.Errorf("%s: required provider %q must be static", fileName, localName)
		}
		if !declaration.IsKnown() || declaration.IsNull() || !declaration.Type().IsObjectType() {
			return nil, fmt.Errorf("%s: required provider %q must be a static object", fileName, localName)
		}
		attributes := declaration.AsValueMap()
		sourceValue, exists := attributes["source"]
		if !exists || sourceValue.Type() != cty.String || !sourceValue.IsKnown() || sourceValue.IsNull() {
			continue
		}
		if !isHubSpotProviderSource(sourceValue.AsString()) {
			continue
		}
		constraintValue, exists := attributes["version"]
		if !exists || constraintValue.Type() != cty.String || !constraintValue.IsKnown() || constraintValue.IsNull() || strings.TrimSpace(constraintValue.AsString()) == "" {
			return nil, fmt.Errorf("%s: HubSpot provider %q has no version constraint", fileName, localName)
		}
		constraints = append(constraints, providerConstraint{file: fileName, text: constraintValue.AsString()})
	}
	return constraints, nil
}

func validateLock(root, engine, relativeLock, providerAddress, candidateVersion, releaseVersion string, candidate *version.Version) error {
	path := filepath.Join(root, relativeLock)
	fileName := filepath.ToSlash(relativeLock)
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("%s: committed %s lock is missing", fileName, engine)
	}
	parser := hclparse.NewParser()
	file, diagnostics := parser.ParseHCLFile(path)
	if diagnostics.HasErrors() {
		return fmt.Errorf("%s: malformed %s lock: %s", fileName, engine, diagnostics.Error())
	}
	body, ok := file.Body.(*hclsyntax.Body)
	if !ok {
		return fmt.Errorf("%s: unsupported %s lock syntax", fileName, engine)
	}
	var selected *hclsyntax.Block
	for _, block := range body.Blocks {
		if block.Type == "provider" && len(block.Labels) == 1 && block.Labels[0] == providerAddress {
			if selected != nil {
				return fmt.Errorf("%s: %s lock contains duplicate HubSpot provider selections", fileName, engine)
			}
			selected = block
		}
	}
	if selected == nil {
		return fmt.Errorf("%s: %s lock has no selection for %s", fileName, engine, providerAddress)
	}

	selectedVersion, err := requiredBlockString(selected, "version", fileName, engine)
	if err != nil {
		return err
	}
	if selectedVersion != releaseVersion {
		return fmt.Errorf("%s: %s lock selected HubSpot provider %q instead of requested version %s", fileName, engine, selectedVersion, candidateVersion)
	}
	constraintText, err := requiredBlockString(selected, "constraints", fileName, engine)
	if err != nil {
		return err
	}
	constraint, err := version.NewConstraint(constraintText)
	if err != nil {
		return fmt.Errorf("%s: malformed %s lock constraint %q: %w", fileName, engine, constraintText, err)
	}
	if !constraint.Check(candidate) {
		return fmt.Errorf("%s: %s lock constraint %q excludes requested version %s", fileName, engine, constraintText, candidateVersion)
	}

	hashesAttribute, exists := selected.Body.Attributes["hashes"]
	if !exists {
		return fmt.Errorf("%s: %s lock has no HubSpot provider package hashes", fileName, engine)
	}
	hashes, diagnostics := hashesAttribute.Expr.Value(nil)
	if diagnostics.HasErrors() || !hashes.IsKnown() || hashes.IsNull() || !hashes.CanIterateElements() {
		return fmt.Errorf("%s: malformed %s HubSpot provider package hashes", fileName, engine)
	}
	count := 0
	iterator := hashes.ElementIterator()
	for iterator.Next() {
		_, hashValue := iterator.Element()
		if hashValue.Type() != cty.String || !hashValue.IsKnown() || hashValue.IsNull() {
			return fmt.Errorf("%s: malformed %s HubSpot provider package hash", fileName, engine)
		}
		hash := hashValue.AsString()
		if !packageHashPattern.MatchString(hash) {
			return fmt.Errorf("%s: malformed %s HubSpot provider package hash %q", fileName, engine, hash)
		}
		count++
	}
	if count == 0 {
		return fmt.Errorf("%s: %s lock has no HubSpot provider package hashes", fileName, engine)
	}
	return nil
}

func requiredBlockString(block *hclsyntax.Block, attributeName, fileName, engine string) (string, error) {
	attribute, exists := block.Body.Attributes[attributeName]
	if !exists {
		return "", fmt.Errorf("%s: malformed %s lock: HubSpot provider %s is missing", fileName, engine, attributeName)
	}
	return constantString(attribute.Expr, fileName, engine+" lock "+attributeName)
}

func constantString(expression hclsyntax.Expression, fileName, description string) (string, error) {
	value, diagnostics := expression.Value(nil)
	if diagnostics.HasErrors() || value.Type() != cty.String || !value.IsKnown() || value.IsNull() {
		return "", fmt.Errorf("%s: %s must be a static string", fileName, description)
	}
	return value.AsString(), nil
}

func isHubSpotProviderSource(source string) bool {
	switch source {
	case "jackemcpherson/hubspot", "registry.terraform.io/jackemcpherson/hubspot", "registry.opentofu.org/jackemcpherson/hubspot":
		return true
	default:
		return false
	}
}

func relativePath(root, path string) string {
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	if relative == "." {
		return "."
	}
	return filepath.ToSlash(relative)
}

func firstLabel(labels []string) string {
	if len(labels) == 0 {
		return "<unnamed>"
	}
	return labels[0]
}
