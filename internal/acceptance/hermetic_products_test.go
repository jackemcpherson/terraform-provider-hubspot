// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

//go:build acceptance

package acceptance_test

import (
	"fmt"
	"net/http/httptest"
	"os/exec"
	"testing"

	"github.com/jackemcpherson/terraform-provider-hubspot/internal/acceptance"
)

func TestHermeticProductLifecycle(t *testing.T) {
	runHermeticProductLifecycle(t, acceptance.OpenTofu, "registry.opentofu.org/jackemcpherson/hubspot")
}

func TestHermeticProductLifecycleTerraformParity(t *testing.T) {
	runHermeticProductLifecycle(t, acceptance.Terraform, "registry.terraform.io/jackemcpherson/hubspot")
}

func TestHermeticProductCreateRecovery(t *testing.T) {
	if _, err := exec.LookPath(string(acceptance.OpenTofu)); err != nil {
		t.Skip("pinned OpenTofu executable is not installed")
	}
	fake := acceptance.NewFakeHubSpot(hermeticToken, 666000668)
	server := httptest.NewServer(fake)
	t.Cleanup(server.Close)
	t.Setenv("HUBSPOT_ACCESS_TOKEN", hermeticToken)
	config := hermeticProductConfig(server.URL, "registry.opentofu.org/jackemcpherson/hubspot", "Annual support", "1200", "300", "P12M")

	acceptance.Run(t, acceptance.Options{
		Engine: acceptance.OpenTofu, Shard: acceptance.ProductDefinitions,
		Prefix: "tf_acc_hermetic_product_recovery_", LedgerPath: t.TempDir() + "/cleanup.jsonl", ProbeBaseURL: server.URL,
	}, func(session *acceptance.Session) {
		fake.FailNextProductOperation(acceptance.ProductFaultCreateKnown)
		session.Apply(config)
		knownID := session.OpaqueStateString("hubspot_product.managed", "id")
		session.RequireEmptyPlan(config)
		session.Destroy(config)

		fake.FailNextProductOperation(acceptance.ProductFaultCreateUnknown)
		session.RequireApplyFailure(config)
		active := fake.ActiveProductIDs()
		if len(active) != 1 || active[0] == knownID {
			t.Fatalf("ambiguous Product identities = %#v", active)
		}
		if !fake.ArchiveProduct(active[0]) {
			t.Fatal("could not clean ambiguous Product create")
		}
	})
}

func runHermeticProductLifecycle(t *testing.T, engine acceptance.Engine, providerSource string) {
	t.Helper()
	if _, err := exec.LookPath(string(engine)); err != nil {
		t.Skipf("pinned %s executable is not installed", engine)
	}
	fake := acceptance.NewFakeHubSpot(hermeticToken, 666000668)
	server := httptest.NewServer(fake)
	t.Cleanup(server.Close)
	t.Setenv("HUBSPOT_ACCESS_TOKEN", hermeticToken)
	initial := hermeticProductConfig(server.URL, providerSource, "Annual support", "1200.00", "300.00", "P12M")

	acceptance.Run(t, acceptance.Options{
		Engine: engine, Shard: acceptance.ProductDefinitions,
		Prefix: "tf_acc_hermetic_products_", LedgerPath: t.TempDir() + "/cleanup.jsonl", ProbeBaseURL: server.URL,
	}, func(session *acceptance.Session) {
		session.Apply(initial)
		id := session.OpaqueStateString("hubspot_product.managed", "id")
		session.RequireStateString("hubspot_product.managed", "price", "1200.00")
		session.RequireStateString("hubspot_product.managed", "cost", "300.00")
		session.RequireEmptyPlan(initial)
		if patches := fake.ProductPatchCount(id); patches != 0 {
			t.Fatalf("initial Product patches = %d", patches)
		}

		semantic := hermeticProductConfig(server.URL, providerSource, "Annual support", "1200", "300", "P12M")
		session.Apply(semantic)
		session.RequireEmptyPlan(semantic)
		if patches := fake.ProductPatchCount(id); patches != 0 {
			t.Fatalf("semantic decimal update sent %d patches", patches)
		}

		if !fake.DriftProduct(id) {
			t.Fatal("could not inject Product drift")
		}
		session.RequirePlanDiffAttributes(semantic, "hubspot_product.managed", "cost", "description", "price", "recurring_billing_period")
		session.Apply(semantic)
		session.RequireEmptyPlan(semantic)
		if patches := fake.ProductPatchCount(id); patches != 1 {
			t.Fatalf("Product drift repair patches = %d, want 1", patches)
		}

		unmanaged := hermeticProductRequiredConfig(server.URL, providerSource)
		session.Apply(unmanaged)
		if !fake.DriftProductOptionals(id) {
			t.Fatal("could not inject unmanaged Product optional drift")
		}
		session.RequireEmptyPlan(unmanaged)
		if patches := fake.ProductPatchCount(id); patches != 1 {
			t.Fatalf("unmanaged optional drift sent %d patches", patches)
		}
		session.Apply(semantic)
		session.RequireEmptyPlan(semantic)
		if patches := fake.ProductPatchCount(id); patches != 2 {
			t.Fatalf("optional adoption repair patches = %d, want 2", patches)
		}

		fake.RejectNextProductPatch()
		rejected := hermeticProductConfig(server.URL, providerSource, "Rejected change", "1200", "300", "P12M")
		session.RequireApplyFailure(rejected)
		session.RequireEmptyPlan(semantic)
		if patches := fake.ProductPatchCount(id); patches != 2 {
			t.Fatalf("rejected Product update changed patch count: %d", patches)
		}

		fake.FailNextProductOperation(acceptance.ProductFaultReadRejected)
		session.RequirePlanFailure(semantic, "Product refresh failed")
		session.RequireStateString("hubspot_product.managed", "id", id)
		session.RequireEmptyPlan(semantic)

		session.RemoveState("hubspot_product.managed")
		for _, invalidID := range []string{"0", "01", "tf_acc_hermetic_product", "sku:tf_acc_hermetic_product"} {
			session.RequireImportFailure(semantic, "hubspot_product.managed", invalidID, "Invalid Product import ID")
		}
		session.Import("hubspot_product.managed", id)
		session.RequireStateString("hubspot_product.managed", "id", id)
		session.Apply(semantic)
		session.RequireEmptyPlan(semantic)

		if !fake.ArchiveProduct(id) {
			t.Fatal("could not archive Product out of band")
		}
		session.Refresh(semantic)
		session.RequireStateAbsent("hubspot_product.managed")
		session.RequireImportFailure(semantic, "hubspot_product.managed", id, "Archived Product cannot be imported")
		session.Apply(semantic)
		recreatedID := session.OpaqueStateString("hubspot_product.managed", "id")
		if recreatedID == id {
			t.Fatal("recreated Product reused an archived generated identity")
		}

		if !fake.RemoveProduct(recreatedID) {
			t.Fatal("could not simulate complete Product absence")
		}
		session.Refresh(semantic)
		session.RequireStateAbsent("hubspot_product.managed")
		session.Apply(semantic)
		terminalID := session.OpaqueStateString("hubspot_product.managed", "id")
		fake.FailNextProductOperation(acceptance.ProductFaultArchiveDisappears)
		session.RequireDestroyFailure(semantic)
		session.RequireStateString("hubspot_product.managed", "id", terminalID)
		if _, ok := fake.ProductSnapshot(terminalID); ok {
			t.Fatal("disappearing archive retained a fake Product identity")
		}
		session.Destroy(semantic)
		session.Apply(semantic)
		terminalID = session.OpaqueStateString("hubspot_product.managed", "id")

		fake.FailNextProductOperation(acceptance.ProductFaultArchiveRejected)
		session.RequireDestroyFailure(semantic)
		session.RequireStateString("hubspot_product.managed", "id", terminalID)
		if product, ok := fake.ProductSnapshot(terminalID); !ok || product.Archived {
			t.Fatalf("rejected archive mutated Product = %#v, %v", product, ok)
		}

		fake.FailNextProductOperation(acceptance.ProductFaultArchiveAmbiguous)
		session.Destroy(semantic)
		product, ok := fake.ProductSnapshot(terminalID)
		if !ok || !product.Archived || product.ID != terminalID {
			t.Fatalf("terminal Product = %#v, %v", product, ok)
		}
		if deletes := fake.ProductDeleteCount(terminalID); deletes != 1 {
			t.Fatalf("Product archive count = %d, want 1", deletes)
		}
	})
}

func hermeticProductRequiredConfig(baseURL, providerSource string) string {
	return fmt.Sprintf(`
terraform {
  required_providers {
    hubspot = {
      source = %q
    }
  }
}

provider "hubspot" {
  api_base_url = %q
}

resource "hubspot_product" "managed" {
  name        = "Northstar support"
  sku         = "tf_acc_hermetic_product"
  description = "Annual support"
  price       = "1200"
}
`, providerSource, baseURL)
}

func hermeticProductConfig(baseURL, providerSource, description, price, cost, recurrence string) string {
	return fmt.Sprintf(`
terraform {
  required_providers {
    hubspot = {
      source = %q
    }
  }
}

provider "hubspot" {
  api_base_url = %q
}

resource "hubspot_product" "managed" {
  name                     = "Northstar support"
  sku                      = "tf_acc_hermetic_product"
  description              = %q
  price                    = %q
  cost                     = %q
  recurring_billing_period = %q
}
`, providerSource, baseURL, description, price, cost, recurrence)
}
