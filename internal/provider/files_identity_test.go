// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

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

func TestFileFolderUpdateReadBackRetriesUntilExactValuesConverge(t *testing.T) {
	reads := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet || request.URL.Path != "/files/2026-03/folders/10001" {
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.Path)
		}
		reads++
		name := "stale"
		if reads > 1 {
			name = "assets"
		}
		writer.Header().Set("Content-Type", "application/json")
		io.WriteString(writer, `{"id":"10001","name":"`+name+`","path":"/`+name+`","createdAt":"2026-08-01T00:00:00Z","updatedAt":"2026-08-01T00:00:01Z"}`)
	}))
	defer server.Close()

	origin, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	clients, err := hubspot.NewClientSet(hubspot.TransportConfig{BaseURL: origin, AccessToken: "test", HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	resource := FileFolderResource{folders: clients.FileFolders}
	plan := fileFolderResourceModel{Name: types.StringValue("assets"), ParentFolderID: types.StringNull()}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	folder, err := resource.waitForFolderPlan(ctx, "10001", plan, "2026-08-01T00:00:00Z")
	if err != nil || folder.Name != "assets" || reads != 2 {
		t.Fatalf("read-back = %#v after %d reads, %v", folder, reads, err)
	}
}

func TestManagedFileUpdateReadBackRetriesUntilExactValuesConverge(t *testing.T) {
	reads := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet || request.URL.Path != "/files/2026-03/files/20001" {
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.Path)
		}
		reads++
		name := "stale"
		access := "PRIVATE"
		if reads > 1 {
			name = "guide"
			access = "PUBLIC_NOT_INDEXABLE"
		}
		writer.Header().Set("Content-Type", "application/json")
		io.WriteString(writer, `{"id":"20001","name":"`+name+`","parentFolderId":"10001","path":"/assets/`+name+`.txt","access":"`+access+`","fileMd5":"900150983cd24fb0d6963f7d28e17f72","size":3,"extension":"txt","type":"DOCUMENT","url":"https://example.invalid/file","defaultHostingUrl":"https://example.invalid/file","createdAt":"2026-08-01T00:00:00Z","updatedAt":"2026-08-01T00:00:01Z"}`)
	}))
	defer server.Close()

	origin, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	clients, err := hubspot.NewClientSet(hubspot.TransportConfig{BaseURL: origin, AccessToken: "test", HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	resource := FileResource{client: clients.Files}
	plan := fileResourceModel{Name: types.StringValue("guide.txt"), FolderID: types.StringValue("10001"), Access: types.StringValue("PUBLIC_NOT_INDEXABLE")}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	file, err := resource.waitForManagedFile(ctx, "20001", func(file hubspot.ManagedFile) bool {
		return managedFileMetadataMatches(file, "20001", plan)
	})
	if err != nil || file.Name != "guide.txt" || reads != 2 {
		t.Fatalf("read-back = %#v after %d reads, %v", file, reads, err)
	}
}
