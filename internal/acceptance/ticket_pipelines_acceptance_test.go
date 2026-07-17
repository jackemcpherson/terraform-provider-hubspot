// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

//go:build acceptance && deferred

package acceptance_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/jackemcpherson/terraform-provider-hubspot/internal/acceptance"
)

func TestTicketPipelinesAcceptanceConfigurationSyntax(t *testing.T) {
	for index, config := range []string{
		liveTicketPipelineConfig("tf_acc_syntax_", false, nil, "registry.opentofu.org/jackemcpherson/hubspot"),
		liveTicketPipelineConfig("tf_acc_syntax_", true, map[string]string{"open": "stage-open", "closed": "stage-closed"}, "registry.terraform.io/jackemcpherson/hubspot"),
	} {
		path := filepath.Join(t.TempDir(), "main.tf")
		if err := os.WriteFile(path, []byte(config), 0o600); err != nil {
			t.Fatalf("write ticket-pipeline syntax fixture %d: %v", index, err)
		}
		if output, err := exec.Command("tofu", "fmt", path).CombinedOutput(); err != nil {
			t.Fatalf("ticket-pipeline syntax fixture %d is invalid: %v: %s", index, err, strings.TrimSpace(string(output)))
		}
	}
}

func TestAcc_ticket_pipelines_Lifecycle(t *testing.T) {
	requireAcceptanceEnabled(t)
	prefix := requiredEnvironment(t, "HUBSPOT_ACCEPTANCE_PREFIX")
	ledger := requiredEnvironment(t, "HUBSPOT_ACCEPTANCE_CLEANUP_LEDGER")
	acceptance.Run(t, acceptance.Options{Engine: acceptance.OpenTofu, Shard: acceptance.TicketPipelines, Prefix: prefix, LedgerPath: ledger}, func(session *acceptance.Session) {
		initial := liveTicketPipelineConfig(prefix, false, nil, "registry.opentofu.org/jackemcpherson/hubspot")
		invalid := strings.Replace(initial, `ticketState = "OPEN"`, `ticketState = "open"`, 1)
		session.RequirePlanFailure(invalid, "Invalid pipeline stage metadata")
		session.Apply(initial)
		initialIDs := session.OpaqueStateMapNestedStrings("hubspot_pipeline.test", "stages", "id")
		session.RequireEmptyPlan(initial)

		updated := liveTicketPipelineConfig(prefix, true, nil, "registry.opentofu.org/jackemcpherson/hubspot")
		session.Apply(updated)
		if !sameOpaqueIdentities(initialIDs, session.OpaqueStateMapNestedStrings("hubspot_pipeline.test", "stages", "id")) {
			t.Fatal("ticket-pipeline update changed a generated stage identity")
		}
		session.RequireEmptyPlan(updated)
		session.MutatePipeline("hubspot_pipeline.test", prefix+"ticket_out_of_band", 130, "open", "Out-of-band ticket stage", 35, map[string]string{"ticketState": "OPEN"})
		session.RequirePlanDiffAttributes(updated, "hubspot_pipeline.test", "display_order", "label", "stages")
		session.Apply(updated)
		session.RequireEmptyPlan(updated)

		compositeID := session.OpaqueStateString("hubspot_pipeline.test", "id")
		stageIDs := session.OpaqueStateMapNestedStrings("hubspot_pipeline.test", "stages", "id")
		imported := liveTicketPipelineConfig(prefix, true, stageIDs, "registry.opentofu.org/jackemcpherson/hubspot")
		session.RemoveState("hubspot_pipeline.test")
		session.Import("hubspot_pipeline.test", compositeID)
		session.RequireEmptyPlan(imported)
		session.ArchivePipeline("hubspot_pipeline.test")
		session.Refresh(imported)
		session.RequirePlanDiffAttributes(imported, "hubspot_pipeline.test", "id")
		session.Apply(imported)
		restoredIDs := session.OpaqueStateMapNestedStrings("hubspot_pipeline.test", "stages", "id")
		if !sameOpaqueIdentities(stageIDs, restoredIDs) {
			t.Fatal("ticket-pipeline restore changed a generated stage identity")
		}
		session.RequireEmptyPlan(imported)
		session.Destroy(imported)
		session.RequirePipelineArchived("tickets", compositeID)
	})
}

func TestAcc_ticket_pipelines_TerraformParity(t *testing.T) {
	requireAcceptanceEnabled(t)
	prefix := requiredEnvironment(t, "HUBSPOT_ACCEPTANCE_PREFIX") + "terraform_"
	ledger := requiredEnvironment(t, "HUBSPOT_ACCEPTANCE_CLEANUP_LEDGER")
	acceptance.Run(t, acceptance.Options{Engine: acceptance.Terraform, Shard: acceptance.TicketPipelines, Prefix: prefix, LedgerPath: ledger}, func(session *acceptance.Session) {
		config := liveTicketPipelineConfig(prefix, false, nil, "registry.terraform.io/jackemcpherson/hubspot")
		session.Apply(config)
		session.RequireEmptyPlan(config)
		compositeID := session.OpaqueStateString("hubspot_pipeline.test", "id")
		session.Destroy(config)
		session.RequirePipelineArchived("tickets", compositeID)
	})
}

func liveTicketPipelineConfig(prefix string, updated bool, importedIDs map[string]string, providerSource string) string {
	label := prefix + "ticket_pipeline"
	order := int64(100)
	openLabel := "Acceptance open ticket stage"
	if updated {
		label = prefix + "updated_ticket_pipeline"
		order = 120
		openLabel = "Updated acceptance open ticket stage"
	}
	stages := map[string]struct{ Label, State string }{
		"open":   {openLabel, "OPEN"},
		"closed": {"Acceptance closed ticket stage", "CLOSED"},
	}
	keys := []string{"open", "closed"}
	sort.Strings(keys)
	var stageConfig strings.Builder
	for index, key := range keys {
		stateKey := key
		if importedIDs != nil {
			stateKey = importedIDs[key]
		}
		stage := stages[key]
		fmt.Fprintf(&stageConfig, `    %q = {
      label         = %q
      display_order = %d
      metadata = {
        ticketState = %q
      }
    }
`, stateKey, stage.Label, (index+1)*10, stage.State)
	}
	return fmt.Sprintf(`
terraform {
  required_providers { hubspot = { source = %q } }
}
provider "hubspot" {}
resource "hubspot_pipeline" "test" {
  object_type   = "tickets"
  label         = %q
  display_order = %d
  stages = {
%s  }
}
`, providerSource, label, order, stageConfig.String())
}
