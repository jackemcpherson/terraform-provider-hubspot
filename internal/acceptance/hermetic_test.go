// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

//go:build acceptance

// This file drives the canonical hermetic lifecycle (create with state
// assertions, empty-plan re-apply, in-place update, import round-trip, and
// archival-aware destroy) against FakeHubSpot exclusively through
// Options.ProbeBaseURL and the provider's api_base_url attribute, using the
// real tofu/terraform CLIs. It needs no HubSpot credentials: run it with
//
//	go test -tags=acceptance ./internal/acceptance -run '^TestHermetic'
//
// Pipeline lifecycle coverage is intentionally not driven through Terraform
// here: hubspot_pipeline is implemented (internal/provider/pipeline_resource.go)
// but is not registered in Provider.Resources(), so no built provider binary
// recognizes it (see TestProviderRegistersOnlyFreeTierTypes in
// internal/provider). Registering it would be a provider schema shape
// change, which is out of scope for this version. FakeHubSpot still serves
// pipelines/stages faithfully (see fake_hubspot.go and fake_hubspot_test.go),
// consistent with the existing dormant //go:build deferred pipeline harness
// tests in this package, so the fake is ready once pipelines ship.
package acceptance_test

import (
	"fmt"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jackemcpherson/terraform-provider-hubspot/internal/acceptance"
)

const hermeticToken = "hermetic-sentinel"

func hermeticServer(t *testing.T, portalID int64) string {
	t.Helper()
	server := httptest.NewServer(acceptance.NewFakeHubSpot(hermeticToken, portalID))
	t.Cleanup(server.Close)
	return server.URL
}

func TestHermeticConsumerModuleLifecycle(t *testing.T) {
	runHermeticConsumerModuleLifecycle(t, acceptance.OpenTofu, "registry.opentofu.org/jackemcpherson/hubspot")
}

func TestHermeticConsumerModuleLifecycleTerraformParity(t *testing.T) {
	runHermeticConsumerModuleLifecycle(t, acceptance.Terraform, "registry.terraform.io/jackemcpherson/hubspot")
}

func TestHermeticV016StateUpgradeCompatibility(t *testing.T) {
	runHermeticV016StateUpgradeCompatibility(t, acceptance.OpenTofu, "registry.opentofu.org/jackemcpherson/hubspot")
}

func TestHermeticV016StateUpgradeCompatibilityTerraformParity(t *testing.T) {
	runHermeticV016StateUpgradeCompatibility(t, acceptance.Terraform, "registry.terraform.io/jackemcpherson/hubspot")
}

func runHermeticV016StateUpgradeCompatibility(t *testing.T, engine acceptance.Engine, providerSource string) {
	t.Helper()
	if _, err := exec.LookPath(string(engine)); err != nil {
		t.Skipf("pinned %s executable is not installed", engine)
	}
	server := hermeticServer(t, 444000444)
	fixture, err := os.ReadFile(filepath.Join("testdata", "v0.1.6-property-state.json"))
	if err != nil {
		t.Fatal(err)
	}
	state := strings.ReplaceAll(string(fixture), "__PROVIDER_SOURCE__", providerSource)
	config := hermeticV016CompatibilityConfig(server, providerSource)
	t.Setenv("HUBSPOT_ACCESS_TOKEN", hermeticToken)
	acceptance.Run(t, acceptance.Options{
		Engine: engine, Shard: acceptance.FreeProperties,
		Prefix: "tf_acc_hermetic_v016_", LedgerPath: t.TempDir() + "/cleanup.jsonl", ProbeBaseURL: server,
	}, func(session *acceptance.Session) {
		session.Apply(config)
		session.PushState(state)
		session.RequireEmptyPlan(config)
		session.Destroy(config)
	})
}

// TestCaptureV016PropertyState regenerates the committed state fixture when
// run explicitly with a released v0.1.6 provider binary.
func TestCaptureV016PropertyState(t *testing.T) {
	path := os.Getenv("HUBSPOT_V016_STATE_CAPTURE")
	if path == "" {
		t.Skip("state capture is an explicit maintenance operation")
	}
	server := hermeticServer(t, 444000444)
	t.Setenv("HUBSPOT_ACCESS_TOKEN", hermeticToken)
	config := hermeticV016CompatibilityConfig(server, "registry.terraform.io/jackemcpherson/hubspot")
	acceptance.Run(t, acceptance.Options{
		Engine: acceptance.Terraform, Shard: acceptance.FreeProperties,
		Prefix: "tf_acc_hermetic_v016_", LedgerPath: t.TempDir() + "/cleanup.jsonl", ProbeBaseURL: server,
	}, func(session *acceptance.Session) {
		session.Apply(config)
		state := strings.ReplaceAll(session.PullState(), "registry.terraform.io/jackemcpherson/hubspot", "__PROVIDER_SOURCE__")
		if err := os.WriteFile(path, []byte(state), 0o600); err != nil {
			t.Fatal(err)
		}
		session.Destroy(config)
	})
}

func runHermeticConsumerModuleLifecycle(t *testing.T, engine acceptance.Engine, providerSource string) {
	t.Helper()
	if _, err := exec.LookPath(string(engine)); err != nil {
		t.Skipf("pinned %s executable is not installed", engine)
	}
	demoRepo := os.Getenv("HUBSPOT_DEMO_REPO")
	if demoRepo == "" {
		demoRepo = filepath.Join("..", "..", "..", "terraform-hubspot-demo")
	}
	moduleSource, err := filepath.Abs(filepath.Join(demoRepo, "modules", "crm-schema"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(moduleSource, "main.tf")); err != nil {
		t.Fatalf("consumer crm-schema module is required: %v", err)
	}

	server := hermeticServer(t, 333000333)
	t.Setenv("HUBSPOT_ACCESS_TOKEN", hermeticToken)
	ledger := t.TempDir() + "/cleanup.jsonl"
	acceptance.Run(t, acceptance.Options{
		Engine: engine, Shard: acceptance.FreeProperties,
		Prefix: "tf_acc_hermetic_", LedgerPath: ledger, ProbeBaseURL: server,
	}, func(session *acceptance.Session) {
		initial := hermeticConsumerModuleConfig(server, providerSource, moduleSource, false)
		session.GetModules(initial)
		session.Apply(initial)
		session.RequireEmptyPlan(initial)

		updated := hermeticConsumerModuleConfig(server, providerSource, moduleSource, true)
		session.RequirePlanWarning(updated, acceptance.PropertyOptionValuesChanged)
		session.Apply(updated)
		session.RequireEmptyPlan(updated)
		session.Destroy(updated)
		session.RequirePropertyAbsent("contacts", "tf_acc_hermetic_module_text")
		session.RequirePropertyAbsent("contacts", "tf_acc_hermetic_module_select")
		session.RequirePropertyGroupAbsent("contacts", "tf_acc_hermetic_module_group")
	})
}

// --- property group lifecycle ---

func TestHermeticPropertyGroupLifecycle(t *testing.T) {
	runHermeticPropertyGroupLifecycle(t, acceptance.OpenTofu, "registry.opentofu.org/jackemcpherson/hubspot")
}

func TestHermeticPropertyGroupLifecycleTerraformParity(t *testing.T) {
	runHermeticPropertyGroupLifecycle(t, acceptance.Terraform, "registry.terraform.io/jackemcpherson/hubspot")
}

func runHermeticPropertyGroupLifecycle(t *testing.T, engine acceptance.Engine, providerSource string) {
	t.Helper()
	if _, err := exec.LookPath(string(engine)); err != nil {
		t.Skipf("pinned %s executable is not installed", engine)
	}

	server := hermeticServer(t, 111000111)
	t.Setenv("HUBSPOT_ACCESS_TOKEN", hermeticToken)
	ledger := t.TempDir() + "/cleanup.jsonl"

	acceptance.Run(t, acceptance.Options{
		Engine:       engine,
		Shard:        acceptance.FreeProperties,
		Prefix:       "tf_acc_hermetic_",
		LedgerPath:   ledger,
		ProbeBaseURL: server,
	}, func(session *acceptance.Session) {
		name := "tf_acc_hermetic_group"
		initial := hermeticPropertyGroupConfig(server, providerSource, name, "Initial label", 10)
		session.Apply(initial)
		session.RequireStateString("hubspot_property_group.test", "id", "contacts/"+name)
		session.RequireStateString("hubspot_property_group.test", "label", "Initial label")
		session.RequireEmptyPlan(initial)

		updated := hermeticPropertyGroupConfig(server, providerSource, name, "Updated label", 20)
		session.Apply(updated)
		session.RequireStateString("hubspot_property_group.test", "label", "Updated label")
		session.RequireEmptyPlan(updated)

		// Drift via out-of-band fake mutation.
		driftOrder := int64(30)
		session.MutatePropertyGroup("contacts", name, "Out-of-band label", &driftOrder)
		session.RequirePlanDiffAttributes(updated, "hubspot_property_group.test", "display_order", "label")
		session.Apply(updated)
		session.RequireEmptyPlan(updated)

		// Import round-trip.
		session.RemoveState("hubspot_property_group.test")
		session.Import("hubspot_property_group.test", "contacts/"+name)
		session.RequireEmptyPlan(updated)

		// Removed-resource recovery: the group vanishes out of band and the
		// next refresh clears state; a subsequent apply recreates it.
		session.ArchivePropertyGroup("contacts", name)
		session.Refresh(updated)
		session.RequireStateAbsent("hubspot_property_group.test")
		session.Apply(updated)
		session.RequireEmptyPlan(updated)

		// Destroy with archival-aware verification.
		session.Destroy(updated)
		session.RequirePropertyGroupAbsent("contacts", name)
		// Archived group names are not reserved: recreating it must succeed.
		session.RequirePropertyGroupReusable("contacts", name)
	})
}

func TestHermeticPropertyGroupBlockedDestroy(t *testing.T) {
	runHermeticPropertyGroupBlockedDestroy(t, acceptance.OpenTofu, "registry.opentofu.org/jackemcpherson/hubspot")
}

func TestHermeticPropertyGroupBlockedDestroyTerraformParity(t *testing.T) {
	runHermeticPropertyGroupBlockedDestroy(t, acceptance.Terraform, "registry.terraform.io/jackemcpherson/hubspot")
}

func runHermeticPropertyGroupBlockedDestroy(t *testing.T, engine acceptance.Engine, providerSource string) {
	t.Helper()
	if _, err := exec.LookPath(string(engine)); err != nil {
		t.Skipf("pinned %s executable is not installed", engine)
	}

	server := hermeticServer(t, 111000222)
	t.Setenv("HUBSPOT_ACCESS_TOKEN", hermeticToken)
	ledger := t.TempDir() + "/cleanup.jsonl"

	acceptance.Run(t, acceptance.Options{
		Engine:       engine,
		Shard:        acceptance.FreeProperties,
		Prefix:       "tf_acc_hermetic_",
		LedgerPath:   ledger,
		ProbeBaseURL: server,
	}, func(session *acceptance.Session) {
		groupName := "tf_acc_hermetic_blocked_group"
		propertyName := "tf_acc_hermetic_blocked_property"
		initial := hermeticBlockedGroupConfig(server, providerSource, groupName, propertyName, true)
		session.Apply(initial)
		// Group-deletion-with-active-properties failure: removing the group
		// from config while its property remains active must fail with the
		// harness's canonical failure identity, and retain state.
		session.RequireApplyFailureWithStatus(hermeticBlockedGroupConfig(server, providerSource, groupName, propertyName, false), acceptance.PropertyGroupHasActiveProperties)
		session.RequireStateString("hubspot_property_group.blocked", "label", "Hermetic blocked group")
		session.Destroy(initial)
	})
}

// --- property definition lifecycle ---

func TestHermeticPropertyDefinitionLifecycle(t *testing.T) {
	runHermeticPropertyDefinitionLifecycle(t, acceptance.OpenTofu, "registry.opentofu.org/jackemcpherson/hubspot")
}

func TestHermeticPropertyDefinitionLifecycleTerraformParity(t *testing.T) {
	runHermeticPropertyDefinitionLifecycle(t, acceptance.Terraform, "registry.terraform.io/jackemcpherson/hubspot")
}

func runHermeticPropertyDefinitionLifecycle(t *testing.T, engine acceptance.Engine, providerSource string) {
	t.Helper()
	if _, err := exec.LookPath(string(engine)); err != nil {
		t.Skipf("pinned %s executable is not installed", engine)
	}

	server := hermeticServer(t, 222000222)
	t.Setenv("HUBSPOT_ACCESS_TOKEN", hermeticToken)
	ledger := t.TempDir() + "/cleanup.jsonl"

	acceptance.Run(t, acceptance.Options{
		Engine:       engine,
		Shard:        acceptance.FreeProperties,
		Prefix:       "tf_acc_hermetic_",
		LedgerPath:   ledger,
		ProbeBaseURL: server,
	}, func(session *acceptance.Session) {
		for _, objectType := range []string{"contacts", "companies", "deals", "tickets"} {
			groupName := "tf_acc_hermetic_" + objectType + "_group"
			textName := "tf_acc_hermetic_" + objectType + "_text"
			selectName := "tf_acc_hermetic_" + objectType + "_select"
			initial := hermeticObjectSchemaConfig(server, providerSource, objectType, groupName, textName, selectName, false)
			session.Apply(initial)
			session.RequireStateString("hubspot_property.text", "id", objectType+"/"+textName)
			session.RequireStateString("hubspot_property.text", "type", "string")
			session.RequireStateString("hubspot_property.select", "field_type", "select")
			session.RequireEmptyPlan(initial)

			updated := hermeticObjectSchemaConfig(server, providerSource, objectType, groupName, textName, selectName, true)
			session.RequirePlanWarning(updated, acceptance.PropertyOptionValuesChanged)
			session.Apply(updated)
			session.RequireEmptyPlan(updated)

			// Drift via out-of-band fake mutation.
			session.MutatePropertyLabel(objectType, textName, "Out-of-band label")
			session.RequirePlanDiffAttributes(updated, "hubspot_property.text", "label")
			session.Apply(updated)
			session.RequireEmptyPlan(updated)

			// Direct group and property import round-trips use canonical IDs.
			session.RemoveState("hubspot_property_group.test")
			session.Import("hubspot_property_group.test", objectType+"/"+groupName)
			session.RemoveState("hubspot_property.text")
			session.Import("hubspot_property.text", objectType+"/"+textName)
			session.RequireEmptyPlan(updated)

			// Refresh removes an out-of-band archived property, and apply
			// immediately reuses its immutable name while retaining tombstone visibility.
			session.ArchiveProperty(objectType, textName)
			session.Refresh(updated)
			session.RequireStateAbsent("hubspot_property.text")
			session.Apply(updated)
			session.RequirePropertyArchived(objectType, textName)
			session.RequireEmptyPlan(updated)

			session.Destroy(updated)
			session.RequirePropertyAbsent(objectType, textName)
			session.RequirePropertyAbsent(objectType, selectName)
			session.RequirePropertyArchived(objectType, textName)
			session.RequirePropertyArchived(objectType, selectName)
			session.RequirePropertyGroupAbsent(objectType, groupName)
			session.RequirePropertyGroupReusable(objectType, groupName)
		}
	})
}

// --- shared config builders ---

func hermeticConsumerModuleConfig(apiBaseURL, providerSource, moduleSource string, updated bool) string {
	groupLabel := "Hermetic module group"
	textDescription := ""
	options := `
      alpha = { label = "Alpha", display_order = 10 }
      beta  = { label = "Beta", display_order = 20 }
`
	if updated {
		groupLabel = "Updated hermetic module group"
		textDescription = "Updated through the consumer module"
		options = `
      alpha = { label = "Alpha updated", display_order = 10, hidden = true }
      gamma = { label = "Gamma", description = "Added option", display_order = 30 }
`
	}
	return fmt.Sprintf(`
terraform {
  required_providers {
    hubspot = {
      source = %q
    }
  }
}

provider "hubspot" {
  access_token = %q
  api_base_url = %q
}

module "schema" {
  source      = %q
  object_type = "contacts"
  groups = {
    tf_acc_hermetic_module_group = {
      label         = %q
      display_order = 10
    }
  }
  properties = {
    tf_acc_hermetic_module_text = {
      label         = "Hermetic module text"
      group         = "tf_acc_hermetic_module_group"
      description   = %q
      display_order = 20
    }
    tf_acc_hermetic_module_select = {
      label         = "Hermetic module select"
      group         = "tf_acc_hermetic_module_group"
      kind          = "select"
      display_order = 30
      options = {%s
      }
    }
  }
}
`, providerSource, hermeticToken, apiBaseURL, moduleSource, groupLabel, textDescription, options)
}

func hermeticV016CompatibilityConfig(apiBaseURL, providerSource string) string {
	return fmt.Sprintf(`
terraform {
  required_providers {
    hubspot = {
      source = %q
    }
  }
}

provider "hubspot" {
  access_token = %q
  api_base_url = %q
}

resource "hubspot_property_group" "v016" {
  object_type   = "contacts"
  name          = "tf_acc_hermetic_v016_group"
  label         = "v0.1.6 group"
  display_order = 10
}

resource "hubspot_property" "v016" {
  object_type = "contacts"
  name        = "tf_acc_hermetic_v016_property"
  label       = "v0.1.6 property"
  group_name  = hubspot_property_group.v016.name
  type        = "string"
  field_type  = "text"
}
`, providerSource, hermeticToken, apiBaseURL)
}

func hermeticPropertyGroupConfig(apiBaseURL, providerSource, name, label string, displayOrder int64) string {
	return fmt.Sprintf(`
terraform {
  required_providers {
    hubspot = {
      source = %q
    }
  }
}

provider "hubspot" {
  access_token = %q
  api_base_url = %q
}

resource "hubspot_property_group" "test" {
  object_type   = "contacts"
  name          = %q
  label         = %q
  display_order = %d
}
`, providerSource, hermeticToken, apiBaseURL, name, label, displayOrder)
}

func hermeticBlockedGroupConfig(apiBaseURL, providerSource, groupName, propertyName string, includeGroup bool) string {
	group := ""
	dependency := ""
	if includeGroup {
		group = fmt.Sprintf(`
resource "hubspot_property_group" "blocked" {
  object_type   = "contacts"
  name          = %q
  label         = "Hermetic blocked group"
  display_order = -1
}
`, groupName)
		dependency = "depends_on = [hubspot_property_group.blocked]"
	}
	return fmt.Sprintf(`
terraform {
  required_providers {
    hubspot = {
      source = %q
    }
  }
}

provider "hubspot" {
  access_token = %q
  api_base_url = %q
}
%s
resource "hubspot_property" "blocker" {
  object_type = "contacts"
  name        = %q
  label       = "Hermetic group deletion blocker"
  group_name  = %q
  type        = "string"
  field_type  = "text"
  %s
}
`, providerSource, hermeticToken, apiBaseURL, group, propertyName, groupName, dependency)
}

func hermeticObjectSchemaConfig(apiBaseURL, providerSource, objectType, groupName, textName, selectName string, updated bool) string {
	groupLabel := "Hermetic property schema"
	textLabel := "Hermetic text"
	textDescription := ""
	textOrder := int64(10)
	textHidden := false
	alphaLabel := "Alpha"
	alphaOrder := int64(20)
	options := `
    alpha = { label = %q, display_order = %d }
    beta  = { label = "Beta", display_order = 30 }
`
	if updated {
		groupLabel = "Updated hermetic property schema"
		textLabel = "Updated hermetic text"
		textDescription = "Updated through the provider"
		textOrder = 15
		textHidden = true
		alphaLabel = "Alpha updated"
		alphaOrder = 25
		options = `
    alpha = { label = %q, display_order = %d, hidden = true }
    gamma = { label = "Gamma", description = "Added option", display_order = 35 }
`
	}
	return fmt.Sprintf(`
terraform {
  required_providers {
    hubspot = {
      source = %q
    }
  }
}

provider "hubspot" {
  access_token = %q
  api_base_url = %q
}

resource "hubspot_property_group" "test" {
  object_type = %q
  name        = %q
  label       = %q
  display_order = 5
}

resource "hubspot_property" "text" {
  object_type = %q
  name        = %q
  label       = %q
  group_name  = hubspot_property_group.test.name
  type        = "string"
  field_type  = "text"
  description = %q
  display_order = %d
  hidden      = %t
}

resource "hubspot_property" "select" {
  object_type = %q
  name        = %q
  label       = "Hermetic select"
  group_name  = hubspot_property_group.test.name
  type        = "enumeration"
  field_type  = "select"

  options = {%s
  }
}
`, providerSource, hermeticToken, apiBaseURL, objectType, groupName, groupLabel, objectType, textName, textLabel, textDescription, textOrder, textHidden, objectType, selectName, fmt.Sprintf(options, alphaLabel, alphaOrder))
}
