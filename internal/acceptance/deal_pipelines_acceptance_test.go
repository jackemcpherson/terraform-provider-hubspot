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

func TestDealPipelinesAcceptanceConfigurationSyntax(t *testing.T) {
	updated := liveDealPipelineConfig("tf_acc_syntax_", true, nil, "registry.opentofu.org/jackemcpherson/hubspot")
	withNurture := strings.Replace(updated, `    "open" = {`, `    "nurture" = {
      label         = "Acceptance nurture stage"
      display_order = 35
      metadata      = { probability = "0.5" }
    }
    "open" = {`, 1)
	configs := []string{
		liveDealPipelineConfig("tf_acc_syntax_", false, nil, "registry.opentofu.org/jackemcpherson/hubspot"),
		updated,
		strings.Replace(withNurture, `    "nurture" = {`, `    "follow_up" = {`, 1),
		liveDealPipelineConfig("tf_acc_syntax_", true, map[string]string{"open": "stage-open", "closed": "stage-closed"}, "registry.opentofu.org/jackemcpherson/hubspot"),
	}
	for index, config := range configs {
		directory := t.TempDir()
		path := filepath.Join(directory, "main.tf")
		if err := os.WriteFile(path, []byte(config), 0o600); err != nil {
			t.Fatalf("write deal-pipeline syntax fixture %d: %v", index, err)
		}
		command := exec.Command("tofu", "fmt", path)
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("deal-pipeline syntax fixture %d is invalid: %v: %s", index, err, strings.TrimSpace(string(output)))
		}
	}
}

func TestAcc_deal_pipelines_Lifecycle(t *testing.T) {
	requireAcceptanceEnabled(t)
	prefix := requiredEnvironment(t, "HUBSPOT_ACCEPTANCE_PREFIX")
	ledger := requiredEnvironment(t, "HUBSPOT_ACCEPTANCE_CLEANUP_LEDGER")

	acceptance.Run(t, acceptance.Options{
		Engine: acceptance.OpenTofu, Shard: acceptance.DealPipelines,
		Prefix: prefix, LedgerPath: ledger,
	}, func(session *acceptance.Session) {
		initial := liveDealPipelineConfig(prefix, false, nil, "registry.opentofu.org/jackemcpherson/hubspot")
		session.Apply(initial)
		session.RequireStateStringPrefix("hubspot_pipeline.test", "id", "deals/")
		initialIDs := session.OpaqueStateMapNestedStrings("hubspot_pipeline.test", "stages", "id")
		session.RequireEmptyPlan(initial)

		updated := liveDealPipelineConfig(prefix, true, nil, "registry.opentofu.org/jackemcpherson/hubspot")
		session.Apply(updated)
		updatedIDs := session.OpaqueStateMapNestedStrings("hubspot_pipeline.test", "stages", "id")
		if !sameOpaqueIdentities(initialIDs, updatedIDs) {
			t.Fatal("deal-pipeline update changed a generated stage identity")
		}
		session.RequireEmptyPlan(updated)

		withNurture := strings.Replace(updated, `    "open" = {`, `    "nurture" = {
      label         = "Acceptance nurture stage"
      display_order = 35
      metadata      = { probability = "0.5" }
    }
    "open" = {`, 1)
		session.Apply(withNurture)
		withNurtureIDs := session.OpaqueStateMapNestedStrings("hubspot_pipeline.test", "stages", "id")
		if withNurtureIDs["open"] != updatedIDs["open"] || withNurtureIDs["closed"] != updatedIDs["closed"] {
			t.Fatal("deal-pipeline stage addition changed an existing identity")
		}
		session.RequireEmptyPlan(withNurture)
		renamedStageKey := strings.Replace(withNurture, `    "nurture" = {`, `    "follow_up" = {`, 1)
		session.Apply(renamedStageKey)
		renamedIDs := session.OpaqueStateMapNestedStrings("hubspot_pipeline.test", "stages", "id")
		if renamedIDs["open"] != updatedIDs["open"] || renamedIDs["closed"] != updatedIDs["closed"] {
			t.Fatal("deal-pipeline logical-key replacement changed an unrelated identity")
		}
		session.Apply(updated)
		session.RequireEmptyPlan(updated)

		remoteStageID := session.CreatePipelineStageOutOfBand("hubspot_pipeline.test", prefix+"remote_stage", 50, map[string]string{"probability": "0.7"})
		session.Refresh(updated)
		session.RequireStateMapKey("hubspot_pipeline.test", "stages", remoteStageID, true)
		session.Apply(updated)
		session.RequireEmptyPlan(updated)

		session.MutatePipeline("hubspot_pipeline.test", prefix+"out_of_band_pipeline", 130, "open", "Out-of-band open stage", 35, map[string]string{"probability": "0.3"})
		session.RequirePlanDiffAttributes(updated, "hubspot_pipeline.test", "display_order", "label", "stages")
		session.Apply(updated)
		session.RequireEmptyPlan(updated)

		compositeID := session.OpaqueStateString("hubspot_pipeline.test", "id")
		stageIDs := session.OpaqueStateMapNestedStrings("hubspot_pipeline.test", "stages", "id")
		imported := liveDealPipelineConfig(prefix, true, stageIDs, "registry.opentofu.org/jackemcpherson/hubspot")
		session.RemoveState("hubspot_pipeline.test")
		session.Import("hubspot_pipeline.test", compositeID)
		session.RequireEmptyPlan(imported)

		session.ArchivePipeline("hubspot_pipeline.test")
		session.Refresh(imported)
		session.RequirePlanDiffAttributes(imported, "hubspot_pipeline.test", "id")
		session.Apply(imported)
		session.RequireEmptyPlan(imported)
		session.Destroy(imported)
		session.RequirePipelineArchived("deals", compositeID)
	})
}

func TestAcc_deal_pipelines_TerraformParity(t *testing.T) {
	requireAcceptanceEnabled(t)
	prefix := requiredEnvironment(t, "HUBSPOT_ACCEPTANCE_PREFIX") + "terraform_"
	ledger := requiredEnvironment(t, "HUBSPOT_ACCEPTANCE_CLEANUP_LEDGER")
	acceptance.Run(t, acceptance.Options{
		Engine: acceptance.Terraform, Shard: acceptance.DealPipelines,
		Prefix: prefix, LedgerPath: ledger,
	}, func(session *acceptance.Session) {
		initial := liveDealPipelineConfig(prefix, false, nil, "registry.terraform.io/jackemcpherson/hubspot")
		session.Apply(initial)
		updated := liveDealPipelineConfig(prefix, true, nil, "registry.terraform.io/jackemcpherson/hubspot")
		session.Apply(updated)
		session.MutatePipeline("hubspot_pipeline.test", prefix+"terraform_out_of_band", 140, "open", "Terraform out-of-band stage", 45, map[string]string{"probability": "0.4"})
		session.RequirePlanDiffAttributes(updated, "hubspot_pipeline.test", "display_order", "label", "stages")
		session.Apply(updated)
		session.RequireEmptyPlan(updated)
		compositeID := session.OpaqueStateString("hubspot_pipeline.test", "id")
		stageIDs := session.OpaqueStateMapNestedStrings("hubspot_pipeline.test", "stages", "id")
		imported := liveDealPipelineConfig(prefix, true, stageIDs, "registry.terraform.io/jackemcpherson/hubspot")
		session.RemoveState("hubspot_pipeline.test")
		session.Import("hubspot_pipeline.test", compositeID)
		session.RequireEmptyPlan(imported)
		session.Destroy(imported)
		session.RequirePipelineArchived("deals", compositeID)
	})
}

func sameOpaqueIdentities(before, after map[string]string) bool {
	if len(before) != len(after) {
		return false
	}
	for key, value := range before {
		if after[key] != value {
			return false
		}
	}
	return true
}

func liveDealPipelineConfig(prefix string, updated bool, importedIDs map[string]string, providerSource string) string {
	pipelineLabel := prefix + "deal_pipeline"
	pipelineOrder := int64(100)
	openLabel := "Acceptance open stage"
	openOrder := int64(10)
	openProbability := "0.1"
	if updated {
		pipelineLabel = prefix + "updated_deal_pipeline"
		pipelineOrder = 120
		openLabel = "Updated acceptance open stage"
		openOrder = 30
		openProbability = "0.2"
	}
	stages := map[string]struct {
		Label       string
		Order       int64
		Probability string
	}{
		"open":   {Label: openLabel, Order: openOrder, Probability: openProbability},
		"closed": {Label: "Acceptance closed stage", Order: 40, Probability: "1.0"},
	}
	keys := []string{"open", "closed"}
	sort.Strings(keys)
	var stageConfig strings.Builder
	for _, logicalKey := range keys {
		stateKey := logicalKey
		if importedIDs != nil {
			stateKey = importedIDs[logicalKey]
		}
		stage := stages[logicalKey]
		fmt.Fprintf(&stageConfig, `    %q = {
      label         = %q
      display_order = %d
      metadata      = { probability = %q }
    }
`, stateKey, stage.Label, stage.Order, stage.Probability)
	}
	return fmt.Sprintf(`
terraform {
  required_providers {
    hubspot = {
      source = %q
    }
  }
}

provider "hubspot" {}

resource "hubspot_pipeline" "test" {
  object_type   = "deals"
  label         = %q
  display_order = %d

  stages = {
%s  }
}
`, providerSource, pipelineLabel, pipelineOrder, stageConfig.String())
}
