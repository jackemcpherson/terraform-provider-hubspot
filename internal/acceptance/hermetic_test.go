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
	"os/exec"
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
		groupName := "tf_acc_hermetic_property_group"
		name := "tf_acc_hermetic_property"
		initial := hermeticPropertyConfig(server, providerSource, groupName, name, "Initial label")
		session.Apply(initial)
		session.RequireStateString("hubspot_property.test", "id", "contacts/"+name)
		session.RequireStateString("hubspot_property.test", "label", "Initial label")
		session.RequireEmptyPlan(initial)

		updated := hermeticPropertyConfig(server, providerSource, groupName, name, "Updated label")
		session.Apply(updated)
		session.RequireEmptyPlan(updated)

		// Drift via out-of-band fake mutation.
		session.MutatePropertyLabel("contacts", name, "Out-of-band label")
		session.RequirePlanDiffAttributes(updated, "hubspot_property.test", "label")
		session.Apply(updated)
		session.RequireEmptyPlan(updated)

		// Import round-trip.
		session.RemoveState("hubspot_property.test")
		session.Import("hubspot_property.test", "contacts/"+name)
		session.RequireEmptyPlan(updated)

		// Destroy with archival-aware verification: remove just the
		// property from config (destroying it) while keeping its group.
		groupOnly := hermeticPropertyGroupOnlyConfig(server, providerSource, groupName)
		session.Apply(groupOnly)
		session.RequirePropertyAbsent("contacts", name)
		session.RequirePropertyArchived("contacts", name)

		// Archived-name reservation on re-create: unlike property groups, a
		// property definition's name stays reserved after archival, so
		// reapplying the original configuration must fail rather than
		// silently recreate it.
		session.RequireApplyFailure(updated)
	})
}

// --- shared config builders ---

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

func hermeticPropertyGroupOnlyConfig(apiBaseURL, providerSource, groupName string) string {
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
  object_type = "contacts"
  name        = %q
  label       = "Hermetic property definitions"
}
`, providerSource, hermeticToken, apiBaseURL, groupName)
}

func hermeticPropertyConfig(apiBaseURL, providerSource, groupName, name, label string) string {
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
  object_type = "contacts"
  name        = %q
  label       = "Hermetic property definitions"
}

resource "hubspot_property" "test" {
  object_type = "contacts"
  name        = %q
  label       = %q
  group_name  = hubspot_property_group.test.name
  type        = "string"
  field_type  = "text"
}
`, providerSource, hermeticToken, apiBaseURL, groupName, name, label)
}
