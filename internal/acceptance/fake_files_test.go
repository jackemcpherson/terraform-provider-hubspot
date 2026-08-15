// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

//go:build acceptance

package acceptance

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/jackemcpherson/terraform-provider-hubspot/internal/hubspot"
)

func TestFakeHubSpotModelsFilesIdentityCollisionsAndInPlaceReplacement(t *testing.T) {
	fake := NewFakeHubSpot("token", 123)
	server := httptest.NewServer(fake)
	defer server.Close()
	origin, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	clients, err := hubspot.NewClientSet(hubspot.TransportConfig{BaseURL: origin, AccessToken: "token", UserAgent: "fake-files-test"})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	root, err := clients.FileFolders.Create(ctx, hubspot.FileFolderWrite{Name: "root"})
	if err != nil {
		t.Fatal(err)
	}
	repeated, err := clients.FileFolders.Create(ctx, hubspot.FileFolderWrite{Name: "root"})
	if err != nil || repeated.ID != root.ID {
		t.Fatalf("same-parent/name folder create = %#v, %v", repeated, err)
	}
	file, err := clients.Files.Upload(ctx, hubspot.FileUpload{Name: "fixture.txt", FolderID: root.ID, Access: "PRIVATE", Bytes: []byte("one")})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := clients.Files.Upload(ctx, hubspot.FileUpload{Name: "duplicate.txt", FolderID: root.ID, Access: "PRIVATE", Bytes: []byte("one")}); err == nil {
		t.Fatal("EXACT_FOLDER/REJECT accepted duplicate contents")
	}
	returned := rawFakeUpload(t, server.URL, root.ID, "returned.txt", "one", "EXACT_FOLDER", "RETURN_EXISTING")
	if returned.ID != file.ID {
		t.Fatal("RETURN_EXISTING did not expose the existing identity distinctly from REJECT")
	}
	normalized := rawFakeUpload(t, server.URL, root.ID, "fixture.txt", "different", "", "")
	if normalized.ID == file.ID || normalized.Name == file.Name {
		t.Fatal("ordinary same-name upload did not allocate and normalize a distinct identity")
	}
	if err := clients.Files.Delete(ctx, normalized.ID); err != nil {
		t.Fatal(err)
	}
	createdAt := file.CreatedAt
	replaced, err := clients.Files.Replace(ctx, file.ID, hubspot.FileReplacement{Name: file.Name, Access: "PUBLIC_NOT_INDEXABLE", Bytes: []byte("replacement")})
	if err != nil {
		t.Fatal(err)
	}
	if replaced.ID != file.ID || replaced.CreatedAt != createdAt || replaced.FileMD5 == file.FileMD5 || replaced.Size == file.Size {
		t.Fatalf("in-place replacement = %#v, original %#v", replaced, file)
	}
	if err := clients.FileFolders.Delete(ctx, root.ID); err == nil {
		t.Fatal("non-empty folder deletion unexpectedly cascaded")
	}
	if err := clients.Files.Delete(ctx, file.ID); err != nil {
		t.Fatal(err)
	}
	if err := clients.FileFolders.Delete(ctx, root.ID); err != nil {
		t.Fatal(err)
	}
	reused, err := clients.FileFolders.Create(ctx, hubspot.FileFolderWrite{Name: "root"})
	if err != nil || reused.ID == root.ID {
		t.Fatalf("same-name recreation = %#v, %v", reused, err)
	}
}

func TestFakeHubSpotDistinguishesPatchFromAsyncDescendantPropagation(t *testing.T) {
	fake := NewFakeHubSpot("token", 123)
	server := httptest.NewServer(fake)
	defer server.Close()
	origin, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	clients, err := hubspot.NewClientSet(hubspot.TransportConfig{BaseURL: origin, AccessToken: "token", UserAgent: "fake-folder-propagation-test"})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	root, err := clients.FileFolders.Create(ctx, hubspot.FileFolderWrite{Name: "root"})
	if err != nil {
		t.Fatal(err)
	}
	child, err := clients.FileFolders.Create(ctx, hubspot.FileFolderWrite{Name: "child", ParentFolderID: &root.ID})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := clients.FileFolders.Rename(ctx, root.ID, "patched"); err != nil {
		t.Fatal(err)
	}
	patchedChild, err := clients.FileFolders.Get(ctx, child.ID)
	if err != nil || patchedChild.Path != "/root/child" {
		t.Fatalf("PATCH descendant path = %q, %v", patchedChild.Path, err)
	}
	task, err := clients.FileFolders.Update(ctx, root.ID, hubspot.FileFolderWrite{Name: "async"})
	if err != nil || task.Status != "PENDING" {
		t.Fatalf("initial async task = %#v, %v", task, err)
	}
	taskID := task.ID
	task, err = clients.FileFolders.GetUpdateTask(ctx, taskID)
	if err != nil || task.Status != "PENDING" {
		t.Fatalf("polled async task = %#v, %v", task, err)
	}
	pendingChild, err := clients.FileFolders.Get(ctx, child.ID)
	if err != nil || pendingChild.Path != "/root/child" {
		t.Fatalf("pending async descendant path = %q, %v", pendingChild.Path, err)
	}
	task, err = clients.FileFolders.GetUpdateTask(ctx, taskID)
	if err != nil || task.Status != "COMPLETE" {
		t.Fatalf("completed async task = %#v, %v", task, err)
	}
	updatedChild, err := clients.FileFolders.Get(ctx, child.ID)
	if err != nil || updatedChild.Path != "/async/child" {
		t.Fatalf("async descendant path = %q, %v", updatedChild.Path, err)
	}
}

func rawFakeUpload(t *testing.T, serverURL, folderID, name, contents, scope, strategy string) hubspot.ManagedFile {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write([]byte(contents)); err != nil {
		t.Fatal(err)
	}
	_ = writer.WriteField("fileName", name)
	_ = writer.WriteField("folderId", folderID)
	options, _ := json.Marshal(map[string]string{"access": "PRIVATE", "duplicateValidationScope": scope, "duplicateValidationStrategy": strategy})
	_ = writer.WriteField("options", string(options))
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	request, err := http.NewRequest(http.MethodPost, serverURL+"/files/2026-03/files", &body)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer token")
	request.Header.Set("Content-Type", writer.FormDataContentType())
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("raw upload status = %d", response.StatusCode)
	}
	var file hubspot.ManagedFile
	if err := json.NewDecoder(response.Body).Decode(&file); err != nil {
		t.Fatal(err)
	}
	return file
}

func TestFakeHubSpotInjectsFilesAmbiguityAndTaskFailures(t *testing.T) {
	fake := NewFakeHubSpot("token", 123)
	server := httptest.NewServer(fake)
	defer server.Close()
	origin, _ := url.Parse(server.URL)
	clients, err := hubspot.NewClientSet(hubspot.TransportConfig{BaseURL: origin, AccessToken: "token", UserAgent: "fake-files-fault-test"})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	root, err := clients.FileFolders.Create(ctx, hubspot.FileFolderWrite{Name: "root"})
	if err != nil {
		t.Fatal(err)
	}
	fake.FailNextFilesOperation(FilesFaultUploadUnknown)
	_, err = clients.Files.Upload(ctx, hubspot.FileUpload{Name: "unknown.txt", FolderID: root.ID, Access: "PRIVATE", Bytes: []byte("unknown")})
	var apiError *hubspot.Error
	if !errors.As(err, &apiError) || !apiError.Ambiguous || len(fake.ActiveManagedFileIDs()) != 1 {
		t.Fatalf("unknown upload fault = %v, active=%v", err, fake.ActiveManagedFileIDs())
	}
	knownID := fake.ActiveManagedFileIDs()[0]
	fake.FailNextFilesOperation(FilesFaultPatchApplied)
	name := "applied.txt"
	_, err = clients.Files.Update(ctx, knownID, hubspot.FilePatch{Name: &name})
	if !errors.As(err, &apiError) || !apiError.Ambiguous {
		t.Fatalf("applied PATCH fault = %v", err)
	}
	patched, err := clients.Files.Get(ctx, knownID)
	if err != nil || patched.Name != name {
		t.Fatalf("applied PATCH read-back = %#v, %v", patched, err)
	}
	fake.FailNextFilesOperation(FilesFaultFolderTaskCanceled)
	task, err := clients.FileFolders.Update(ctx, root.ID, hubspot.FileFolderWrite{Name: "canceled"})
	if err != nil {
		t.Fatal(err)
	}
	status, err := clients.FileFolders.GetUpdateTask(ctx, task.ID)
	if err != nil || status.Status != "CANCELED" {
		t.Fatalf("canceled task = %#v, %v", status, err)
	}
	task, err = clients.FileFolders.Update(ctx, root.ID, hubspot.FileFolderWrite{Name: "pending"})
	if err != nil {
		t.Fatal(err)
	}
	status, err = clients.FileFolders.GetUpdateTask(ctx, task.ID)
	if err != nil || status.Status != "PENDING" {
		t.Fatalf("first normal task status = %#v, %v", status, err)
	}
	status, err = clients.FileFolders.GetUpdateTask(ctx, task.ID)
	if err != nil || status.Status != "COMPLETE" {
		t.Fatalf("terminal normal task status = %#v, %v", status, err)
	}
	fake.FailNextFilesOperation(FilesFaultFolderTaskTimeout)
	task, err = clients.FileFolders.Update(ctx, root.ID, hubspot.FileFolderWrite{Name: "forever-pending"})
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		status, err = clients.FileFolders.GetUpdateTask(ctx, task.ID)
		if err != nil || status.Status != "PENDING" {
			t.Fatalf("timeout task status = %#v, %v", status, err)
		}
	}
}

func TestFakeHubSpotFilesSearchPaginatesWithoutLosingOwnedResults(t *testing.T) {
	fake := NewFakeHubSpot("token", 123)
	fake.fileFolders["10001"] = &hubspot.FileFolder{ID: "10001", Name: "a", Path: "/a", CreatedAt: "2026-08-01T00:00:00Z", UpdatedAt: "2026-08-01T00:00:00Z"}
	fake.fileFolders["10002"] = &hubspot.FileFolder{ID: "10002", Name: "b", Path: "/b", CreatedAt: "2026-08-01T00:00:00Z", UpdatedAt: "2026-08-01T00:00:00Z"}

	readPage := func(target string) struct {
		Results []hubspot.FileFolder `json:"results"`
		Paging  struct {
			Next struct {
				After string `json:"after"`
			} `json:"next"`
		} `json:"paging"`
	} {
		request := httptest.NewRequest(http.MethodGet, target, nil)
		request.Header.Set("Authorization", "Bearer token")
		response := httptest.NewRecorder()
		fake.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("search status = %d", response.Code)
		}
		var page struct {
			Results []hubspot.FileFolder `json:"results"`
			Paging  struct {
				Next struct {
					After string `json:"after"`
				} `json:"next"`
			} `json:"paging"`
		}
		if err := json.NewDecoder(response.Body).Decode(&page); err != nil {
			t.Fatal(err)
		}
		return page
	}
	first := readPage("http://example.invalid/files/2026-03/folders/search?limit=1")
	second := readPage("http://example.invalid/files/2026-03/folders/search?limit=1&after=" + first.Paging.Next.After)
	if len(first.Results) != 1 || first.Results[0].ID != "10001" || first.Paging.Next.After != "1" || len(second.Results) != 1 || second.Results[0].ID != "10002" || second.Paging.Next.After != "" {
		t.Fatalf("paged folder search = first %#v second %#v", first, second)
	}
}

func TestFakeHubSpotInjectsMalformedReadsAndStalePagination(t *testing.T) {
	fake := NewFakeHubSpot("token", 123)
	server := httptest.NewServer(fake)
	defer server.Close()
	origin, _ := url.Parse(server.URL)
	clients, err := hubspot.NewClientSet(hubspot.TransportConfig{BaseURL: origin, AccessToken: "token", UserAgent: "fake-files-malformed-test"})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	root, err := clients.FileFolders.Create(ctx, hubspot.FileFolderWrite{Name: "root"})
	if err != nil {
		t.Fatal(err)
	}
	file, err := clients.Files.Upload(ctx, hubspot.FileUpload{Name: "fixture.txt", FolderID: root.ID, Access: "PRIVATE", Bytes: []byte("one")})
	if err != nil {
		t.Fatal(err)
	}
	fake.FailNextFilesOperation(FilesFaultMalformedRead)
	if _, err := clients.Files.Get(ctx, file.ID); err == nil {
		t.Fatal("malformed Managed file read unexpectedly decoded")
	}
	fake.FailNextFilesOperation(FilesFaultStalePagination)
	if _, err := clients.Files.Search(ctx, &root.ID, ""); err == nil || !strings.Contains(err.Error(), "cursor repeated") {
		t.Fatalf("stale pagination error = %v", err)
	}
}
