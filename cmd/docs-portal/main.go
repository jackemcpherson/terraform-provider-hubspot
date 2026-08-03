// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jackemcpherson/terraform-provider-hubspot/internal/docsportal"
	providerimpl "github.com/jackemcpherson/terraform-provider-hubspot/internal/provider"
)

func main() {
	root, err := os.Getwd()
	if err != nil {
		fatal(err)
	}
	demo := os.Getenv("HUBSPOT_DEMO_REPO")
	if demo == "" {
		demo = filepath.Join(root, "..", "terraform-hubspot-demo")
	}
	version := os.Getenv("DOCS_PORTAL_VERSION")
	if version == "" {
		version = "0.3.0"
	}
	config := docsportal.Config{
		Provider: providerimpl.New(version)(), ProviderRepo: root, DemoRepo: demo,
		OutputDir: filepath.Join(root, "dist", "docs-portal"), Version: version,
		Released:               os.Getenv("DOCS_PORTAL_RELEASED") == "1",
		RequireClean:           os.Getenv("DOCS_PORTAL_REQUIRE_CLEAN") == "1",
		ExpectedProviderCommit: os.Getenv("DOCS_PORTAL_PROVIDER_COMMIT"),
		ExpectedDemoCommit:     os.Getenv("DOCS_PORTAL_DEMO_COMMIT"),
	}
	sourceOutput, err := os.MkdirTemp("", "hubspot-docs-portal-source-")
	if err != nil {
		fatal(err)
	}
	defer os.RemoveAll(sourceOutput)
	sourceConfig := config
	sourceConfig.OutputDir = sourceOutput
	sourceConfig.RequireClean = false
	sourceConfig.ExpectedProviderCommit = ""
	sourceConfig.ExpectedDemoCommit = ""
	sourceConfig.ProviderProvenance = docsportal.Provenance{Commit: "provider-source", Timestamp: "2000-01-01T00:00:00Z"}
	sourceConfig.DemoProvenance = docsportal.Provenance{Commit: "demo-source", Timestamp: "2000-01-01T00:00:00Z"}
	if err := docsportal.Generate(context.Background(), sourceConfig); err != nil {
		fatal(err)
	}
	digest, err := docsportal.TreeDigest(sourceOutput)
	if err != nil {
		fatal(err)
	}
	digestPath := filepath.Join(root, "docs", "portal-source.sha256")
	if os.Getenv("DOCS_PORTAL_UPDATE") == "1" {
		if err := os.WriteFile(digestPath, []byte(digest+"\n"), 0o644); err != nil {
			fatal(err)
		}
	} else {
		expected, readErr := os.ReadFile(digestPath)
		if readErr != nil {
			fatal(fmt.Errorf("portal source digest is missing; run make docs-portal-update: %w", readErr))
		}
		if strings.TrimSpace(string(expected)) != digest {
			fatal(fmt.Errorf("generated portal source is stale; run make docs-portal-update"))
		}
	}
	err = docsportal.Generate(context.Background(), config)
	if err != nil {
		fatal(err)
	}
	fmt.Println(filepath.Join(root, "dist", "docs-portal", "index.html"))
}

func fatal(err error) { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
