// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

package acceptance_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"sync"
	"testing"

	"github.com/jackemcpherson/terraform-provider-hubspot/internal/acceptance"
)

func TestRunExecutesPropertyGroupLifecycleThroughOpenTofu(t *testing.T) {
	runPropertyGroupLifecycle(t, acceptance.OpenTofu, "registry.opentofu.org/jackemcpherson/hubspot")
}

func TestRunExecutesPropertyGroupLifecycleThroughTerraform(t *testing.T) {
	runPropertyGroupLifecycle(t, acceptance.Terraform, "registry.terraform.io/jackemcpherson/hubspot")
}

func TestRunRetainsStateWhenPropertyGroupDestroyIsBlocked(t *testing.T) {
	if _, err := exec.LookPath("tofu"); err != nil {
		t.Skip("pinned OpenTofu executable is not installed")
	}
	api := newPropertyGroupAPI(t)
	server := httptest.NewServer(api)
	t.Cleanup(server.Close)
	ledger := t.TempDir() + "/cleanup.jsonl"

	acceptance.Run(t, acceptance.Options{
		Engine:     acceptance.OpenTofu,
		Shard:      acceptance.FreeProperties,
		Prefix:     "tf_acc_harness_",
		LedgerPath: ledger,
	}, func(session *acceptance.Session) {
		config := propertyGroupConfig(server.URL, "registry.opentofu.org/jackemcpherson/hubspot", "Blocked group", -1)
		session.Apply(config)
		api.setDeleteBlocked(true)
		session.RequireApplyFailureWithStatus(providerOnlyConfig(server.URL, "registry.opentofu.org/jackemcpherson/hubspot"), acceptance.PropertyGroupHasActiveProperties)
		session.RequireStateString("hubspot_property_group.test", "label", "Blocked group")
		api.setDeleteBlocked(false)
	})
}

func TestRunDetectsPropertyGroupAppendOrderDrift(t *testing.T) {
	if _, err := exec.LookPath("tofu"); err != nil {
		t.Skip("pinned OpenTofu executable is not installed")
	}
	api := newPropertyGroupAPI(t)
	server := httptest.NewServer(api)
	t.Cleanup(server.Close)
	ledger := t.TempDir() + "/cleanup.jsonl"
	acceptance.Run(t, acceptance.Options{
		Engine: acceptance.OpenTofu, Shard: acceptance.FreeProperties,
		Prefix: "tf_acc_harness_", LedgerPath: ledger,
	}, func(session *acceptance.Session) {
		config := propertyGroupConfig(server.URL, "registry.opentofu.org/jackemcpherson/hubspot", "Append group", -1)
		session.Apply(config)
		session.RequireEmptyPlan(config)
		api.setCompetingOrder(20)
		session.RequirePlanDiffAttributes(config, "hubspot_property_group.test", "display_order")
		session.Apply(config)
		session.RequireEmptyPlan(config)
	})
}

func runPropertyGroupLifecycle(t *testing.T, engine acceptance.Engine, providerSource string) {
	t.Helper()
	if _, err := exec.LookPath(string(engine)); err != nil {
		t.Skipf("pinned %s executable is not installed", engine)
	}

	api := newPropertyGroupAPI(t)
	server := httptest.NewServer(api)
	t.Cleanup(server.Close)
	t.Setenv("HUBSPOT_ACCESS_TOKEN", "acceptance-sentinel")

	ledger := t.TempDir() + "/cleanup.jsonl"
	acceptance.Run(t, acceptance.Options{
		Engine:       engine,
		Shard:        acceptance.FreeProperties,
		Prefix:       "tf_acc_harness_",
		LedgerPath:   ledger,
		ProbeBaseURL: server.URL,
	}, func(session *acceptance.Session) {
		initial := propertyGroupConfig(server.URL, providerSource, "Initial label", 10)
		session.Apply(initial)
		session.RequireStateString("hubspot_property_group.test", "label", "Initial label")
		session.RequireEmptyPlan(initial)

		updated := propertyGroupConfig(server.URL, providerSource, "Updated label", 20)
		session.Apply(updated)
		driftOrder := int64(30)
		session.MutatePropertyGroup("contacts", "tf_acc_harness_group", "Out-of-band label", &driftOrder)
		session.RequirePlanDiffAttributes(updated, "hubspot_property_group.test", "display_order", "label")
		session.Apply(updated)
		session.RequireEmptyPlan(updated)
		session.RemoveState("hubspot_property_group.test")
		session.Import("hubspot_property_group.test", "contacts/tf_acc_harness_group")
		session.RequireEmptyPlan(updated)
		session.ArchivePropertyGroup("contacts", "tf_acc_harness_group")
		session.Refresh(updated)
		session.RequireStateAbsent("hubspot_property_group.test")
		session.Apply(updated)
		session.RequireEmptyPlan(updated)
	})

	if api.isActive() {
		t.Fatal("acceptance cleanup left the property group active")
	}
}

func propertyGroupConfig(apiBaseURL, providerSource, label string, displayOrder int64) string {
	return fmt.Sprintf(`
terraform {
  required_providers {
    hubspot = {
      source = %q
    }
  }
}

provider "hubspot" {
  access_token = "acceptance-sentinel"
  api_base_url = %q
}

resource "hubspot_property_group" "test" {
  object_type = "contacts"
  name        = "tf_acc_harness_group"
  label       = %q
  display_order = %d
}
`, providerSource, apiBaseURL, label, displayOrder)
}

func providerOnlyConfig(apiBaseURL, providerSource string) string {
	return fmt.Sprintf(`
terraform {
  required_providers {
    hubspot = {
      source = %q
    }
  }
}

provider "hubspot" {
  access_token = "acceptance-sentinel"
  api_base_url = %q
}
`, providerSource, apiBaseURL)
}

type propertyGroupAPI struct {
	t              *testing.T
	mu             sync.Mutex
	active         bool
	label          string
	displayOrder   int64
	blockDelete    bool
	competingOrder int64
}

func newPropertyGroupAPI(t *testing.T) *propertyGroupAPI {
	t.Helper()
	return &propertyGroupAPI{t: t}
}

func (a *propertyGroupAPI) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	a.mu.Lock()
	defer a.mu.Unlock()

	collection := "/crm/properties/2026-03/contacts/groups"
	item := collection + "/tf_acc_harness_group"
	response.Header().Set("Content-Type", "application/json")

	switch {
	case request.Method == http.MethodPost && request.URL.Path == collection:
		var body struct {
			Label        string `json:"label"`
			DisplayOrder int64  `json:"displayOrder"`
		}
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			a.t.Errorf("decode create request: %v", err)
			response.WriteHeader(http.StatusBadRequest)
			return
		}
		a.active = true
		a.label = body.Label
		a.displayOrder = a.canonicalOrder(body.DisplayOrder)
		response.WriteHeader(http.StatusCreated)
		a.writeGroupWithLabel(response, "Noncanonical mutation response")
	case request.Method == http.MethodGet && request.URL.Path == item:
		if !a.active {
			response.WriteHeader(http.StatusNotFound)
			return
		}
		a.writeGroup(response)
	case request.Method == http.MethodGet && request.URL.Path == collection:
		results := []any{}
		if a.active {
			results = append(results, map[string]any{
				"name": "tf_acc_harness_group", "label": a.label,
				"displayOrder": a.displayOrder, "archived": false,
			})
		}
		if a.competingOrder != 0 {
			results = append(results, map[string]any{
				"name": "unowned_competing_group", "label": "Unowned competing group",
				"displayOrder": a.competingOrder, "archived": false,
			})
		}
		if err := json.NewEncoder(response).Encode(map[string]any{"results": results}); err != nil {
			a.t.Errorf("encode group collection response: %v", err)
		}
	case request.Method == http.MethodPatch && request.URL.Path == item:
		var body struct {
			Label        string `json:"label"`
			DisplayOrder int64  `json:"displayOrder"`
		}
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			a.t.Errorf("decode update request: %v", err)
			response.WriteHeader(http.StatusBadRequest)
			return
		}
		a.label = body.Label
		a.displayOrder = a.canonicalOrder(body.DisplayOrder)
		a.writeGroup(response)
	case request.Method == http.MethodDelete && request.URL.Path == item:
		if a.blockDelete {
			response.WriteHeader(http.StatusBadRequest)
			if err := json.NewEncoder(response).Encode(map[string]any{
				"status":  "error",
				"message": `{"status":"error","message":"Can't delete or purge a group with active properties","category":"VALIDATION_ERROR","subCategory":"PropertyGroupError.GROUP_WITH_ACTIVE_PROPERTIES"}`,
			}); err != nil {
				a.t.Errorf("encode blocked delete response: %v", err)
			}
			return
		}
		a.active = false
		response.WriteHeader(http.StatusNoContent)
	default:
		a.t.Errorf("unexpected request: %s %s", request.Method, request.URL.RequestURI())
		response.WriteHeader(http.StatusNotFound)
	}
}

func (a *propertyGroupAPI) writeGroup(response http.ResponseWriter) {
	a.writeGroupWithLabel(response, a.label)
}

func (a *propertyGroupAPI) writeGroupWithLabel(response http.ResponseWriter, label string) {
	if err := json.NewEncoder(response).Encode(map[string]any{
		"name":         "tf_acc_harness_group",
		"label":        label,
		"displayOrder": a.displayOrder,
		"archived":     false,
	}); err != nil {
		a.t.Errorf("encode response: %v", err)
	}
}

func (a *propertyGroupAPI) isActive() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.active
}

func (a *propertyGroupAPI) setDeleteBlocked(blocked bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.blockDelete = blocked
}

func (a *propertyGroupAPI) setCompetingOrder(order int64) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.competingOrder = order
}

func (a *propertyGroupAPI) canonicalOrder(requested int64) int64 {
	if requested != -1 {
		return requested
	}
	if a.competingOrder != 0 {
		return a.competingOrder + 10
	}
	return 10
}
