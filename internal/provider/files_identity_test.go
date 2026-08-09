// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/jackemcpherson/terraform-provider-hubspot/internal/hubspot"
)

func TestFilesCanonicalReadBackRequiresExpectedIdentity(t *testing.T) {
	folderPlan := fileFolderResourceModel{Name: types.StringValue("assets")}
	folder := hubspot.FileFolder{ID: "10002", Name: "assets", Path: "/assets"}
	if folderMatchesPlan(folder, "10001", folderPlan) {
		t.Fatal("folder read-back with a different generated ID must not converge")
	}

	filePlan := fileResourceModel{
		Name:     types.StringValue("guide.txt"),
		FolderID: types.StringValue("10001"),
		Access:   types.StringValue("PRIVATE"),
	}
	file := hubspot.ManagedFile{
		ID: "20002", Name: "guide.txt", FolderID: "10001", Access: "PRIVATE",
		Path: "/assets/guide.txt", FileMD5: "900150983cd24fb0d6963f7d28e17f72", Size: 3,
	}
	if managedFileMatchesPlan(file, "20001", filePlan, managedFileSource{MD5: file.FileMD5, Size: file.Size}) {
		t.Fatal("file read-back with a different generated ID must not converge")
	}
}
