// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

//go:build acceptance

package acceptance_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jackemcpherson/terraform-provider-hubspot/internal/acceptance"
)

const liveFormAddress = "hubspot_form_definition.test"

var exactCandidateCommit = regexp.MustCompile(`^[0-9a-f]{40}$`)

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

	if t.Failed() {
		return
	}
	writeFormsAcceptanceEvidence(t, engine, finalID, []string{initialID, finalID})
}

func TestFormsAcceptanceEvidenceIsSanitized(t *testing.T) {
	t.Setenv("HUBSPOT_ACCEPTANCE_EVIDENCE_DIR", t.TempDir())
	t.Setenv("HUBSPOT_ACCEPTANCE_CANDIDATE_COMMIT", strings.Repeat("a", 40))
	t.Setenv("HUBSPOT_ACCEPTANCE_PORTAL_ID", "12345678")
	initialID := "00000000-0000-4000-8000-000000000001"
	finalID := "00000000-0000-4000-8000-000000000002"
	writeFormsAcceptanceEvidence(t, acceptance.OpenTofu, finalID, []string{initialID, finalID})
	contents, err := os.ReadFile(filepath.Join(os.Getenv("HUBSPOT_ACCEPTANCE_EVIDENCE_DIR"), "form_definitions-tofu.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"12345678", initialID, finalID, "tf_acc_"} {
		if bytes.Contains(contents, []byte(forbidden)) {
			t.Fatal("Forms acceptance evidence exposed raw portal configuration")
		}
	}
	var evidence formsAcceptanceEvidence
	if err := json.Unmarshal(contents, &evidence); err != nil {
		t.Fatal(err)
	}
	if evidence.Engine != "tofu" || evidence.APIFamily != "marketing/v3/forms" || evidence.ScopeFamily != "forms" || evidence.Cleanup != "passed" || len(evidence.TerminalIdentityHashes) != 2 {
		t.Fatalf("Forms acceptance evidence did not preserve the sanitized contract: %#v", evidence)
	}
}

type formsAcceptanceEvidence struct {
	CandidateCommit        string   `json:"candidate_commit"`
	Engine                 string   `json:"engine"`
	APIFamily              string   `json:"api_family"`
	ScopeFamily            string   `json:"scope_family"`
	PortalFingerprint      string   `json:"portal_fingerprint"`
	GeneratedIdentityHash  string   `json:"generated_identity_hash"`
	TerminalIdentityHashes []string `json:"terminal_identity_hashes"`
	Timestamp              string   `json:"timestamp"`
	Cleanup                string   `json:"cleanup"`
}

func writeFormsAcceptanceEvidence(t *testing.T, engine acceptance.Engine, finalID string, terminalIDs []string) {
	t.Helper()
	directory := requiredEnvironment(t, "HUBSPOT_ACCEPTANCE_EVIDENCE_DIR")
	commit := requiredEnvironment(t, "HUBSPOT_ACCEPTANCE_CANDIDATE_COMMIT")
	if !exactCandidateCommit.MatchString(commit) {
		t.Fatal("Forms acceptance evidence requires an exact candidate commit")
	}
	portalID := requiredEnvironment(t, "HUBSPOT_ACCEPTANCE_PORTAL_ID")
	terminalHashes := make([]string, 0, len(terminalIDs))
	for _, id := range terminalIDs {
		terminalHashes = append(terminalHashes, evidenceHash("hubspot-form-identity", id))
	}
	sort.Strings(terminalHashes)
	evidence := formsAcceptanceEvidence{
		CandidateCommit:        commit,
		Engine:                 string(engine),
		APIFamily:              "marketing/v3/forms",
		ScopeFamily:            "forms",
		PortalFingerprint:      evidenceHash("hubspot-portal", portalID),
		GeneratedIdentityHash:  evidenceHash("hubspot-form-identity", finalID),
		TerminalIdentityHashes: terminalHashes,
		Timestamp:              time.Now().UTC().Format(time.RFC3339),
		Cleanup:                "passed",
	}
	contents, err := json.Marshal(evidence)
	if err != nil {
		t.Fatalf("encode sanitized Forms acceptance evidence: %v", err)
	}
	for _, forbidden := range append([]string{portalID}, terminalIDs...) {
		if bytes.Contains(contents, []byte(forbidden)) {
			t.Fatal("Forms acceptance evidence contained a raw portal or form identity")
		}
	}
	path := filepath.Join(directory, "form_definitions-"+string(engine)+".json")
	if err := os.WriteFile(path, append(contents, '\n'), 0o600); err != nil {
		t.Fatalf("write sanitized Forms acceptance evidence: %v", err)
	}
}

func evidenceHash(domain, value string) string {
	digest := sha256.Sum256([]byte(domain + "\x00" + value))
	return hex.EncodeToString(digest[:])
}

func liveFormDefinitionConfig(providerSource, name string, updated bool) string {
	return formDefinitionConfig(providerSource, "", name, updated)
}
