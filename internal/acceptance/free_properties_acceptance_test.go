// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

//go:build acceptance

package acceptance_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jackemcpherson/terraform-provider-hubspot/internal/acceptance"
)

func TestFreePropertiesAcceptanceConfigurationSyntax(t *testing.T) {
	order := int64(110)
	configs := []string{
		livePropertyGroupConfig("tf_acc_syntax_group", "Syntax group", nil),
		livePropertyGroupConfig("tf_acc_syntax_group", "Syntax group", &order),
		livePropertyConfig("tf_acc_syntax_", false),
		livePropertyConfig("tf_acc_syntax_", true),
		liveBlockedGroupConfig("tf_acc_syntax_", true),
		liveBlockedGroupConfig("tf_acc_syntax_", false),
		liveProviderOnlyConfig(),
		liveArchivedDiscoveryConfig("tf_acc_syntax_"),
		liveActiveMissingDiscoveryConfig("tf_acc_syntax_"),
		liveBuiltInDiscoveryConfig(false),
		liveBuiltInDiscoveryConfig(true),
		liveInvalidDateDisplayHintConfig(),
	}
	configs = append(configs, warningTransitionConfigs("tf_acc_syntax_")...)
	for index, config := range configs {
		directory := t.TempDir()
		path := filepath.Join(directory, "main.tf")
		if err := os.WriteFile(path, []byte(config), 0o600); err != nil {
			t.Fatalf("write acceptance syntax fixture %d: %v", index, err)
		}
		command := exec.Command("tofu", "fmt", path)
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("acceptance syntax fixture %d is invalid: %v: %s", index, err, strings.TrimSpace(string(output)))
		}
	}
}

func TestLivePropertyConfigPreservesGroupDependencies(t *testing.T) {
	tests := []struct {
		updated bool
		group   string
	}{
		{updated: false, group: "hubspot_property_group.test.name"},
		{updated: true, group: "hubspot_property_group.secondary.name"},
	}
	for _, test := range tests {
		config := livePropertyConfig("tf_acc_dependency_", test.updated)
		scalarBlock := strings.Split(strings.Split(config, `resource "hubspot_property" "scalar" {`)[1], `resource "hubspot_property" "enumeration"`)[0]
		if !strings.Contains(scalarBlock, "group_name  = "+test.group) {
			t.Fatalf("updated=%t scalar property does not retain its property-group dependency", test.updated)
		}
	}
}

func warningTransitionConfigs(prefix string) []string {
	safe := livePropertyConfig(prefix, true)
	typeTransition := strings.Replace(safe, `field_type  = "text"`, `field_type  = "textarea"`, 1)
	optionRemoval := strings.Replace(typeTransition, `    beta  = { label = "Beta", display_order = 270 }
`, "", 1)
	optionReplacement := strings.Replace(optionRemoval, `    alpha = { label = "Alpha updated", display_order = 250 }`, `    gamma = { label = "Gamma", display_order = 250 }`, 1)
	storageTransition := strings.Replace(optionReplacement, `type        = "enumeration"`, `type        = "string"`, 1)
	storageTransition = strings.Replace(storageTransition, `field_type  = "select"`, `field_type  = "text"`, 1)
	storageTransition = strings.Replace(storageTransition, `
  options = {
    gamma = { label = "Gamma", display_order = 250 }
  }
`, "\n", 1)
	return []string{safe, typeTransition, optionRemoval, optionReplacement, storageTransition}
}

func TestAcc_free_properties_PropertyGroupLifecycle(t *testing.T) {
	runFreePropertyGroupLifecycle(t, acceptance.OpenTofu)
}

func TestAcc_free_properties_PropertyGroupLifecycleTerraformParity(t *testing.T) {
	runFreePropertyGroupLifecycle(t, acceptance.Terraform)
}

func runFreePropertyGroupLifecycle(t *testing.T, engine acceptance.Engine) {
	t.Helper()
	requireAcceptanceEnabled(t)
	prefix := requiredEnvironment(t, "HUBSPOT_ACCEPTANCE_PREFIX")
	ledger := requiredEnvironment(t, "HUBSPOT_ACCEPTANCE_CLEANUP_LEDGER")
	name := prefix + "group_lifecycle"

	acceptance.Run(t, acceptance.Options{
		Engine:     engine,
		Shard:      acceptance.FreeProperties,
		Prefix:     prefix,
		LedgerPath: ledger,
	}, func(session *acceptance.Session) {
		initial := configForEngine(livePropertyGroupConfig(name, "Acceptance property group", nil), engine)
		session.Apply(initial)
		session.RequireStateString("hubspot_property_group.test", "id", "contacts/"+name)
		session.RequireEmptyPlan(initial)

		updatedOrder := int64(110)
		updated := configForEngine(livePropertyGroupConfig(name, "Updated acceptance property group", &updatedOrder), engine)
		session.Apply(updated)
		session.RequireEmptyPlan(updated)
		driftOrder := int64(120)
		session.MutatePropertyGroup("contacts", name, "Out-of-band acceptance label", &driftOrder)
		session.RequirePlanDiffAttributes(updated, "hubspot_property_group.test", "display_order", "label")
		session.Apply(updated)
		session.RequireEmptyPlan(updated)
		session.RemoveState("hubspot_property_group.test")
		session.Import("hubspot_property_group.test", "contacts/"+name)
		session.RequireEmptyPlan(updated)
		session.Destroy(updated)
		session.RequirePropertyGroupAbsent("contacts", name)
		session.RequirePropertyGroupReusable("contacts", name)
		session.Apply(updated)
		session.RequireEmptyPlan(updated)
		session.ArchivePropertyGroup("contacts", name)
		session.Refresh(updated)
		session.RequireStateAbsent("hubspot_property_group.test")
		session.Apply(updated)
		session.RequireEmptyPlan(updated)
	})
	requireFreeOwnedConfigurationAbsent(t, prefix)
}

func TestAcc_free_properties_PropertyGroupBlockedDestroy(t *testing.T) {
	requireAcceptanceEnabled(t)
	prefix := requiredEnvironment(t, "HUBSPOT_ACCEPTANCE_PREFIX")
	ledger := requiredEnvironment(t, "HUBSPOT_ACCEPTANCE_CLEANUP_LEDGER")

	acceptance.Run(t, acceptance.Options{
		Engine:     acceptance.OpenTofu,
		Shard:      acceptance.FreeProperties,
		Prefix:     prefix,
		LedgerPath: ledger,
	}, func(session *acceptance.Session) {
		initial := liveBlockedGroupConfig(prefix, true)
		session.Apply(initial)
		session.RequireApplyFailureWithStatus(liveBlockedGroupConfig(prefix, false), acceptance.PropertyGroupHasActiveProperties)
		session.RequireStateString("hubspot_property_group.blocked", "label", "Acceptance blocked property group")
		session.Apply(liveProviderOnlyConfig())
	})
	requireFreeOwnedConfigurationAbsent(t, prefix)
}

func TestAcc_free_properties_PropertyLifecycleAndDiscovery(t *testing.T) {
	runPropertyLifecycleAndDiscovery(t, acceptance.OpenTofu)
}

func TestAcc_free_properties_BuiltInPropertyImportRejected(t *testing.T) {
	requireAcceptanceEnabled(t)
	prefix := requiredEnvironment(t, "HUBSPOT_ACCEPTANCE_PREFIX")
	ledger := requiredEnvironment(t, "HUBSPOT_ACCEPTANCE_CLEANUP_LEDGER")
	acceptance.Run(t, acceptance.Options{
		Engine:     acceptance.OpenTofu,
		Shard:      acceptance.FreeProperties,
		Prefix:     prefix,
		LedgerPath: ledger,
	}, func(session *acceptance.Session) {
		session.Apply(liveBuiltInDiscoveryConfig(false))
		session.RequireImportFailure(liveBuiltInDiscoveryConfig(true), "hubspot_property.readonly", "contacts/email", "Property is discovery-only")
	})
}

func TestAcc_free_properties_TerraformParity(t *testing.T) {
	runPropertyLifecycleAndDiscovery(t, acceptance.Terraform)
}

func runPropertyLifecycleAndDiscovery(t *testing.T, engine acceptance.Engine) {
	t.Helper()
	requireAcceptanceEnabled(t)
	prefix := requiredEnvironment(t, "HUBSPOT_ACCEPTANCE_PREFIX")
	ledger := requiredEnvironment(t, "HUBSPOT_ACCEPTANCE_CLEANUP_LEDGER")

	acceptance.Run(t, acceptance.Options{
		Engine:     engine,
		Shard:      acceptance.FreeProperties,
		Prefix:     prefix,
		LedgerPath: ledger,
	}, func(session *acceptance.Session) {
		initial := configForEngine(livePropertyConfig(prefix, false), engine)
		session.Apply(initial)
		session.RequireStateString("hubspot_property.scalar", "label", "Acceptance scalar property")
		session.RequireStateString("hubspot_property.scalar", "id", "contacts/"+prefix+"scalar")
		session.RequireEmptyPlan(initial)

		updated := configForEngine(livePropertyConfig(prefix, true), engine)
		session.Apply(updated)
		session.RequireEmptyPlan(updated)
		session.MutatePropertyLabel("contacts", prefix+"scalar", "Out-of-band scalar label")
		session.RequirePlanDiffAttributes(updated, "hubspot_property.scalar", "label")
		session.Apply(updated)
		session.MutatePropertyLabel("contacts", prefix+"enumeration", "Out-of-band enumeration label")
		session.RequirePlanDiffAttributes(updated, "hubspot_property.enumeration", "label")
		session.Apply(updated)
		session.RemoveState("hubspot_property.scalar")
		session.RemoveState("hubspot_property.enumeration")
		session.Import("hubspot_property.scalar", "contacts/"+prefix+"scalar")
		session.Import("hubspot_property.enumeration", "contacts/"+prefix+"enumeration")
		session.RequireEmptyPlan(updated)
		session.Destroy(updated)
		session.RequirePropertyAbsent("contacts", prefix+"scalar")
		session.RequirePropertyAbsent("contacts", prefix+"enumeration")
		session.RequirePropertyGroupAbsent("contacts", prefix+"property_group")
		session.RequirePropertyArchived("contacts", prefix+"scalar")
		session.RequirePropertyArchived("contacts", prefix+"enumeration")
		session.RequirePropertyGroupReusable("contacts", prefix+"property_group")
		session.Apply(updated)
		session.RequireEmptyPlan(updated)
		managedOnly := strings.Split(updated, `
data "hubspot_property_definition"`)[0]
		session.Apply(managedOnly)
		session.ArchiveProperty("contacts", prefix+"scalar")
		session.Refresh(managedOnly)
		session.RequireStateAbsent("hubspot_property.scalar")
		session.Apply(updated)
		session.RequireEmptyPlan(updated)

		session.ArchiveProperty("contacts", prefix+"scalar")
		session.Refresh(managedOnly)
		archivedDiscovery := configForEngine(liveArchivedDiscoveryConfig(prefix), engine)
		session.Apply(archivedDiscovery)
		session.RequireStateString("data.hubspot_property_definition.archived", "id", "contacts/"+prefix+"scalar")
		session.RequireStateMapKey("data.hubspot_property_definitions.archived", "definitions", prefix+"scalar", true)
		session.RequireStateMapKey("data.hubspot_property_definitions.active", "definitions", prefix+"scalar", false)
		session.RequireEmptyPlan(archivedDiscovery)
		session.RequireApplyFailure(configForEngine(liveActiveMissingDiscoveryConfig(prefix), engine))

		builtInDiscovery := configForEngine(liveBuiltInDiscoveryConfig(false), engine)
		session.Apply(builtInDiscovery)
		session.RequireStateMapNestedStringOneOf("data.hubspot_property_definitions.built_in", "definitions", "date_display_hint", "absolute", "absolute_with_relative", "time_since", "time_until")
		session.RequireValidationFailure(configForEngine(liveInvalidDateDisplayHintConfig(), engine), "Unsupported argument")
		session.Apply(builtInDiscovery)
		session.RequireImportFailure(configForEngine(liveBuiltInDiscoveryConfig(true), engine), "hubspot_property.readonly", "contacts/email", "Property is discovery-only")
	})
	requireFreeOwnedConfigurationAbsent(t, prefix)
}

func TestAcc_free_properties_OptionAndTypeWarnings(t *testing.T) {
	runOptionAndTypeWarnings(t, acceptance.OpenTofu)
}

func TestAcc_free_properties_OptionAndTypeWarningsTerraformParity(t *testing.T) {
	runOptionAndTypeWarnings(t, acceptance.Terraform)
}

func runOptionAndTypeWarnings(t *testing.T, engine acceptance.Engine) {
	t.Helper()
	requireAcceptanceEnabled(t)
	prefix := requiredEnvironment(t, "HUBSPOT_ACCEPTANCE_PREFIX")
	ledger := requiredEnvironment(t, "HUBSPOT_ACCEPTANCE_CLEANUP_LEDGER")

	acceptance.Run(t, acceptance.Options{
		Engine:     engine,
		Shard:      acceptance.FreeProperties,
		Prefix:     prefix,
		LedgerPath: ledger,
	}, func(session *acceptance.Session) {
		initial := configForEngine(livePropertyConfig(prefix, false), engine)
		session.Apply(initial)
		session.RequireEmptyPlan(initial)

		transitions := warningTransitionConfigs(prefix)
		for index, transition := range transitions {
			transitions[index] = configForEngine(transition, engine)
		}
		safe := transitions[0]
		session.RequirePlanWithoutWarning(safe, acceptance.PropertyOptionValuesChanged)
		session.Apply(safe)
		typeTransition := transitions[1]
		session.RequirePlanWarning(typeTransition, acceptance.PropertyTypeTransition)
		session.Apply(typeTransition)
		session.RequireEmptyPlan(typeTransition)
		optionRemoval := transitions[2]
		session.RequirePlanWarning(optionRemoval, acceptance.PropertyOptionValuesChanged)
		session.Apply(optionRemoval)
		session.RequireEmptyPlan(optionRemoval)

		optionReplacement := transitions[3]
		session.RequirePlanWarning(optionReplacement, acceptance.PropertyOptionValuesChanged)
		session.Apply(optionReplacement)
		session.RequireEmptyPlan(optionReplacement)

		storageTransition := transitions[4]
		session.RequirePlanWarning(storageTransition, acceptance.PropertyTypeTransition)
		session.Apply(storageTransition)
		session.RequireEmptyPlan(storageTransition)
	})
	requireFreeOwnedConfigurationAbsent(t, prefix)
}

func configForEngine(config string, engine acceptance.Engine) string {
	if engine == acceptance.Terraform {
		return strings.Replace(config, "registry.opentofu.org", "registry.terraform.io", 1)
	}
	return config
}

func TestAcc_free_properties_StandardObjectTypeCoverage(t *testing.T) {
	runStandardObjectTypeCoverage(t, acceptance.OpenTofu, "registry.opentofu.org")
}

func TestAcc_free_properties_StandardObjectTypeTerraformParity(t *testing.T) {
	runStandardObjectTypeCoverage(t, acceptance.Terraform, "registry.terraform.io")
}

func runStandardObjectTypeCoverage(t *testing.T, engine acceptance.Engine, registryHost string) {
	t.Helper()
	requireAcceptanceEnabled(t)
	prefix := requiredEnvironment(t, "HUBSPOT_ACCEPTANCE_PREFIX")
	ledger := requiredEnvironment(t, "HUBSPOT_ACCEPTANCE_CLEANUP_LEDGER")

	acceptance.Run(t, acceptance.Options{
		Engine:     engine,
		Shard:      acceptance.FreeProperties,
		Prefix:     prefix,
		LedgerPath: ledger,
	}, func(session *acceptance.Session) {
		for _, objectType := range []string{"contacts", "companies", "deals", "tickets"} {
			config := liveStandardObjectConfig(prefix, objectType, registryHost)
			session.Apply(config)
			session.RequireEmptyPlan(config)
			session.MutatePropertyLabel(objectType, prefix+objectType+"_property", "Out-of-band "+objectType+" property")
			session.RequirePlanDiffAttributes(config, "hubspot_property.test", "label")
			session.Apply(config)
			session.RemoveState("hubspot_property.test")
			session.Import("hubspot_property.test", objectType+"/"+prefix+objectType+"_property")
			session.RequireEmptyPlan(config)
			session.ArchiveProperty(objectType, prefix+objectType+"_property")
			session.Refresh(config)
			session.RequireStateAbsent("hubspot_property.test")
			session.Apply(config)
			session.RequireEmptyPlan(config)
			session.Destroy(config)
			session.RequirePropertyAbsent(objectType, prefix+objectType+"_property")
			session.RequirePropertyGroupAbsent(objectType, prefix+objectType+"_group")
			session.RequirePropertyArchived(objectType, prefix+objectType+"_property")
			session.RequirePropertyGroupReusable(objectType, prefix+objectType+"_group")
		}
	})
	requireFreeOwnedConfigurationAbsentForStandardObjectTypes(t, prefix)
}

func requireAcceptanceEnabled(t *testing.T) {
	t.Helper()
	if os.Getenv("HUBSPOT_ACCEPTANCE") != "1" {
		t.Fatal("live acceptance requires HUBSPOT_ACCEPTANCE=1")
	}
}

func requiredEnvironment(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("live acceptance requires %s", name)
	}
	return value
}

func livePropertyGroupConfig(name, label string, displayOrder *int64) string {
	order := ""
	if displayOrder != nil {
		order = fmt.Sprintf("  display_order = %d\n", *displayOrder)
	}
	return fmt.Sprintf(`
terraform {
  required_providers {
    hubspot = {
      source = "registry.opentofu.org/jackemcpherson/hubspot"
    }
  }
}

provider "hubspot" {}

resource "hubspot_property_group" "test" {
  object_type = "contacts"
  name        = %q
  label       = %q
%s
}
`, name, label, order)
}

func liveStandardObjectConfig(prefix, objectType, registryHost string) string {
	groupName := prefix + objectType + "_group"
	propertyName := prefix + objectType + "_property"
	return fmt.Sprintf(`
terraform {
  required_providers {
    hubspot = {
      source = %q
    }
  }
}

provider "hubspot" {}

resource "hubspot_property_group" "test" {
  object_type = %q
  name        = %q
  label       = %q
}

resource "hubspot_property" "test" {
  object_type = %q
  name        = %q
  label       = %q
  group_name  = hubspot_property_group.test.name
  type        = "string"
  field_type  = "text"
}
`, registryHost+"/jackemcpherson/hubspot", objectType, groupName, "Acceptance "+objectType+" properties", objectType, propertyName, "Acceptance "+objectType+" property")
}

func livePropertyConfig(prefix string, updated bool) string {
	groupLabel := "Acceptance property definitions"
	scalarLabel := "Acceptance scalar property"
	scalarDescription := ""
	alphaLabel := "Alpha"
	scalarGroup := "hubspot_property_group.test.name"
	scalarOrder := int64(220)
	scalarHidden := false
	alphaOrder := int64(260)
	if updated {
		groupLabel = "Updated acceptance property definitions"
		scalarLabel = "Updated acceptance scalar property"
		scalarDescription = "Updated through the provider acceptance lifecycle"
		alphaLabel = "Alpha updated"
		scalarGroup = "hubspot_property_group.secondary.name"
		scalarOrder = 230
		scalarHidden = true
		alphaOrder = 250
	}
	return fmt.Sprintf(`
terraform {
  required_providers {
    hubspot = {
      source = "registry.opentofu.org/jackemcpherson/hubspot"
    }
  }
}

provider "hubspot" {}

resource "hubspot_property_group" "test" {
  object_type = "contacts"
  name        = %q
  label       = %q
  display_order = 200
}

resource "hubspot_property_group" "secondary" {
  object_type = "contacts"
  name        = %q
  label       = "Acceptance secondary property definitions"
  display_order = 210
}

resource "hubspot_property" "scalar" {
  object_type = "contacts"
  name        = %q
  label       = %q
  group_name  = %s
  type        = "string"
  field_type  = "text"
  description = %q
  display_order = %d
  hidden      = %t
}

resource "hubspot_property" "enumeration" {
  object_type = "contacts"
  name        = %q
  label       = "Acceptance enumeration property"
  group_name  = hubspot_property_group.test.name
  type        = "enumeration"
  field_type  = "select"
  display_order = 240

  options = {
    alpha = { label = %q, display_order = %d }
    beta  = { label = "Beta", display_order = 270 }
  }
}

data "hubspot_property_definition" "scalar" {
  object_type = "contacts"
  name        = hubspot_property.scalar.name
}

data "hubspot_property_definitions" "owned" {
  object_type = "contacts"
  depends_on  = [hubspot_property.enumeration]
}
`, prefix+"property_group", groupLabel, prefix+"secondary_group", prefix+"scalar", scalarLabel, scalarGroup, scalarDescription, scalarOrder, scalarHidden, prefix+"enumeration", alphaLabel, alphaOrder)
}

func liveBlockedGroupConfig(prefix string, includeGroup bool) string {
	group := ""
	dependency := ""
	if includeGroup {
		group = fmt.Sprintf(`
resource "hubspot_property_group" "blocked" {
  object_type = "contacts"
  name        = %q
  label       = "Acceptance blocked property group"
}
`, prefix+"blocked_group")
		dependency = "depends_on = [hubspot_property_group.blocked]"
	}
	return fmt.Sprintf(`
terraform {
  required_providers {
    hubspot = {
      source = "registry.opentofu.org/jackemcpherson/hubspot"
    }
  }
}

provider "hubspot" {}
%s
resource "hubspot_property" "blocker" {
  object_type = "contacts"
  name        = %q
  label       = "Acceptance group deletion blocker"
  group_name  = %q
  type        = "string"
  field_type  = "text"
  %s
}
`, group, prefix+"blocked_property", prefix+"blocked_group", dependency)
}

func liveProviderOnlyConfig() string {
	return `
terraform {
  required_providers {
    hubspot = {
      source = "registry.opentofu.org/jackemcpherson/hubspot"
    }
  }
}

provider "hubspot" {}
`
}

func liveArchivedDiscoveryConfig(prefix string) string {
	return fmt.Sprintf(`
terraform {
  required_providers {
    hubspot = {
      source = "registry.opentofu.org/jackemcpherson/hubspot"
    }
  }
}

provider "hubspot" {}

data "hubspot_property_definition" "archived" {
  object_type = "contacts"
  name        = %q
  archived    = true
}

data "hubspot_property_definitions" "archived" {
  object_type = "contacts"
  archived    = true
}

data "hubspot_property_definitions" "active" {
  object_type = "contacts"
  archived    = false
}
`, prefix+"scalar")
}

func liveActiveMissingDiscoveryConfig(prefix string) string {
	return fmt.Sprintf(`
terraform {
  required_providers {
    hubspot = {
      source = "registry.opentofu.org/jackemcpherson/hubspot"
    }
  }
}

provider "hubspot" {}

data "hubspot_property_definition" "active_missing" {
  object_type = "contacts"
  name        = %q
  archived    = false
}
`, prefix+"scalar")
}

func liveBuiltInDiscoveryConfig(includeManagedImport bool) string {
	resource := ""
	if includeManagedImport {
		resource = `
resource "hubspot_property" "readonly" {
  object_type = "contacts"
  name        = "email"
  label       = "Email"
  group_name  = "contactinformation"
  type        = "string"
  field_type  = "text"
}
`
	}
	return fmt.Sprintf(`
terraform {
  required_providers {
    hubspot = {
      source = "registry.opentofu.org/jackemcpherson/hubspot"
    }
  }
}

provider "hubspot" {}

data "hubspot_property_definition" "createdate" {
  object_type = "contacts"
  name        = "createdate"
}

data "hubspot_property_definitions" "built_in" {
  object_type = "contacts"
}
%s
`, resource)
}

func liveInvalidDateDisplayHintConfig() string {
	return `
terraform {
  required_providers {
    hubspot = {
      source = "registry.opentofu.org/jackemcpherson/hubspot"
    }
  }
}

provider "hubspot" {}

resource "hubspot_property" "invalid_date_hint" {
  object_type       = "contacts"
  name              = "validation_only"
  label             = "Validation only"
  group_name        = "contactinformation"
  type              = "date"
  field_type        = "date"
  date_display_hint = "absolute"
}
`
}
