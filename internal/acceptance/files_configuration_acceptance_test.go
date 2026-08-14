// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

//go:build acceptance

package acceptance_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/jackemcpherson/terraform-provider-hubspot/internal/acceptance"
)

const (
	liveFileFolderRootAddress   = "hubspot_file_folder.root"
	liveFileFolderChildAddress  = "hubspot_file_folder.child"
	liveFileFolderTargetAddress = "hubspot_file_folder.target"
	liveManagedFileAddress      = "hubspot_file.managed"
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
	prefix := requiredEnvironment(t, "HUBSPOT_ACCEPTANCE_PREFIX") + string(engine) + "_"
	ledger := requiredEnvironment(t, "HUBSPOT_ACCEPTANCE_CLEANUP_LEDGER")
	directory := t.TempDir()
	sourcePath := filepath.Join(directory, "managed.txt")
	initialBytes := []byte("Files maintenance content v1\n")
	updatedBytes := []byte("Files maintenance content v2\n")
	if err := os.WriteFile(sourcePath, initialBytes, 0o600); err != nil {
		t.Fatalf("write initial Files acceptance source: %v", err)
	}
	initialDigest := sourceDigest(initialBytes)
	updatedDigest := sourceDigest(updatedBytes)
	initial := liveFilesConfiguration(providerSource, sourcePath, initialDigest, prefix, filesConfigInitial)
	folderCollision := liveFilesConfiguration(providerSource, sourcePath, initialDigest, prefix, filesConfigFolderCollision)
	fileCollision := liveFilesConfiguration(providerSource, sourcePath, initialDigest, prefix, filesConfigFileCollision)
	var rootID, childID, targetID, initialFileID, finalFileID, reusedRootID, reusedChildID, reusedTargetID, reusedFileID string

	acceptance.Run(t, acceptance.Options{
		Engine: engine, Shard: acceptance.FilesConfiguration, Prefix: prefix, LedgerPath: ledger,
	}, func(session *acceptance.Session) {
		session.Apply(initial)
		rootID = session.OpaqueStateString(liveFileFolderRootAddress, "id")
		childID = session.OpaqueStateString(liveFileFolderChildAddress, "id")
		targetID = session.OpaqueStateString(liveFileFolderTargetAddress, "id")
		initialFileID = session.OpaqueStateString(liveManagedFileAddress, "id")
		for _, id := range []string{rootID, childID, targetID, initialFileID} {
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
		fileUpdated := liveFilesConfiguration(providerSource, sourcePath, updatedDigest, prefix, filesConfigFileUpdated)
		session.Apply(fileUpdated)
		requireFilesIdentitiesUnchanged(t, session, rootID, childID, targetID, initialFileID)
		session.RequireStateString(liveManagedFileAddress, "access", "PUBLIC_NOT_INDEXABLE")
		session.RequireEmptyPlan(fileUpdated)

		renamed := liveFilesConfiguration(providerSource, sourcePath, updatedDigest, prefix, filesConfigRenamed)
		session.Apply(renamed)
		requireFilesIdentitiesUnchanged(t, session, rootID, childID, targetID, initialFileID)
		session.RequireStateStringAfterRefresh(renamed, liveManagedFileAddress, "path", "/"+prefix+"root_updated/"+prefix+"child/"+prefix+"managed_updated.txt")
		session.RequireStateString(liveFileFolderChildAddress, "path", "/"+prefix+"root_updated/"+prefix+"child")
		session.RequireEmptyPlan(renamed)

		moved := liveFilesConfiguration(providerSource, sourcePath, updatedDigest, prefix, filesConfigMoved)
		session.Apply(moved)
		requireFilesIdentitiesUnchanged(t, session, rootID, childID, targetID, initialFileID)

		private := liveFilesConfiguration(providerSource, sourcePath, updatedDigest, prefix, filesConfigPrivate)
		session.Apply(private)
		session.RequireStateStringAfterRefresh(private, liveManagedFileAddress, "path", "/"+prefix+"target/"+prefix+"child/"+prefix+"managed_updated.txt")
		session.RequireStateString(liveManagedFileAddress, "access", "PRIVATE")
		session.RequireStateString(liveFileFolderChildAddress, "path", "/"+prefix+"target/"+prefix+"child")
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
		files, folders := session.RequireFilesTerminal(prefix, []string{initialFileID, finalFileID}, []string{rootID, childID, targetID})
		if files != 0 || folders != 0 {
			t.Fatal("Files destroy retained active owned configuration")
		}
		session.Apply(private)
		reusedRootID = session.OpaqueStateString(liveFileFolderRootAddress, "id")
		reusedChildID = session.OpaqueStateString(liveFileFolderChildAddress, "id")
		reusedTargetID = session.OpaqueStateString(liveFileFolderTargetAddress, "id")
		reusedFileID = session.OpaqueStateString(liveManagedFileAddress, "id")
		if reusedRootID == rootID || reusedChildID == childID || reusedTargetID == targetID || reusedFileID == initialFileID || reusedFileID == finalFileID {
			t.Fatal("immediate Files name reuse did not allocate new generated identities")
		}
		session.Destroy(private)
		session.RequireFilesTerminal(prefix, []string{reusedFileID}, []string{reusedRootID, reusedChildID, reusedTargetID})
	})

}

func requireFilesIdentitiesUnchanged(t *testing.T, session *acceptance.Session, rootID, childID, targetID, fileID string) {
	t.Helper()
	if session.OpaqueStateString(liveFileFolderRootAddress, "id") != rootID || session.OpaqueStateString(liveFileFolderChildAddress, "id") != childID || session.OpaqueStateString(liveFileFolderTargetAddress, "id") != targetID || session.OpaqueStateString(liveManagedFileAddress, "id") != fileID {
		t.Fatal("an in-place Files update changed a generated identity")
	}
}

type filesConfigStage int

const (
	filesConfigInitial filesConfigStage = iota
	filesConfigFolderCollision
	filesConfigFileCollision
	filesConfigFileUpdated
	filesConfigRenamed
	filesConfigMoved
	filesConfigPrivate
)

func liveFilesConfiguration(providerSource, sourcePath, digest, prefix string, stage filesConfigStage) string {
	rootName := prefix + "root"
	childName := prefix + "child"
	targetName := prefix + "target"
	fileName := prefix + "managed.txt"
	access := "PRIVATE"
	parent := "  parent_folder_id = hubspot_file_folder.root.id\n"
	folderCollision := ""
	fileCollision := ""
	if stage >= filesConfigFileUpdated {
		fileName = prefix + "managed_updated.txt"
		access = "PUBLIC_NOT_INDEXABLE"
	}
	if stage >= filesConfigRenamed {
		rootName = prefix + "root_updated"
	}
	if stage >= filesConfigMoved {
		parent = "  parent_folder_id = hubspot_file_folder.target.id\n"
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

resource "hubspot_file_folder" "target" {
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
%s%s`, providerSource, rootName, targetName, childName, parent, fileName, sourcePath, digest, access, folderCollision, fileCollision)
}
