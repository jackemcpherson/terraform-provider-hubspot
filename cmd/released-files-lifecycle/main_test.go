// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

//go:build acceptance

package main

import (
	"context"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/jackemcpherson/terraform-provider-hubspot/internal/acceptance"
	"github.com/jackemcpherson/terraform-provider-hubspot/internal/hubspot"
)

func TestReleasedFilesActionsPreserveExactIdentitiesAndCleanupLeafFirst(t *testing.T) {
	fake := acceptance.NewFakeHubSpot("token", 123)
	server := httptest.NewServer(fake)
	defer server.Close()
	origin, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	clients, err := hubspot.NewClientSet(hubspot.TransportConfig{BaseURL: origin, AccessToken: "token"})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	const prefix = "tf_acc_released_"
	root, err := clients.FileFolders.Create(ctx, hubspot.FileFolderWrite{Name: prefix + "released_root"})
	if err != nil {
		t.Fatal(err)
	}
	leaf, err := clients.FileFolders.Create(ctx, hubspot.FileFolderWrite{Name: prefix + "released_leaf", ParentFolderID: &root.ID})
	if err != nil {
		t.Fatal(err)
	}
	file, err := clients.Files.Upload(ctx, hubspot.FileUpload{Name: prefix + "released_file.txt", FolderID: leaf.ID, Access: "PRIVATE", Bytes: []byte("released-before\n")})
	if err != nil {
		t.Fatal(err)
	}
	ids := []string{root.ID, leaf.ID, file.ID}
	if _, err := execute(ctx, "verify-active", ids, prefix, clients); err != nil {
		t.Fatal(err)
	}
	if _, err := execute(ctx, "drift", ids, prefix, clients); err != nil {
		t.Fatal(err)
	}
	drifted, err := clients.Files.Get(ctx, file.ID)
	if err != nil || drifted.ID != file.ID || drifted.Name != prefix+"released_file_drift.txt" || drifted.Access != "PUBLIC_NOT_INDEXABLE" || drifted.FileMD5 == file.FileMD5 {
		t.Fatalf("drifted file = %#v, %v", drifted, err)
	}
	if _, err := execute(ctx, "cleanup", ids, prefix, clients); err != nil {
		t.Fatal(err)
	}
	record, err := execute(ctx, "verify-terminal", ids, prefix, clients)
	if err != nil || strings.Contains(record, root.ID) || strings.Contains(record, file.ID) || !strings.Contains(record, `"active_owned_files":0`) || !strings.Contains(record, `"active_owned_folders":0`) {
		t.Fatalf("unsafe terminal record = %q, %v", record, err)
	}
}

func TestReleasedFilesActionsRejectUnsafeOwnership(t *testing.T) {
	clients := &hubspot.ClientSet{}
	if _, err := execute(context.Background(), "verify-active", []string{"1", "2", "3"}, "released_", clients); err == nil {
		t.Fatal("unsafe prefix accepted")
	}
	if _, err := execute(context.Background(), "unknown", []string{"1", "2", "3"}, "tf_acc_released_", clients); err == nil {
		t.Fatal("unknown action accepted")
	}
}
