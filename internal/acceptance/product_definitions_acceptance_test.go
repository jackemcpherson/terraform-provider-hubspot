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

const liveProductAddress = "hubspot_product.test"

func TestProductDefinitionsAcceptanceConfigurationSyntax(t *testing.T) {
	for _, config := range []string{
		liveProductDefinitionConfig("registry.opentofu.org/jackemcpherson/hubspot", "tf_acc_syntax_product", false),
		liveProductDefinitionConfig("registry.terraform.io/jackemcpherson/hubspot", "tf_acc_syntax_product", true),
	} {
		path := filepath.Join(t.TempDir(), "main.tf")
		if err := os.WriteFile(path, []byte(config), 0o600); err != nil {
			t.Fatalf("write Product acceptance syntax fixture: %v", err)
		}
		if output, err := exec.Command("tofu", "fmt", "-check", path).CombinedOutput(); err != nil {
			t.Fatalf("Product acceptance syntax fixture is invalid: %v: %s", err, strings.TrimSpace(string(output)))
		}
	}
}

func TestAcc_product_definitions_OpenTofuLifecycle(t *testing.T) {
	runLiveProductDefinitionLifecycle(t, acceptance.OpenTofu, "registry.opentofu.org/jackemcpherson/hubspot")
}

func TestAcc_product_definitions_TerraformLifecycle(t *testing.T) {
	runLiveProductDefinitionLifecycle(t, acceptance.Terraform, "registry.terraform.io/jackemcpherson/hubspot")
}

func runLiveProductDefinitionLifecycle(t *testing.T, engine acceptance.Engine, providerSource string) {
	t.Helper()
	requireAcceptanceEnabled(t)
	prefix := requiredEnvironment(t, "HUBSPOT_ACCEPTANCE_PREFIX") + string(engine) + "_"
	ledger := requiredEnvironment(t, "HUBSPOT_ACCEPTANCE_CLEANUP_LEDGER")
	sku := prefix + "managed_product"
	initial := liveProductDefinitionConfig(providerSource, sku, false)
	updated := liveProductDefinitionConfig(providerSource, sku, true)
	var initialID, finalID string

	acceptance.Run(t, acceptance.Options{
		Engine: engine, Shard: acceptance.ProductDefinitions, Prefix: prefix, LedgerPath: ledger,
	}, func(session *acceptance.Session) {
		session.Apply(initial)
		initialID = session.OpaqueStateString(liveProductAddress, "id")
		session.RequireEmptyPlan(initial)

		session.Apply(updated)
		session.RequireEmptyPlan(updated)
		session.MutateProduct(liveProductAddress)
		session.RequirePlanDiffAttributes(updated, liveProductAddress, "cost", "description", "price", "recurring_billing_period")
		session.Apply(updated)
		session.RequireEmptyPlan(updated)

		session.RemoveState(liveProductAddress)
		session.Import(liveProductAddress, initialID)
		session.RequireEmptyPlan(updated)

		if archivedID := session.ArchiveProduct(liveProductAddress); archivedID != initialID {
			t.Fatal("external Product archival did not preserve the exact generated identity")
		}
		session.Refresh(updated)
		session.RequireStateAbsent(liveProductAddress)
		session.Apply(updated)
		finalID = session.OpaqueStateString(liveProductAddress, "id")
		if finalID == initialID {
			t.Fatal("external Product archival recreation reused the terminal generated identity")
		}
		session.RequireEmptyPlan(updated)

		session.Destroy(updated)
		session.RequireStateAbsent(liveProductAddress)
		session.RequireProductsTerminal(prefix, initialID, finalID)
	})
}

func liveProductDefinitionConfig(providerSource, sku string, updated bool) string {
	description := "Annual support"
	price := "1200.00"
	cost := "300.00"
	recurrence := "P12M"
	if updated {
		description = "Priority annual support"
		price = "1500.00"
		cost = ""
		recurrence = ""
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

resource "hubspot_product" "test" {
  name                     = "Provider acceptance Product"
  sku                      = %q
  description              = %q
  price                    = %q
  cost                     = %q
  recurring_billing_period = %q
}
`, providerSource, sku, description, price, cost, recurrence)
}
