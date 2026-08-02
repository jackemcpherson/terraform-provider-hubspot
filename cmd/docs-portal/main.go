// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

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
		version = "0.2.0"
	}
	config := docsportal.Config{
		Provider: providerimpl.New(version)(), ProviderRepo: root, DemoRepo: demo,
		OutputDir: filepath.Join(root, "dist", "docs-portal"), Version: version,
		Released:               os.Getenv("DOCS_PORTAL_RELEASED") == "1",
		RequireClean:           os.Getenv("DOCS_PORTAL_REQUIRE_CLEAN") == "1",
		ExpectedProviderCommit: os.Getenv("DOCS_PORTAL_PROVIDER_COMMIT"),
		ExpectedDemoCommit:     os.Getenv("DOCS_PORTAL_DEMO_COMMIT"),
	}
	err = docsportal.Generate(context.Background(), config)
	if err != nil {
		fatal(err)
	}
	checkOutput := config.OutputDir + ".regenerated"
	defer os.RemoveAll(checkOutput)
	config.OutputDir = checkOutput
	if err := docsportal.Generate(context.Background(), config); err != nil {
		fatal(err)
	}
	if err := docsportal.CompareTrees(filepath.Join(root, "dist", "docs-portal"), checkOutput); err != nil {
		fatal(err)
	}
	fmt.Println(filepath.Join(root, "dist", "docs-portal", "index.html"))
}

func fatal(err error) { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
