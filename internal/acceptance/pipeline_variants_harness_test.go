// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

//go:build deferred

package acceptance_test

import (
	"fmt"
	"net/http/httptest"
	"os/exec"
	"sort"
	"strings"
	"testing"

	"github.com/jackemcpherson/terraform-provider-hubspot/internal/acceptance"
)

func TestRunPreservesTicketPipelineMetadata(t *testing.T) {
	if _, err := exec.LookPath("tofu"); err != nil {
		t.Skip("pinned OpenTofu executable is not installed")
	}
	api := newPipelineAPIForObjectType(t, "tickets")
	server := httptest.NewServer(api)
	t.Cleanup(server.Close)
	ledger := t.TempDir() + "/cleanup.jsonl"

	acceptance.Run(t, acceptance.Options{
		Engine: acceptance.OpenTofu, Shard: acceptance.TicketPipelines,
		Prefix: "tf_acc_harness_", LedgerPath: ledger,
	}, func(session *acceptance.Session) {
		initial := pipelineVariantConfig(server.URL, "tickets", "Ticket pipeline", map[string]map[string]string{
			"open": {"ticketState": "OPEN"},
			"done": {"ticketState": "CLOSED"},
		})
		invalid := strings.Replace(initial, `"ticketState" = "OPEN"`, `"ticketState" = "open"`, 1)
		session.RequirePlanFailure(invalid, "Invalid pipeline stage metadata")
		unsupported := strings.Replace(initial, `"ticketState" = "OPEN"`, `"ticketState" = "OPEN"
        "extension" = "unsupported"`, 1)
		session.RequirePlanFailure(unsupported, "Invalid pipeline stage metadata")
		session.Apply(initial)
		initialIDs := api.stageIDs()
		session.RequireEmptyPlan(initial)

		updated := pipelineVariantConfig(server.URL, "tickets", "Updated ticket pipeline", map[string]map[string]string{
			"open": {"ticketState": "OPEN"},
			"done": {"ticketState": "CLOSED"},
		})
		session.Apply(updated)
		if got := api.stageIDs(); strings.Join(got, ",") != strings.Join(initialIDs, ",") {
			t.Fatal("ticket pipeline update changed generated stage identities")
		}
		session.RequireEmptyPlan(updated)
		api.mutateStageMetadata(initialIDs[0], "Out-of-band ticket stage", 55, map[string]string{"ticketState": "OPEN"})
		session.RequirePlanDiffAttributes(updated, "hubspot_pipeline.test", "stages")
		session.Apply(updated)
		session.RequireEmptyPlan(updated)
	})
	if api.isActive() || !api.isArchived() {
		t.Fatal("ticket pipeline cleanup did not verify archived terminal state")
	}
}

func TestRunPreservesUnknownCustomPipelineMetadata(t *testing.T) {
	if _, err := exec.LookPath("tofu"); err != nil {
		t.Skip("pinned OpenTofu executable is not installed")
	}
	const objectType = "2-123456"
	api := newPipelineAPIForObjectType(t, objectType)
	server := httptest.NewServer(api)
	t.Cleanup(server.Close)
	ledger := t.TempDir() + "/cleanup.jsonl"

	acceptance.Run(t, acceptance.Options{
		Engine: acceptance.OpenTofu, Shard: acceptance.CustomPipelines,
		Prefix: "tf_acc_harness_", LedgerPath: ledger,
	}, func(session *acceptance.Session) {
		config := pipelineVariantConfig(server.URL, objectType, "Custom pipeline", map[string]map[string]string{
			"queued": {"vendorState": "queued", "futureKey": "opaque"},
			"done":   {"vendorState": "done"},
		})
		session.Apply(config)
		session.RequireStateString("hubspot_pipeline.test", "id", objectType+"/pipeline-1")
		session.RequireEmptyPlan(config)
		ids := api.stageIDs()
		api.mutateStageMetadata(ids[0], "Out-of-band custom stage", 60, map[string]string{"futureKey": "remote-value"})
		session.RequirePlanDiffAttributes(config, "hubspot_pipeline.test", "stages")
		session.Apply(config)
		session.RequireEmptyPlan(config)
	})
	if api.isActive() || !api.isArchived() {
		t.Fatal("custom pipeline cleanup did not verify archived terminal state")
	}
}

func pipelineVariantConfig(apiBaseURL, objectType, label string, stages map[string]map[string]string) string {
	keys := make([]string, 0, len(stages))
	for key := range stages {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var stageConfig strings.Builder
	for index, key := range keys {
		metadataKeys := make([]string, 0, len(stages[key]))
		for metadataKey := range stages[key] {
			metadataKeys = append(metadataKeys, metadataKey)
		}
		sort.Strings(metadataKeys)
		var metadata strings.Builder
		for _, metadataKey := range metadataKeys {
			fmt.Fprintf(&metadata, "        %q = %q\n", metadataKey, stages[key][metadataKey])
		}
		fmt.Fprintf(&stageConfig, `    %q = {
      label         = %q
      display_order = %d
      metadata = {
%s      }
    }
`, key, key+" stage", (index+1)*10, metadata.String())
	}
	return fmt.Sprintf(`
terraform {
  required_providers {
    hubspot = { source = "registry.opentofu.org/jackemcpherson/hubspot" }
  }
}
provider "hubspot" {
  access_token = "acceptance-sentinel"
  api_base_url = %q
}
resource "hubspot_pipeline" "test" {
  object_type   = %q
  label         = %q
  display_order = 10
  stages = {
%s  }
}
`, apiBaseURL, objectType, label, stageConfig.String())
}
