// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

package acceptance_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/jackemcpherson/terraform-provider-hubspot/internal/acceptance"
)

func TestRunPreservesDealPipelineStageIdentityAcrossUpdates(t *testing.T) {
	if _, err := exec.LookPath("tofu"); err != nil {
		t.Skip("pinned OpenTofu executable is not installed")
	}

	api := newPipelineAPI(t)
	server := httptest.NewServer(api)
	t.Cleanup(server.Close)
	ledger := t.TempDir() + "/cleanup.jsonl"
	archiveRequestsBeforeAmbiguity := 0

	acceptance.Run(t, acceptance.Options{
		Engine:     acceptance.OpenTofu,
		Shard:      acceptance.DealPipelines,
		Prefix:     "tf_acc_harness_",
		LedgerPath: ledger,
	}, func(session *acceptance.Session) {
		initial := dealPipelineConfig(server.URL, "Initial pipeline", 10, "Open", "0.1")
		missingProbability := strings.Replace(initial, `metadata      = { probability = "0.1" }`, `metadata      = {}`, 1)
		session.RequirePlanFailure(missingProbability, "Invalid pipeline stage metadata")
		duplicateLabel := strings.Replace(initial, `label         = "Closed"`, `label         = "Open"`, 1)
		session.RequirePlanFailure(duplicateLabel, "Invalid pipeline stage metadata")
		session.Apply(initial)
		session.RequireStateString("hubspot_pipeline.test", "id", "deals/pipeline-1")
		session.RequireStateMapKey("hubspot_pipeline.test", "stages", "open", true)
		session.RequireStateMapKey("hubspot_pipeline.test", "stages", "closed", true)
		session.RequireEmptyPlan(initial)

		initialIDs := api.stageIDs()
		updated := dealPipelineConfig(server.URL, "Updated pipeline", 20, "Qualified", "0.2")
		updatesBeforeAmbiguity := api.updateRequestCount()
		api.setAmbiguousUpdateOnce()
		session.Apply(updated)
		if api.updateRequestCount() != updatesBeforeAmbiguity+1 {
			t.Fatal("ambiguous pipeline update was replayed")
		}
		session.RequireEmptyPlan(updated)
		if got := api.stageIDs(); strings.Join(got, ",") != strings.Join(initialIDs, ",") {
			t.Fatalf("stage IDs changed across in-place update: before %v, after %v", initialIDs, got)
		}

		withNurture := strings.Replace(updated, `    closed = {`, `    nurture = {
      label         = "Nurture"
      display_order = 15
      metadata      = { probability = "0.5" }
    }
    closed = {`, 1)
		session.Apply(withNurture)
		session.RequireStateMapKey("hubspot_pipeline.test", "stages", "nurture", true)
		if got := api.stageIDs(); len(got) != 3 || got[0] != initialIDs[0] || got[1] != initialIDs[1] {
			t.Fatalf("adding a stage changed existing remote identities")
		}
		session.RequireEmptyPlan(withNurture)

		withoutClosed := strings.Replace(withNurture, `    closed = {
      label         = "Closed"
      display_order = 20
      metadata      = { probability = "1.0" }
    }
`, "", 1)
		session.Apply(withoutClosed)
		if got := api.stageIDs(); len(got) != 2 || got[0] != initialIDs[1] {
			t.Fatalf("removing one stage changed an unrelated remote identity")
		}
		session.RequireEmptyPlan(withoutClosed)

		renamedKey := strings.Replace(withoutClosed, `    nurture = {`, `    follow_up = {`, 1)
		session.Apply(renamedKey)
		if !api.hasStage(initialIDs[1]) {
			t.Fatal("logical-key replacement changed an unrelated remote identity")
		}
		session.RequireStateMapKey("hubspot_pipeline.test", "stages", "follow_up", true)
		session.RequireEmptyPlan(renamedKey)

		remoteStageID := api.addOutOfBandStage("Remote stage", 30, "0.7")
		session.Refresh(renamedKey)
		session.RequireStateMapKey("hubspot_pipeline.test", "stages", remoteStageID, true)
		session.Apply(renamedKey)
		if api.hasStage(remoteStageID) {
			t.Fatal("authored reconciliation retained an unexpected remote stage")
		}
		session.RequireEmptyPlan(renamedKey)

		api.mutatePipeline("Out-of-band pipeline", 40)
		api.mutateStage(initialIDs[1], "Out-of-band stage", 50, "0.9")
		session.RequirePlanDiffAttributes(renamedKey, "hubspot_pipeline.test", "display_order", "label", "stages")
		session.Apply(renamedKey)
		session.RequireEmptyPlan(renamedKey)

		beforeArchive := api.stageIDs()
		api.archiveOutOfBand()
		session.Refresh(renamedKey)
		session.RequireStateString("hubspot_pipeline.test", "id", "deals/pipeline-1")
		session.RequirePlanDiffAttributes(renamedKey, "hubspot_pipeline.test", "id")
		restoresBeforeAmbiguity := api.restoreRequestCount()
		api.setAmbiguousRestoreOnce()
		session.Apply(renamedKey)
		if api.restoreRequestCount() != restoresBeforeAmbiguity+1 {
			t.Fatal("ambiguous pipeline restore was replayed")
		}
		if !api.isActive() || strings.Join(api.stageIDs(), ",") != strings.Join(beforeArchive, ",") {
			t.Fatal("pipeline restore did not preserve canonical pipeline and stage identity")
		}
		session.RequireEmptyPlan(renamedKey)

		imported := importedDealPipelineConfig(server.URL, "Updated pipeline", 20, api.stageSnapshot())
		session.RemoveState("hubspot_pipeline.test")
		session.Import("hubspot_pipeline.test", "deals/pipeline-1")
		for _, stageID := range api.stageIDs() {
			session.RequireStateMapKey("hubspot_pipeline.test", "stages", stageID, true)
		}
		session.RequireEmptyPlan(imported)

		api.archiveOutOfBand()
		session.RemoveState("hubspot_pipeline.test")
		session.Import("hubspot_pipeline.test", "deals/pipeline-1")
		session.RequirePlanDiffAttributes(imported, "hubspot_pipeline.test", "id")
		session.Apply(imported)
		if !api.isActive() {
			t.Fatal("archived pipeline import did not restore in place")
		}
		session.RequireEmptyPlan(imported)
		archiveRequestsBeforeAmbiguity = api.archiveRequestCount()
		api.setAmbiguousArchiveOnce()
	})
	if api.archiveRequestCount() != archiveRequestsBeforeAmbiguity+1 {
		t.Fatal("ambiguous pipeline archive was replayed")
	}
	if api.isActive() || !api.isArchived() {
		t.Fatal("acceptance cleanup did not verify the archived pipeline terminal state")
	}
}

func TestRunRetainsDealPipelineStateWhenReferencedDeletionIsBlocked(t *testing.T) {
	if _, err := exec.LookPath("tofu"); err != nil {
		t.Skip("pinned OpenTofu executable is not installed")
	}

	api := newPipelineAPI(t)
	server := httptest.NewServer(api)
	t.Cleanup(server.Close)
	ledger := t.TempDir() + "/cleanup.jsonl"

	acceptance.Run(t, acceptance.Options{
		Engine: acceptance.OpenTofu, Shard: acceptance.DealPipelines,
		Prefix: "tf_acc_harness_", LedgerPath: ledger,
	}, func(session *acceptance.Session) {
		config := dealPipelineConfig(server.URL, "Referenced pipeline", 10, "Open", "0.1")
		session.Apply(config)
		api.setReferencedStage(api.stageIDs()[0])
		withoutClosed := strings.Replace(config, `    closed = {
      label         = "Closed"
      display_order = 20
      metadata      = { probability = "1.0" }
    }
`, "", 1)
		session.RequireApplyFailureWithStatus(withoutClosed, acceptance.PipelineStageInUse)
		session.RequireStateMapKey("hubspot_pipeline.test", "stages", "closed", true)
		api.setReferencedStage("")
		api.setDeleteBlocked(true)
		session.RequireApplyFailureWithStatus(providerOnlyConfig(server.URL, "registry.opentofu.org/jackemcpherson/hubspot"), acceptance.PipelineStageInUse)
		session.RequireStateString("hubspot_pipeline.test", "id", "deals/pipeline-1")
		api.setDeleteBlocked(false)
	})
}

func TestRunRejectsProtectedDealPipelineImport(t *testing.T) {
	if _, err := exec.LookPath("tofu"); err != nil {
		t.Skip("pinned OpenTofu executable is not installed")
	}

	api := newPipelineAPI(t)
	server := httptest.NewServer(api)
	t.Cleanup(server.Close)
	ledger := t.TempDir() + "/cleanup.jsonl"
	acceptance.Run(t, acceptance.Options{
		Engine: acceptance.OpenTofu, Shard: acceptance.DealPipelines,
		Prefix: "tf_acc_harness_", LedgerPath: ledger,
	}, func(session *acceptance.Session) {
		config := dealPipelineConfig(server.URL, "Protected pipeline", 10, "Open", "0.1")
		session.RequireImportFailure(config, "hubspot_pipeline.test", "deals/default", "Pipeline contains protected stages")
	})
	if api.isActive() || api.isArchived() {
		t.Fatal("protected pipeline import mutated remote configuration")
	}
}

func TestRunDetectsDealPipelineAppendOrderDrift(t *testing.T) {
	if _, err := exec.LookPath("tofu"); err != nil {
		t.Skip("pinned OpenTofu executable is not installed")
	}
	api := newPipelineAPI(t)
	api.setCanonicalAppend(true)
	server := httptest.NewServer(api)
	t.Cleanup(server.Close)
	ledger := t.TempDir() + "/cleanup.jsonl"
	acceptance.Run(t, acceptance.Options{
		Engine: acceptance.OpenTofu, Shard: acceptance.DealPipelines,
		Prefix: "tf_acc_harness_", LedgerPath: ledger,
	}, func(session *acceptance.Session) {
		config := dealPipelineAppendConfig(server.URL)
		session.Apply(config)
		session.RequireEmptyPlan(config)
		api.setCompetingPipelineOrder(50)
		session.RequirePlanDiffAttributes(config, "hubspot_pipeline.test", "display_order")
		session.Apply(config)
		session.RequireEmptyPlan(config)
		api.addOutOfBandStage("Trailing remote stage", 100, "0.7")
		session.RequirePlanDiffAttributes(config, "hubspot_pipeline.test", "stages")
		session.Apply(config)
		session.RequireEmptyPlan(config)
	})
}

func dealPipelineConfig(apiBaseURL, label string, displayOrder int64, openLabel, probability string) string {
	return fmt.Sprintf(`
terraform {
  required_providers {
    hubspot = {
      source = "registry.opentofu.org/jackemcpherson/hubspot"
    }
  }
}

provider "hubspot" {
  access_token = "acceptance-sentinel"
  api_base_url = %q
}

resource "hubspot_pipeline" "test" {
  object_type   = "deals"
  label         = %q
  display_order = %d

  stages = {
    open = {
      label         = %q
      display_order = 10
      metadata      = { probability = %q }
    }
    closed = {
      label         = "Closed"
      display_order = 20
      metadata      = { probability = "1.0" }
    }
  }
}
`, apiBaseURL, label, displayOrder, openLabel, probability)
}

func importedDealPipelineConfig(apiBaseURL, label string, displayOrder int64, stages []pipelineAPIStage) string {
	var stageConfig strings.Builder
	for _, stage := range stages {
		fmt.Fprintf(&stageConfig, `    %q = {
      label         = %q
      display_order = %d
      metadata      = { probability = %q }
    }
`, stage.ID, stage.Label, stage.Order, stage.Metadata["probability"])
	}
	return fmt.Sprintf(`
terraform {
  required_providers {
    hubspot = {
      source = "registry.opentofu.org/jackemcpherson/hubspot"
    }
  }
}

provider "hubspot" {
  access_token = "acceptance-sentinel"
  api_base_url = %q
}

resource "hubspot_pipeline" "test" {
  object_type   = "deals"
  label         = %q
  display_order = %d

  stages = {
%s  }
}
`, apiBaseURL, label, displayOrder, stageConfig.String())
}

func dealPipelineAppendConfig(apiBaseURL string) string {
	config := dealPipelineConfig(apiBaseURL, "Append pipeline", 10, "Open", "0.1")
	config = strings.Replace(config, "  display_order = 10\n", "", 1)
	config = strings.Replace(config, "      display_order = 10\n", "", 1)
	config = strings.Replace(config, "      display_order = 20\n", "", 1)
	return config
}

type pipelineAPI struct {
	t                *testing.T
	mu               sync.Mutex
	active           bool
	archived         bool
	label            string
	order            int64
	stages           map[string]pipelineAPIStage
	nextStage        int
	blockDelete      bool
	referencedStage  string
	canonicalAppend  bool
	competingOrder   int64
	ambiguousUpdate  bool
	ambiguousRestore bool
	ambiguousArchive bool
	updateRequests   int
	restoreRequests  int
	archiveRequests  int
}

type pipelineAPIStage struct {
	ID       string
	Label    string
	Order    int64
	Metadata map[string]string
}

type pipelineWriteRequest struct {
	Label        string                      `json:"label"`
	DisplayOrder int64                       `json:"displayOrder"`
	Stages       []pipelineStageWriteRequest `json:"stages"`
}

type pipelineStageWriteRequest struct {
	StageID      string            `json:"stageId"`
	Label        string            `json:"label"`
	DisplayOrder int64             `json:"displayOrder"`
	Metadata     map[string]string `json:"metadata"`
}

func newPipelineAPI(t *testing.T) *pipelineAPI {
	t.Helper()
	return &pipelineAPI{t: t, stages: map[string]pipelineAPIStage{}, nextStage: 1}
}

func (a *pipelineAPI) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	a.mu.Lock()
	defer a.mu.Unlock()

	collection := "/crm/pipelines/2026-03/deals"
	item := collection + "/pipeline-1"
	stageCollection := item + "/stages"
	response.Header().Set("Content-Type", "application/json")

	switch {
	case request.Method == http.MethodGet && request.URL.Path == collection+"/default":
		_ = json.NewEncoder(response).Encode(map[string]any{
			"id": "default", "label": "Default", "displayOrder": 0, "archived": false,
			"stages": []map[string]any{{
				"id": "default-stage", "label": "Default stage", "displayOrder": 0,
				"metadata": map[string]string{"probability": "0.1"}, "writePermissions": "READ_ONLY",
			}},
		})
	case request.Method == http.MethodPost && request.URL.Path == collection:
		var body pipelineWriteRequest
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			a.fail(response, "decode pipeline create", err)
			return
		}
		a.label = body.Label
		a.order = a.canonicalPipelineOrder(body.DisplayOrder)
		a.active = true
		a.archived = false
		for _, stage := range body.Stages {
			a.createStage(stage)
		}
		response.WriteHeader(http.StatusCreated)
		a.writePipeline(response)
	case request.Method == http.MethodGet && request.URL.Path == collection:
		results := []map[string]any{}
		if a.active {
			results = append(results, a.pipelineDocument())
		}
		if a.competingOrder != 0 {
			results = append(results, map[string]any{
				"id": "unowned-pipeline", "label": "Unowned pipeline", "displayOrder": a.competingOrder,
				"stages": []any{}, "archived": false,
			})
		}
		_ = json.NewEncoder(response).Encode(map[string]any{"results": results})
	case request.Method == http.MethodGet && request.URL.Path == item:
		wantArchived := request.URL.Query().Get("archived") == "true"
		if (wantArchived && !a.archived) || (!wantArchived && !a.active) {
			response.WriteHeader(http.StatusNotFound)
			return
		}
		a.writePipeline(response)
	case request.Method == http.MethodPut && request.URL.Path == item:
		a.updateRequests++
		var body pipelineWriteRequest
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			a.fail(response, "decode pipeline replace", err)
			return
		}
		knownStageSeen := false
		replacement := make(map[string]pipelineAPIStage, len(body.Stages))
		for _, input := range body.Stages {
			if input.StageID == "" {
				stage := a.createStage(input)
				replacement[stage.ID] = stage
				continue
			}
			stage, ok := a.stages[input.StageID]
			if !ok {
				response.WriteHeader(http.StatusBadRequest)
				return
			}
			knownStageSeen = true
			stage.Label = input.Label
			stage.Order = input.DisplayOrder
			if a.canonicalAppend && input.DisplayOrder == -1 {
				stage.Order = int64(len(replacement)+1) * 10
			}
			stage.Metadata = input.Metadata
			replacement[stage.ID] = stage
		}
		if len(a.stages) != 0 && !knownStageSeen {
			response.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(response).Encode(map[string]any{
				"status": "error", "message": "replace omitted every known stage ID", "category": "VALIDATION_ERROR",
			})
			return
		}
		if a.referencedStage != "" {
			referencedRetained := false
			for _, input := range body.Stages {
				if input.StageID == a.referencedStage {
					referencedRetained = true
					break
				}
			}
			if !referencedRetained {
				response.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(response).Encode(map[string]any{
					"status": "error", "message": "pipeline stage is referenced",
					"category": "VALIDATION_ERROR", "subCategory": "PipelineError.STAGE_ID_IN_USE",
				})
				return
			}
		}
		if len(replacement) != len(body.Stages) {
			response.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(response).Encode(map[string]any{
				"status": "error", "message": "replace reused a stage ID", "category": "DUPLICATE_STAGE_ID",
			})
			return
		}
		a.label = body.Label
		a.order = a.canonicalPipelineOrder(body.DisplayOrder)
		a.stages = replacement
		if a.ambiguousUpdate {
			a.ambiguousUpdate = false
			response.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(response).Encode(map[string]any{"status": "error", "message": "response lost", "category": "INTERNAL_ERROR"})
			return
		}
		a.writePipeline(response)
	case request.Method == http.MethodPatch && request.URL.Path == item:
		a.restoreRequests++
		var body struct {
			Label        *string `json:"label"`
			DisplayOrder *int64  `json:"displayOrder"`
			Archived     *bool   `json:"archived"`
		}
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			a.fail(response, "decode pipeline patch", err)
			return
		}
		if body.Archived != nil && !*body.Archived {
			a.active = true
			a.archived = false
		}
		if body.Label != nil {
			a.label = *body.Label
		}
		if body.DisplayOrder != nil {
			a.order = *body.DisplayOrder
		}
		if a.ambiguousRestore {
			a.ambiguousRestore = false
			response.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(response).Encode(map[string]any{"status": "error", "message": "response lost", "category": "INTERNAL_ERROR"})
			return
		}
		a.writePipeline(response)
	case request.Method == http.MethodPost && request.URL.Path == stageCollection:
		var body pipelineStageWriteRequest
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			a.fail(response, "decode stage create", err)
			return
		}
		stage := a.createStage(body)
		response.WriteHeader(http.StatusCreated)
		a.writeStage(response, stage)
	case request.Method == http.MethodPut && strings.HasPrefix(request.URL.Path, stageCollection+"/"):
		stageID := strings.TrimPrefix(request.URL.Path, stageCollection+"/")
		stage, ok := a.stages[stageID]
		if !ok {
			response.WriteHeader(http.StatusNotFound)
			return
		}
		var body pipelineStageWriteRequest
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			a.fail(response, "decode stage update", err)
			return
		}
		stage.Label = body.Label
		stage.Order = body.DisplayOrder
		stage.Metadata = body.Metadata
		a.stages[stageID] = stage
		a.writeStage(response, stage)
	case request.Method == http.MethodDelete && strings.HasPrefix(request.URL.Path, stageCollection+"/"):
		stageID := strings.TrimPrefix(request.URL.Path, stageCollection+"/")
		delete(a.stages, stageID)
		response.WriteHeader(http.StatusNoContent)
	case request.Method == http.MethodDelete && request.URL.Path == item:
		a.archiveRequests++
		if a.blockDelete {
			response.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(response).Encode(map[string]any{
				"status": "error", "message": "pipeline stages are referenced",
				"category": "VALIDATION_ERROR", "subCategory": "PipelineError.STAGE_ID_IN_USE",
			})
			return
		}
		a.active = false
		a.archived = true
		if a.ambiguousArchive {
			a.ambiguousArchive = false
			response.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(response).Encode(map[string]any{"status": "error", "message": "response lost", "category": "INTERNAL_ERROR"})
			return
		}
		response.WriteHeader(http.StatusNoContent)
	default:
		a.t.Errorf("unexpected pipeline request: %s %s", request.Method, request.URL.RequestURI())
		response.WriteHeader(http.StatusNotFound)
	}
}

func (a *pipelineAPI) createStage(body pipelineStageWriteRequest) pipelineAPIStage {
	id := fmt.Sprintf("stage-%d", a.nextStage)
	a.nextStage++
	order := body.DisplayOrder
	if a.canonicalAppend && order == -1 {
		order = int64(len(a.stages)+1) * 10
	}
	stage := pipelineAPIStage{ID: id, Label: body.Label, Order: order, Metadata: body.Metadata}
	a.stages[id] = stage
	return stage
}

func (a *pipelineAPI) writePipeline(response http.ResponseWriter) {
	if err := json.NewEncoder(response).Encode(a.pipelineDocument()); err != nil {
		a.t.Errorf("encode pipeline response: %v", err)
	}
}

func (a *pipelineAPI) pipelineDocument() map[string]any {
	stages := make([]map[string]any, 0, len(a.stages))
	for _, id := range a.stageIDsLocked() {
		stage := a.stages[id]
		stages = append(stages, a.stageDocument(stage))
	}
	return map[string]any{
		"id": "pipeline-1", "label": a.label, "displayOrder": a.order,
		"stages": stages, "archived": a.archived,
	}
}

func (a *pipelineAPI) writeStage(response http.ResponseWriter, stage pipelineAPIStage) {
	if err := json.NewEncoder(response).Encode(a.stageDocument(stage)); err != nil {
		a.t.Errorf("encode pipeline stage response: %v", err)
	}
}

func (a *pipelineAPI) stageDocument(stage pipelineAPIStage) map[string]any {
	return map[string]any{
		"id": stage.ID, "label": stage.Label, "displayOrder": stage.Order,
		"metadata": stage.Metadata, "writePermissions": "CRM_PERMISSIONS_ENFORCEMENT",
	}
}

func (a *pipelineAPI) fail(response http.ResponseWriter, operation string, err error) {
	a.t.Errorf("%s: %v", operation, err)
	response.WriteHeader(http.StatusBadRequest)
}

func (a *pipelineAPI) stageIDs() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.stageIDsLocked()
}

func (a *pipelineAPI) stageIDsLocked() []string {
	ids := make([]string, 0, len(a.stages))
	for id := range a.stages {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func (a *pipelineAPI) hasStage(id string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	_, ok := a.stages[id]
	return ok
}

func (a *pipelineAPI) addOutOfBandStage(label string, order int64, probability string) string {
	a.mu.Lock()
	defer a.mu.Unlock()
	stage := a.createStage(pipelineStageWriteRequest{
		Label: label, DisplayOrder: order, Metadata: map[string]string{"probability": probability},
	})
	return stage.ID
}

func (a *pipelineAPI) mutatePipeline(label string, order int64) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.label = label
	a.order = order
}

func (a *pipelineAPI) mutateStage(id, label string, order int64, probability string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	stage := a.stages[id]
	stage.Label = label
	stage.Order = order
	stage.Metadata = map[string]string{"probability": probability}
	a.stages[id] = stage
}

func (a *pipelineAPI) archiveOutOfBand() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.active = false
	a.archived = true
}

func (a *pipelineAPI) isActive() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.active
}

func (a *pipelineAPI) isArchived() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.archived
}

func (a *pipelineAPI) stageSnapshot() []pipelineAPIStage {
	a.mu.Lock()
	defer a.mu.Unlock()
	stages := make([]pipelineAPIStage, 0, len(a.stages))
	for _, id := range a.stageIDsLocked() {
		stage := a.stages[id]
		metadata := make(map[string]string, len(stage.Metadata))
		for key, value := range stage.Metadata {
			metadata[key] = value
		}
		stage.Metadata = metadata
		stages = append(stages, stage)
	}
	return stages
}

func (a *pipelineAPI) setDeleteBlocked(blocked bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.blockDelete = blocked
}

func (a *pipelineAPI) setReferencedStage(id string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.referencedStage = id
}

func (a *pipelineAPI) setCanonicalAppend(enabled bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.canonicalAppend = enabled
}

func (a *pipelineAPI) setCompetingPipelineOrder(order int64) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.competingOrder = order
}

func (a *pipelineAPI) setAmbiguousUpdateOnce() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.ambiguousUpdate = true
}

func (a *pipelineAPI) setAmbiguousRestoreOnce() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.ambiguousRestore = true
}

func (a *pipelineAPI) setAmbiguousArchiveOnce() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.ambiguousArchive = true
}

func (a *pipelineAPI) updateRequestCount() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.updateRequests
}

func (a *pipelineAPI) archiveRequestCount() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.archiveRequests
}

func (a *pipelineAPI) restoreRequestCount() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.restoreRequests
}

func (a *pipelineAPI) canonicalPipelineOrder(requested int64) int64 {
	if !a.canonicalAppend || requested != -1 {
		return requested
	}
	if a.competingOrder != 0 {
		return a.competingOrder + 10
	}
	return 10
}
