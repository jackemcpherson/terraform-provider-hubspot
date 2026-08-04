// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

//go:build acceptance

package acceptance_test

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jackemcpherson/terraform-provider-hubspot/internal/acceptance"
)

func TestHermeticFilesConfigurationLifecycle(t *testing.T) {
	runHermeticFilesConfigurationLifecycle(t, acceptance.OpenTofu, "registry.opentofu.org/jackemcpherson/hubspot")
}

func TestHermeticFilesConfigurationLifecycleTerraformParity(t *testing.T) {
	runHermeticFilesConfigurationLifecycle(t, acceptance.Terraform, "registry.terraform.io/jackemcpherson/hubspot")
}

func TestHermeticFilesConfigurationRecovery(t *testing.T) {
	runHermeticFilesConfigurationRecovery(t, acceptance.OpenTofu, "registry.opentofu.org/jackemcpherson/hubspot")
}

func TestHermeticFilesConfigurationRecoveryTerraformParity(t *testing.T) {
	runHermeticFilesConfigurationRecovery(t, acceptance.Terraform, "registry.terraform.io/jackemcpherson/hubspot")
}

func runHermeticFilesConfigurationRecovery(t *testing.T, engine acceptance.Engine, providerSource string) {
	t.Helper()
	if _, err := exec.LookPath(string(engine)); err != nil {
		t.Skipf("pinned %s executable is not installed", engine)
	}
	fake := acceptance.NewFakeHubSpot(hermeticToken, 666000667)
	server := httptest.NewServer(fake)
	t.Cleanup(server.Close)
	sourcePath := filepath.Join(t.TempDir(), "recovery.txt")
	contents := []byte("recovery bytes\n")
	if err := os.WriteFile(sourcePath, contents, 0o600); err != nil {
		t.Fatal(err)
	}
	initial := hermeticFilesConfig(server.URL, providerSource, sourcePath, sourceDigest(contents), false)
	updated := hermeticFilesConfig(server.URL, providerSource, sourcePath, sourceDigest(contents), true)
	t.Setenv("HUBSPOT_ACCESS_TOKEN", hermeticToken)
	acceptance.Run(t, acceptance.Options{
		Engine: engine, Shard: acceptance.FilesConfiguration,
		Prefix: "tf_acc_hermetic_files_recovery_", LedgerPath: t.TempDir() + "/cleanup.jsonl", ProbeBaseURL: server.URL,
	}, func(session *acceptance.Session) {
		fake.FailNextFilesOperation(acceptance.FilesFaultUploadUnknown)
		session.RequireApplyFailure(initial)
		unknown := fake.ActiveManagedFileIDs()
		if len(unknown) != 1 || !fake.DisappearManagedFile(unknown[0]) {
			t.Fatalf("no-ID ambiguous upload residual = %v", unknown)
		}

		fake.FailNextFilesOperation(acceptance.FilesFaultUploadKnown)
		session.Apply(initial)
		session.RequireEmptyPlan(initial)

		fake.FailNextFilesOperation(acceptance.FilesFaultPatchApplied)
		session.Apply(updated)
		session.RequireEmptyPlan(updated)

		replacedBytes := []byte("ambiguous replacement applied\n")
		if err := os.WriteFile(sourcePath, replacedBytes, 0o600); err != nil {
			t.Fatal(err)
		}
		replaced := hermeticFilesConfig(server.URL, providerSource, sourcePath, sourceDigest(replacedBytes), true)
		fake.FailNextFilesOperation(acceptance.FilesFaultReplaceApplied)
		session.Apply(replaced)
		session.RequireEmptyPlan(replaced)

		notAppliedBytes := []byte("ambiguous replacement retry\n")
		if err := os.WriteFile(sourcePath, notAppliedBytes, 0o600); err != nil {
			t.Fatal(err)
		}
		notApplied := hermeticFilesConfig(server.URL, providerSource, sourcePath, sourceDigest(notAppliedBytes), true)
		fake.FailNextFilesOperation(acceptance.FilesFaultReplaceNotApplied)
		session.RequireApplyFailure(notApplied)
		session.Apply(notApplied)
		session.RequireEmptyPlan(notApplied)

		canceled := strings.Replace(notApplied, "Hermetic Files root updated", "Hermetic Files root retry", 1)
		fake.FailNextFilesOperation(acceptance.FilesFaultFolderTaskCanceled)
		session.RequireApplyFailure(canceled)
		session.Apply(canceled)
		session.RequireEmptyPlan(canceled)

		malformed := strings.Replace(canceled, "Hermetic Files root retry", "Hermetic Files root final", 1)
		fake.FailNextFilesOperation(acceptance.FilesFaultFolderTaskMalformed)
		session.RequireApplyFailure(malformed)
		session.Apply(malformed)
		session.RequireEmptyPlan(malformed)

		fake.FailNextFilesOperation(acceptance.FilesFaultDeleteNotApplied)
		session.RequireDestroyFailure(malformed)
		fake.FailNextFilesOperation(acceptance.FilesFaultDeleteApplied)
		session.Destroy(malformed)
		if len(fake.ActiveManagedFileIDs()) != 0 || len(fake.ActiveFileFolderIDs()) != 0 {
			t.Fatalf("recovery destroy left active fake configuration: files=%v folders=%v", fake.ActiveManagedFileIDs(), fake.ActiveFileFolderIDs())
		}
	})
}

func runHermeticFilesConfigurationLifecycle(t *testing.T, engine acceptance.Engine, providerSource string) {
	t.Helper()
	if _, err := exec.LookPath(string(engine)); err != nil {
		t.Skipf("pinned %s executable is not installed", engine)
	}
	fake := acceptance.NewFakeHubSpot(hermeticToken, 666000666)
	server := httptest.NewServer(fake)
	t.Cleanup(server.Close)
	directory := t.TempDir()
	sourcePath := filepath.Join(directory, "managed.txt")
	initialBytes := []byte("initial managed bytes\n")
	updatedBytes := []byte("updated managed bytes\n")
	if err := os.WriteFile(sourcePath, initialBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	initial := hermeticFilesConfig(server.URL, providerSource, sourcePath, sourceDigest(initialBytes), false)
	t.Setenv("HUBSPOT_ACCESS_TOKEN", hermeticToken)
	acceptance.Run(t, acceptance.Options{
		Engine: engine, Shard: acceptance.FilesConfiguration,
		Prefix: "tf_acc_hermetic_files_", LedgerPath: t.TempDir() + "/cleanup.jsonl", ProbeBaseURL: server.URL,
	}, func(session *acceptance.Session) {
		session.Apply(initial)
		rootID := session.OpaqueStateString("hubspot_file_folder.root", "id")
		childID := session.OpaqueStateString("hubspot_file_folder.child", "id")
		fileID := session.OpaqueStateString("hubspot_file.managed", "id")
		session.RequireEmptyPlan(initial)

		if err := os.WriteFile(sourcePath, updatedBytes, 0o600); err != nil {
			t.Fatal(err)
		}
		updated := hermeticFilesConfig(server.URL, providerSource, sourcePath, sourceDigest(updatedBytes), true)
		session.Apply(updated)
		if session.OpaqueStateString("hubspot_file_folder.root", "id") != rootID || session.OpaqueStateString("hubspot_file_folder.child", "id") != childID || session.OpaqueStateString("hubspot_file.managed", "id") != fileID {
			t.Fatal("in-place Files updates changed a generated identity")
		}
		session.RequireStateString("hubspot_file.managed", "access", "PUBLIC_NOT_INDEXABLE")
		session.RequireEmptyPlan(updated)
		session.RequireStateString("hubspot_file_folder.root", "path", "/Hermetic Files root updated")
		session.RequireStateString("hubspot_file_folder.child", "path", "/Hermetic Files root updated/Hermetic Files child updated")
		session.RequireStateString("hubspot_file.managed", "path", "/Hermetic Files root updated/Hermetic Files child updated/managed-updated.txt")

		if !fake.DriftFileFolderName(childID, "out-of-band-folder-drift") {
			t.Fatal("could not inject File folder name drift")
		}
		session.RequirePlanDiff(updated)
		session.Apply(updated)
		session.RequireEmptyPlan(updated)

		session.RemoveState("hubspot_file_folder.child")
		session.Import("hubspot_file_folder.child", childID)
		session.RequireEmptyPlan(updated)

		if !fake.DriftManagedFileContent(fileID, []byte("out-of-band drift\n")) {
			t.Fatal("could not inject Managed file content drift")
		}
		session.RequirePlanDiff(updated)
		session.Apply(updated)
		session.RequireEmptyPlan(updated)

		session.RemoveState("hubspot_file.managed")
		session.Import("hubspot_file.managed", fileID)
		session.RequirePlanDiff(updated)
		patchesBeforeImport, replacementsBeforeImport := fake.ManagedFileWriteCounts(fileID)
		session.Apply(updated)
		if patches, replacements := fake.ManagedFileWriteCounts(fileID); patches != patchesBeforeImport || replacements != replacementsBeforeImport {
			t.Fatal("import source reconciliation sent an unnecessary remote write")
		}
		session.RequireEmptyPlan(updated)

		movedSourcePath := filepath.Join(directory, "moved-managed.txt")
		if err := os.WriteFile(movedSourcePath, updatedBytes, 0o600); err != nil {
			t.Fatal(err)
		}
		pathOnly := hermeticFilesConfig(server.URL, providerSource, movedSourcePath, sourceDigest(updatedBytes), true)
		session.RequirePlanDiff(pathOnly)
		session.Apply(pathOnly)
		if patches, replacements := fake.ManagedFileWriteCounts(fileID); patches != patchesBeforeImport || replacements != replacementsBeforeImport {
			t.Fatal("path-only source change sent an unnecessary remote write")
		}
		session.RequireEmptyPlan(pathOnly)
		updated = pathOnly

		if !fake.DisappearManagedFile(fileID) {
			t.Fatal("could not inject external Managed file disappearance")
		}
		session.Refresh(updated)
		session.RequireStateAbsent("hubspot_file.managed")
		session.Apply(updated)
		recreatedFileID := session.OpaqueStateString("hubspot_file.managed", "id")
		if recreatedFileID == fileID {
			t.Fatal("external disappearance recreation reused the generated identity")
		}
		session.RequireEmptyPlan(updated)

		session.Destroy(updated)
		if len(fake.ActiveManagedFileIDs()) != 0 || len(fake.ActiveFileFolderIDs()) != 0 {
			t.Fatalf("Files destroy left active fake configuration: files=%v folders=%v", fake.ActiveManagedFileIDs(), fake.ActiveFileFolderIDs())
		}
		session.Apply(updated)
		if session.OpaqueStateString("hubspot_file_folder.root", "id") == rootID || session.OpaqueStateString("hubspot_file.managed", "id") == recreatedFileID {
			t.Fatal("immediate same-name reuse did not allocate new generated identities")
		}
		session.Destroy(updated)
	})
}

func hermeticFilesConfig(apiBaseURL, providerSource, sourcePath, sourceSHA256 string, updated bool) string {
	rootName := "Hermetic Files root"
	childName := "Hermetic Files child"
	fileName := "managed.txt"
	access := "PRIVATE"
	if updated {
		rootName = "Hermetic Files root updated"
		childName = "Hermetic Files child updated"
		fileName = "managed-updated.txt"
		access = "PUBLIC_NOT_INDEXABLE"
	}
	return fmt.Sprintf(`
terraform {
  required_providers {
    hubspot = {
      source = %q
    }
  }
}

provider "hubspot" {
  access_token = %q
  api_base_url = %q
}

resource "hubspot_file_folder" "root" {
  name = %q
}

resource "hubspot_file_folder" "child" {
  name             = %q
  parent_folder_id = hubspot_file_folder.root.id
}

resource "hubspot_file" "managed" {
  name          = %q
  folder_id     = hubspot_file_folder.child.id
  source_path   = %q
  source_sha256 = %q
  access        = %q
}
`, providerSource, hermeticToken, apiBaseURL, rootName, childName, fileName, sourcePath, sourceSHA256, access)
}

func sourceDigest(contents []byte) string {
	digest := sha256.Sum256(contents)
	return hex.EncodeToString(digest[:])
}
