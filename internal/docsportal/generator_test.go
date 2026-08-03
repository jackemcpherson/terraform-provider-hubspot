// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

package docsportal_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jackemcpherson/terraform-provider-hubspot/internal/docsportal"
	providerimpl "github.com/jackemcpherson/terraform-provider-hubspot/internal/provider"
)

func TestGenerateBuildsRegisteredProviderAndConsumerModulePages(t *testing.T) {
	providerRepo := filepath.Clean(filepath.Join("..", ".."))
	demoRepo := createDemoFixture(t)
	output := filepath.Join(t.TempDir(), "portal")

	err := docsportal.Generate(context.Background(), docsportal.Config{
		Provider:               providerimpl.New("0.3.0")(),
		ProviderRepo:           providerRepo,
		DemoRepo:               demoRepo,
		OutputDir:              output,
		Version:                "0.3.0",
		RequireClean:           true,
		ExpectedProviderCommit: "provider-commit",
		ExpectedDemoCommit:     "demo-commit",
		ProviderProvenance: docsportal.Provenance{
			Commit: "provider-commit", Timestamp: "2026-08-02T00:00:00Z",
		},
		DemoProvenance: docsportal.Provenance{Commit: "demo-commit", Timestamp: "2026-08-01T00:00:00Z"},
	})
	if err != nil {
		t.Fatal(err)
	}

	for _, relative := range []string{
		"index.html", "crm-property-schema.html", "form-definition.html", "resources/index.html",
		"resources/hubspot_property.html", "resources/hubspot_property_group.html",
		"resources/hubspot_form_definition.html",
		"data-sources/index.html", "data-sources/hubspot_property_definition.html",
		"data-sources/hubspot_property_definitions.html", "modules/index.html",
		"modules/crm-schema.html", "modules/form-definition.html", "provenance.json",
	} {
		if _, err := os.Stat(filepath.Join(output, relative)); err != nil {
			t.Errorf("missing generated %s: %v", relative, err)
		}
	}

	modulePage := readFile(t, filepath.Join(output, "modules", "crm-schema.html"))
	for _, expected := range []string{"object_type", "groups", "properties", "hubspot_property_group", "hubspot_property", "Typed inputs", "Outputs", "Complete usage", `module &#34;crm_schema&#34;`} {
		if !strings.Contains(modulePage, expected) {
			t.Errorf("module page missing %q", expected)
		}
	}
	formModulePage := readFile(t, filepath.Join(output, "modules", "form-definition.html"))
	for _, expected := range []string{"forms", "hubspot_form_definition", "ids", "Stable map keys", "built-in contacts email", "Complete usage", `module &#34;contact_forms&#34;`, "moved", "archives the old form", "0.3.0"} {
		if !strings.Contains(formModulePage, expected) {
			t.Errorf("form module page missing %q", expected)
		}
	}
	formResourcePage := readFile(t, filepath.Join(output, "resources", "hubspot_form_definition.html"))
	for _, expected := range []string{"field_groups", "configuration", "display_options", "exact lowercase generated UUID", "Unsupported structure fails closed", "Form display name; it is presentation, not identity"} {
		if !strings.Contains(formResourcePage, expected) {
			t.Errorf("form resource page missing %q", expected)
		}
	}
	formOverview := readFile(t, filepath.Join(output, "form-definition.html"))
	for _, expected := range []string{"forms", "duplicate active names", "no-consent", "bounded writes", "Archived form definition", "beta"} {
		if !strings.Contains(formOverview, expected) {
			t.Errorf("form overview missing %q", expected)
		}
	}
	propertyPage := readFile(t, filepath.Join(output, "resources", "hubspot_property.html"))
	for _, expected := range []string{"object_type", "field_type", "options", "Import", "Lifecycle"} {
		if !strings.Contains(propertyPage, expected) {
			t.Errorf("property page missing %q", expected)
		}
	}
	provenance := readFile(t, filepath.Join(output, "provenance.json"))
	for _, expected := range []string{"provider-commit", "demo-commit", `"version": "0.3.0"`, "2026-08-02T00:00:00Z", `"state": "unreleased"`, `"dirty": false`, "hubspot_form_definition", "form-definition"} {
		if !strings.Contains(provenance, expected) {
			t.Errorf("provenance missing %q", expected)
		}
	}
	if err := docsportal.ValidateLinks(output); err != nil {
		t.Fatalf("generated portal links: %v", err)
	}
	secondOutput := filepath.Join(t.TempDir(), "portal")
	err = docsportal.Generate(context.Background(), docsportal.Config{
		Provider: providerimpl.New("0.3.0")(), ProviderRepo: providerRepo, DemoRepo: demoRepo,
		OutputDir: secondOutput, Version: "0.3.0",
		ProviderProvenance: docsportal.Provenance{Commit: "provider-commit", Timestamp: "2026-08-02T00:00:00Z"},
		DemoProvenance:     docsportal.Provenance{Commit: "demo-commit", Timestamp: "2026-08-01T00:00:00Z"},
	})
	if err != nil {
		t.Fatal(err)
	}
	firstFiles := directoryContents(t, output)
	secondFiles := directoryContents(t, secondOutput)
	if len(firstFiles) != len(secondFiles) {
		t.Fatalf("regeneration file count = %d, want %d", len(secondFiles), len(firstFiles))
	}
	for name, contents := range firstFiles {
		if secondFiles[name] != contents {
			t.Errorf("regeneration changed %s", name)
		}
	}
}

func TestGenerateRejectsMissingCRMModule(t *testing.T) {
	err := docsportal.Generate(context.Background(), docsportal.Config{
		Provider:     providerimpl.New("0.2.0")(),
		ProviderRepo: filepath.Clean(filepath.Join("..", "..")),
		DemoRepo:     t.TempDir(),
		OutputDir:    filepath.Join(t.TempDir(), "portal"),
		Version:      "0.2.0",
		ProviderProvenance: docsportal.Provenance{
			Commit: "provider-commit", Timestamp: "2026-08-02T00:00:00Z",
		},
		DemoProvenance: docsportal.Provenance{Commit: "demo-commit", Timestamp: "2026-08-01T00:00:00Z"},
	})
	if err == nil || !strings.Contains(err.Error(), "crm-schema") {
		t.Fatalf("error = %v, want missing crm-schema module", err)
	}
}

func TestGenerateRejectsDirtyCandidateInputs(t *testing.T) {
	err := docsportal.Generate(context.Background(), docsportal.Config{
		Provider: providerimpl.New("0.2.0")(), ProviderRepo: filepath.Clean(filepath.Join("..", "..")),
		DemoRepo: createDemoFixture(t), OutputDir: filepath.Join(t.TempDir(), "portal"), Version: "0.2.0",
		RequireClean:       true,
		ProviderProvenance: docsportal.Provenance{Commit: "provider-commit", Timestamp: "2026-08-02T00:00:00Z", Dirty: true},
		DemoProvenance:     docsportal.Provenance{Commit: "demo-commit", Timestamp: "2026-08-01T00:00:00Z"},
	})
	if err == nil || !strings.Contains(err.Error(), "clean") {
		t.Fatalf("error = %v, want clean-checkout requirement", err)
	}
}

func TestValidateRenderedHTMLRejectsMissingLandmarks(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "broken.html"), []byte("<html><head><title>Broken</title></head><body><main><h1>Broken</h1></main></body></html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := docsportal.ValidateRenderedHTML(root); err == nil || !strings.Contains(err.Error(), "header") {
		t.Fatalf("error = %v, want missing header", err)
	}
}

func TestTreeDigestTracksGeneratedBytesNotFilesystemOrder(t *testing.T) {
	first := t.TempDir()
	second := t.TempDir()
	for _, root := range []string{first, second} {
		if err := os.Mkdir(filepath.Join(root, "nested"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "nested", "b.html"), []byte("b"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "a.html"), []byte("a"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	firstDigest, err := docsportal.TreeDigest(first)
	if err != nil {
		t.Fatal(err)
	}
	secondDigest, err := docsportal.TreeDigest(second)
	if err != nil {
		t.Fatal(err)
	}
	if firstDigest != secondDigest {
		t.Fatal("identical trees produced different source digests")
	}
	if err := os.WriteFile(filepath.Join(second, "a.html"), []byte("changed"), 0o644); err != nil {
		t.Fatal(err)
	}
	changedDigest, err := docsportal.TreeDigest(second)
	if err != nil {
		t.Fatal(err)
	}
	if changedDigest == firstDigest {
		t.Fatal("changed generated bytes retained stale source digest")
	}
}

func createDemoFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	crmModule := filepath.Join(root, "modules", "crm-schema")
	formModule := filepath.Join(root, "modules", "form-definition")
	if err := os.MkdirAll(crmModule, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(formModule, 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"main.tf": `resource "hubspot_property_group" "this" { for_each = var.groups }
resource "hubspot_property" "this" { for_each = var.properties }
`,
		"variables.tf": `variable "object_type" { type = string }
variable "groups" { type = map(object({ label = string })) }
variable "properties" { type = map(object({ label = string })) }
`,
		"outputs.tf": `output "groups" { value = hubspot_property_group.this }
output "properties" { value = hubspot_property.this }
`,
		"versions.tf": `terraform { required_version = ">= 1.8, < 2.0" }
`,
	}
	for name, contents := range files {
		if err := os.WriteFile(filepath.Join(crmModule, name), []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	formFiles := map[string]string{
		"main.tf": `resource "hubspot_form_definition" "this" { for_each = var.forms }
`,
		"variables.tf": `variable "forms" { type = map(object({ name = string })) }
`,
		"outputs.tf": `output "ids" { value = { for key, form in hubspot_form_definition.this : key => form.id } }
`,
		"versions.tf": `terraform { required_version = ">= 1.8, < 2.0" }
`,
	}
	for name, contents := range formFiles {
		if err := os.WriteFile(filepath.Join(formModule, name), []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(formModule, "README.md"), []byte("Use a moved block when changing a key; otherwise replacement archives the old form. Requires provider 0.3.0.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "main.tf"), []byte(`module "crm_schema" { source = "./modules/crm-schema" }
`), 0o644); err != nil {
		t.Fatal(err)
	}
	example := filepath.Join(root, "examples", "form-definition")
	if err := os.MkdirAll(example, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(example, "main.tf"), []byte(`module "contact_forms" {
  source = "../../modules/form-definition"
  forms  = { contact = { name = "Contact" } }
}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(contents)
}

func directoryContents(t *testing.T, root string) map[string]string {
	t.Helper()
	files := make(map[string]string)
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		contents, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files[relative] = string(contents)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}
