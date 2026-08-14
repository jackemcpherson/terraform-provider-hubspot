// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

//go:build acceptance

package acceptance_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jackemcpherson/terraform-provider-hubspot/internal/acceptance"
)

const liveFormAddress = "hubspot_form_definition.test"

func TestFormDefinitionsAcceptanceConfigurationSyntax(t *testing.T) {
	for _, config := range []string{
		liveFormDefinitionConfig("registry.opentofu.org/jackemcpherson/hubspot", "tf_acc_syntax_form", false),
		liveFormDefinitionConfig("registry.terraform.io/jackemcpherson/hubspot", "tf_acc_syntax_form", true),
	} {
		path := filepath.Join(t.TempDir(), "main.tf")
		if err := os.WriteFile(path, []byte(config), 0o600); err != nil {
			t.Fatalf("write Forms acceptance syntax fixture: %v", err)
		}
		if output, err := exec.Command("tofu", "fmt", "-check", path).CombinedOutput(); err != nil {
			t.Fatalf("Forms acceptance syntax fixture is invalid: %v: %s", err, strings.TrimSpace(string(output)))
		}
	}
}

func TestAcc_form_definitions_OpenTofuLifecycle(t *testing.T) {
	runLiveFormDefinitionLifecycle(t, acceptance.OpenTofu, "registry.opentofu.org/jackemcpherson/hubspot")
}

func TestAcc_form_definitions_TerraformLifecycle(t *testing.T) {
	runLiveFormDefinitionLifecycle(t, acceptance.Terraform, "registry.terraform.io/jackemcpherson/hubspot")
}

func runLiveFormDefinitionLifecycle(t *testing.T, engine acceptance.Engine, providerSource string) {
	t.Helper()
	requireAcceptanceEnabled(t)
	prefix := requiredEnvironment(t, "HUBSPOT_ACCEPTANCE_PREFIX") + string(engine) + "_"
	ledger := requiredEnvironment(t, "HUBSPOT_ACCEPTANCE_CLEANUP_LEDGER")
	name := prefix + "managed"
	updatedName := prefix + "managed_updated"
	initial := liveFormDefinitionConfig(providerSource, name, false)
	updated := liveFormDefinitionConfig(providerSource, updatedName, true)
	var initialID, finalID string

	acceptance.Run(t, acceptance.Options{
		Engine: engine, Shard: acceptance.FormDefinitions, Prefix: prefix, LedgerPath: ledger,
	}, func(session *acceptance.Session) {
		session.Apply(initial)
		session.RequireStateString(liveFormAddress, "name", name)
		initialID = session.OpaqueStateString(liveFormAddress, "id")
		session.RequireEmptyPlan(initial)

		session.Apply(updated)
		session.RequireStateString(liveFormAddress, "name", updatedName)
		session.RequireEmptyPlan(updated)
		session.MutateFormPresentation(liveFormAddress)
		session.RequirePlanDiffAttributes(updated, liveFormAddress, "configuration", "display_options", "field_groups", "name")
		session.Apply(updated)
		session.RequireEmptyPlan(updated)

		session.RemoveState(liveFormAddress)
		session.Import(liveFormAddress, initialID)
		session.RequireEmptyPlan(updated)

		if archivedID := session.ArchiveForm(liveFormAddress); archivedID != initialID {
			t.Fatal("external archival did not preserve the exact generated identity")
		}
		session.Refresh(updated)
		session.RequireStateAbsent(liveFormAddress)
		session.Apply(updated)
		finalID = session.OpaqueStateString(liveFormAddress, "id")
		if finalID == initialID {
			t.Fatal("external archival recreation reused the terminal generated identity")
		}
		session.RequireStateString(liveFormAddress, "name", updatedName)
		session.RequireEmptyPlan(updated)

		session.Destroy(updated)
		session.RequireStateAbsent(liveFormAddress)
		session.RequireFormsTerminal(prefix, initialID, finalID)
	})

}

func liveFormDefinitionConfig(providerSource, name string, updated bool) string {
	return formDefinitionConfig(providerSource, "", name, updated)
}
