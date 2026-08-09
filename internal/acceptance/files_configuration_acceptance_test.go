// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

//go:build acceptance

package acceptance_test

import (
	"bytes"
	"encoding/json"
	"fmt"
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

const (
	liveFileFolderRootAddress  = "hubspot_file_folder.root"
	liveFileFolderChildAddress = "hubspot_file_folder.child"
	liveManagedFileAddress     = "hubspot_file.managed"
)

var generatedFilesID = regexp.MustCompile(`^[1-9][0-9]*$`)

func TestFilesAcceptanceConfigurationSyntax(t *testing.T) {
	config := liveFilesConfiguration("registry.opentofu.org/jackemcpherson/hubspot", "/tmp/managed.txt", strings.Repeat("a", 64), "tf_acc_syntax_files_", filesConfigInitial)
	path := filepath.Join(t.TempDir(), "main.tf")
	if err := os.WriteFile(path, []byte(config), 0o600); err != nil {
		t.Fatalf("write Files acceptance syntax fixture: %v", err)
	}
	if output, err := exec.Command("tofu", "fmt", path).CombinedOutput(); err != nil {
		t.Fatalf("Files acceptance syntax fixture is invalid: %v: %s", err, strings.TrimSpace(string(output)))
	}
}

func TestAcc_files_configuration_OpenTofuLifecycle(t *testing.T) {
	runLiveFilesConfigurationLifecycle(t, acceptance.OpenTofu, "registry.opentofu.org/jackemcpherson/hubspot")
}

func TestAcc_files_configuration_TerraformLifecycle(t *testing.T) {
	runLiveFilesConfigurationLifecycle(t, acceptance.Terraform, "registry.terraform.io/jackemcpherson/hubspot")
}

func runLiveFilesConfigurationLifecycle(t *testing.T, engine acceptance.Engine, providerSource string) {
	t.Helper()
	requireAcceptanceEnabled(t)
	startedAt := time.Now().UTC()
	prefix := requiredEnvironment(t, "HUBSPOT_ACCEPTANCE_PREFIX") + string(engine) + "_"
	ledger := requiredEnvironment(t, "HUBSPOT_ACCEPTANCE_CLEANUP_LEDGER")
	directory := t.TempDir()
	sourcePath := filepath.Join(directory, "managed.txt")
	initialBytes := []byte("Files candidate content v1\n")
	updatedBytes := []byte("Files candidate content v2\n")
	if err := os.WriteFile(sourcePath, initialBytes, 0o600); err != nil {
		t.Fatalf("write initial Files acceptance source: %v", err)
	}
	initialDigest := sourceDigest(initialBytes)
	updatedDigest := sourceDigest(updatedBytes)
	initial := liveFilesConfiguration(providerSource, sourcePath, initialDigest, prefix, filesConfigInitial)
	folderCollision := liveFilesConfiguration(providerSource, sourcePath, initialDigest, prefix, filesConfigFolderCollision)
	fileCollision := liveFilesConfiguration(providerSource, sourcePath, initialDigest, prefix, filesConfigFileCollision)
	var rootID, childID, initialFileID, finalFileID, reusedRootID, reusedChildID, reusedFileID string

	acceptance.Run(t, acceptance.Options{
		Engine: engine, Shard: acceptance.FilesConfiguration, Prefix: prefix, LedgerPath: ledger,
	}, func(session *acceptance.Session) {
		session.Apply(initial)
		rootID = session.OpaqueStateString(liveFileFolderRootAddress, "id")
		childID = session.OpaqueStateString(liveFileFolderChildAddress, "id")
		initialFileID = session.OpaqueStateString(liveManagedFileAddress, "id")
		for _, id := range []string{rootID, childID, initialFileID} {
			if !generatedFilesID.MatchString(id) {
				t.Fatal("Files acceptance received a non-canonical generated identity")
			}
		}
		session.RequireStateString(liveFileFolderRootAddress, "name", prefix+"root")
		session.RequireStateString(liveManagedFileAddress, "access", "PRIVATE")
		session.RequireEmptyPlan(initial)
		session.RequireManagedFileDuplicateRejected(liveManagedFileAddress, initialBytes)

		session.RequireApplyFailure(folderCollision)
		session.RequireEmptyPlan(initial)
		session.RequireApplyFailure(fileCollision)
		session.RequireEmptyPlan(initial)

		if err := os.WriteFile(sourcePath, updatedBytes, 0o600); err != nil {
			t.Fatalf("write updated Files acceptance source: %v", err)
		}
		renamed := liveFilesConfiguration(providerSource, sourcePath, updatedDigest, prefix, filesConfigRenamed)
		session.Apply(renamed)
		requireFilesIdentitiesUnchanged(t, session, rootID, childID, initialFileID)
		session.RequireStateString(liveManagedFileAddress, "access", "PUBLIC_NOT_INDEXABLE")
		session.RequireStateString(liveFileFolderChildAddress, "path", "/"+prefix+"root_updated/"+prefix+"child_updated")
		session.RequireStateString(liveManagedFileAddress, "path", "/"+prefix+"root_updated/"+prefix+"child_updated/"+prefix+"managed_updated.txt")
		session.RequireEmptyPlan(renamed)

		moved := liveFilesConfiguration(providerSource, sourcePath, updatedDigest, prefix, filesConfigMoved)
		session.Apply(moved)
		requireFilesIdentitiesUnchanged(t, session, rootID, childID, initialFileID)

		private := liveFilesConfiguration(providerSource, sourcePath, updatedDigest, prefix, filesConfigPrivate)
		session.Apply(private)
		session.RequireStateString(liveManagedFileAddress, "access", "PRIVATE")
		session.RequireStateString(liveFileFolderChildAddress, "path", "/"+prefix+"child_updated")
		session.RequireStateString(liveManagedFileAddress, "path", "/"+prefix+"child_updated/"+prefix+"managed_updated.txt")
		session.RequireEmptyPlan(private)

		session.MutateManagedFileContent(liveManagedFileAddress, []byte("out-of-band content drift\n"))
		session.RequirePlanDiff(private)
		session.Apply(private)
		session.RequireEmptyPlan(private)

		session.RemoveState(liveFileFolderChildAddress)
		session.Import(liveFileFolderChildAddress, childID)
		session.RequireEmptyPlan(private)
		session.RemoveState(liveManagedFileAddress)
		session.Import(liveManagedFileAddress, initialFileID)
		session.RequirePlanDiff(private)
		session.Apply(private)
		session.RequireEmptyPlan(private)

		if disappearedID := session.DeleteManagedFileOutOfBand(liveManagedFileAddress); disappearedID != initialFileID {
			t.Fatal("external Files disappearance did not preserve the exact generated identity")
		}
		session.Refresh(private)
		session.RequireStateAbsent(liveManagedFileAddress)
		session.Apply(private)
		finalFileID = session.OpaqueStateString(liveManagedFileAddress, "id")
		if finalFileID == initialFileID {
			t.Fatal("external Files disappearance recreation reused the generated identity")
		}
		session.RequireEmptyPlan(private)

		session.Destroy(private)
		files, folders := session.RequireFilesTerminal(prefix, []string{initialFileID, finalFileID}, []string{rootID, childID})
		if files != 0 || folders != 0 {
			t.Fatal("Files destroy retained active owned configuration")
		}
		session.Apply(private)
		reusedRootID = session.OpaqueStateString(liveFileFolderRootAddress, "id")
		reusedChildID = session.OpaqueStateString(liveFileFolderChildAddress, "id")
		reusedFileID = session.OpaqueStateString(liveManagedFileAddress, "id")
		if reusedRootID == rootID || reusedChildID == childID || reusedFileID == initialFileID || reusedFileID == finalFileID {
			t.Fatal("immediate Files name reuse did not allocate new generated identities")
		}
		session.Destroy(private)
		session.RequireFilesTerminal(prefix, []string{reusedFileID}, []string{reusedRootID, reusedChildID})
	})

	if t.Failed() {
		return
	}
	writeFilesAcceptanceEvidence(t, engine, []string{rootID, childID, initialFileID, finalFileID, reusedRootID, reusedChildID, reusedFileID}, initialDigest, updatedDigest, startedAt, time.Now().UTC())
}

func requireFilesIdentitiesUnchanged(t *testing.T, session *acceptance.Session, rootID, childID, fileID string) {
	t.Helper()
	if session.OpaqueStateString(liveFileFolderRootAddress, "id") != rootID || session.OpaqueStateString(liveFileFolderChildAddress, "id") != childID || session.OpaqueStateString(liveManagedFileAddress, "id") != fileID {
		t.Fatal("an in-place Files update changed a generated identity")
	}
}

type filesConfigStage int

const (
	filesConfigInitial filesConfigStage = iota
	filesConfigFolderCollision
	filesConfigFileCollision
	filesConfigRenamed
	filesConfigMoved
	filesConfigPrivate
)

func liveFilesConfiguration(providerSource, sourcePath, digest, prefix string, stage filesConfigStage) string {
	rootName := prefix + "root"
	childName := prefix + "child"
	fileName := prefix + "managed.txt"
	access := "PRIVATE"
	parent := "  parent_folder_id = hubspot_file_folder.root.id\n"
	folderCollision := ""
	fileCollision := ""
	if stage >= filesConfigRenamed {
		rootName = prefix + "root_updated"
		childName = prefix + "child_updated"
		fileName = prefix + "managed_updated.txt"
		access = "PUBLIC_NOT_INDEXABLE"
	}
	if stage >= filesConfigMoved {
		parent = ""
	}
	if stage == filesConfigPrivate {
		access = "PRIVATE"
	}
	if stage == filesConfigFolderCollision {
		folderCollision = fmt.Sprintf(`
resource "hubspot_file_folder" "collision" {
  name = %q
}
`, rootName)
	}
	if stage == filesConfigFileCollision {
		fileCollision = fmt.Sprintf(`
resource "hubspot_file" "collision" {
  name          = %q
  folder_id     = hubspot_file_folder.child.id
  source_path   = %q
  source_sha256 = %q
  access        = %q
}
`, fileName, sourcePath, digest, access)
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

resource "hubspot_file_folder" "root" {
  name = %q
}

resource "hubspot_file_folder" "child" {
  name = %q
%s}

resource "hubspot_file" "managed" {
  name          = %q
  folder_id     = hubspot_file_folder.child.id
  source_path   = %q
  source_sha256 = %q
  access        = %q
}
%s%s`, providerSource, rootName, childName, parent, fileName, sourcePath, digest, access, folderCollision, fileCollision)
}

type filesContentChangeProof struct {
	BeforeSHA256 string `json:"before_sha256"`
	AfterSHA256  string `json:"after_sha256"`
}

type filesActiveCleanupCounts struct {
	Files   int `json:"files"`
	Folders int `json:"folders"`
}

type filesAcceptanceEvidence struct {
	CandidateCommit         string                   `json:"candidate_commit"`
	Engine                  string                   `json:"engine"`
	APIFamily               string                   `json:"api_family"`
	ScopeFamily             string                   `json:"scope_family"`
	PortalFingerprint       string                   `json:"portal_fingerprint"`
	GeneratedIdentityHashes []string                 `json:"generated_identity_hashes"`
	AccessTransitions       []string                 `json:"access_transitions"`
	ContentChangeProof      filesContentChangeProof  `json:"content_change_proof"`
	StartedAt               string                   `json:"started_at"`
	CompletedAt             string                   `json:"completed_at"`
	ActiveCleanupCounts     filesActiveCleanupCounts `json:"active_cleanup_counts"`
	TrashRetention          string                   `json:"trash_retention"`
	FinalState              string                   `json:"final_state"`
	Cleanup                 string                   `json:"cleanup"`
	Status                  string                   `json:"status"`
}

func TestFilesAcceptanceEvidenceIsSanitized(t *testing.T) {
	t.Setenv("HUBSPOT_ACCEPTANCE_EVIDENCE_DIR", t.TempDir())
	t.Setenv("HUBSPOT_ACCEPTANCE_CANDIDATE_COMMIT", strings.Repeat("a", 40))
	t.Setenv("HUBSPOT_ACCEPTANCE_PORTAL_ID", "12345678")
	ids := []string{"10001", "10002", "20001", "20002", "10003", "10004", "20003"}
	writeFilesAcceptanceEvidence(t, acceptance.OpenTofu, ids, strings.Repeat("b", 64), strings.Repeat("c", 64), time.Date(2026, 8, 4, 5, 0, 0, 0, time.UTC), time.Date(2026, 8, 4, 5, 1, 0, 0, time.UTC))
	contents, err := os.ReadFile(filepath.Join(os.Getenv("HUBSPOT_ACCEPTANCE_EVIDENCE_DIR"), "files_configuration-tofu.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range append([]string{"12345678", "tf_acc_", "Files candidate content"}, ids...) {
		if bytes.Contains(contents, []byte(forbidden)) {
			t.Fatal("Files acceptance evidence exposed raw portal configuration")
		}
	}
	var evidence filesAcceptanceEvidence
	if err := json.Unmarshal(contents, &evidence); err != nil {
		t.Fatal(err)
	}
	if evidence.Engine != "tofu" || evidence.APIFamily != "files/2026-03" || evidence.ScopeFamily != "files" || evidence.Cleanup != "passed" || evidence.Status != "passed" || evidence.ActiveCleanupCounts.Files != 0 || evidence.ActiveCleanupCounts.Folders != 0 || len(evidence.GeneratedIdentityHashes) != 7 {
		t.Fatalf("Files acceptance evidence did not preserve the sanitized contract: %#v", evidence)
	}
}

func writeFilesAcceptanceEvidence(t *testing.T, engine acceptance.Engine, ids []string, beforeDigest, afterDigest string, startedAt, completedAt time.Time) {
	t.Helper()
	directory := requiredEnvironment(t, "HUBSPOT_ACCEPTANCE_EVIDENCE_DIR")
	commit := requiredEnvironment(t, "HUBSPOT_ACCEPTANCE_CANDIDATE_COMMIT")
	if !exactCandidateCommit.MatchString(commit) {
		t.Fatal("Files acceptance evidence requires an exact candidate commit")
	}
	portalID := requiredEnvironment(t, "HUBSPOT_ACCEPTANCE_PORTAL_ID")
	identityHashes := make([]string, 0, len(ids))
	for _, id := range ids {
		identityHashes = append(identityHashes, evidenceHash("hubspot-files-identity", id))
	}
	sort.Strings(identityHashes)
	evidence := filesAcceptanceEvidence{
		CandidateCommit:         commit,
		Engine:                  string(engine),
		APIFamily:               "files/2026-03",
		ScopeFamily:             "files",
		PortalFingerprint:       evidenceHash("hubspot-portal", portalID),
		GeneratedIdentityHashes: identityHashes,
		AccessTransitions:       []string{"PRIVATE", "PUBLIC_NOT_INDEXABLE", "PRIVATE"},
		ContentChangeProof:      filesContentChangeProof{BeforeSHA256: beforeDigest, AfterSHA256: afterDigest},
		StartedAt:               startedAt.Format(time.RFC3339),
		CompletedAt:             completedAt.Format(time.RFC3339),
		ActiveCleanupCounts:     filesActiveCleanupCounts{},
		TrashRetention:          "expected",
		FinalState:              "zero_active_configuration",
		Cleanup:                 "passed",
		Status:                  "passed",
	}
	contents, err := json.Marshal(evidence)
	if err != nil {
		t.Fatalf("encode sanitized Files acceptance evidence: %v", err)
	}
	for _, forbidden := range append([]string{portalID}, ids...) {
		if bytes.Contains(contents, []byte(forbidden)) {
			t.Fatal("Files acceptance evidence contained a raw portal or generated identity")
		}
	}
	path := filepath.Join(directory, "files_configuration-"+string(engine)+".json")
	if err := os.WriteFile(path, append(contents, '\n'), 0o600); err != nil {
		t.Fatalf("write sanitized Files acceptance evidence: %v", err)
	}
}
