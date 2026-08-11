// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

package hubspot

import (
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestFileFolderClientPinsGeneratedIDLifecycleAndPagination(t *testing.T) {
	requests := make([]string, 0)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests = append(requests, request.Method+" "+request.URL.RequestURI())
		writer.Header().Set("Content-Type", "application/json")
		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/files/2026-03/folders/search":
			if request.URL.Query().Get("after") == "" {
				io.WriteString(writer, `{"results":[{"id":"11","name":"child","parentFolderId":"7","path":"/root/child","createdAt":"2026-08-01T00:00:00Z","updatedAt":"2026-08-01T00:00:00Z"}],"paging":{"next":{"after":"next"}}}`)
			} else {
				io.WriteString(writer, `{"results":[]}`)
			}
		case request.Method == http.MethodPost && request.URL.Path == "/files/2026-03/folders":
			var input FileFolderWrite
			if err := json.NewDecoder(request.Body).Decode(&input); err != nil || input.Name != "child" || input.ParentFolderID == nil || *input.ParentFolderID != "7" {
				t.Fatalf("folder create payload = %#v, %v", input, err)
			}
			writer.WriteHeader(http.StatusCreated)
			io.WriteString(writer, `{"id":"11","name":"child","parentFolderId":"7","path":"/root/child","createdAt":"2026-08-01T00:00:00Z","updatedAt":"2026-08-01T00:00:00Z"}`)
		case request.Method == http.MethodGet && request.URL.Path == "/files/2026-03/folders/11":
			io.WriteString(writer, `{"id":"11","name":"child","parentFolderId":"7","path":"/root/child","createdAt":"2026-08-01T00:00:00Z","updatedAt":"2026-08-01T00:00:00Z"}`)
		case request.Method == http.MethodPatch && request.URL.Path == "/files/2026-03/folders/11":
			var payload struct {
				Name string `json:"name"`
			}
			if err := json.NewDecoder(request.Body).Decode(&payload); err != nil || payload.Name != "renamed" {
				t.Fatalf("folder rename payload = %#v, %v", payload, err)
			}
			io.WriteString(writer, `{"id":"11","name":"renamed","parentFolderId":"7","path":"/root/renamed","createdAt":"2026-08-01T00:00:00Z","updatedAt":"2026-08-01T00:00:01Z"}`)
		case request.Method == http.MethodPost && request.URL.Path == "/files/2026-03/folders/update/async":
			var payload struct {
				ID             string `json:"id"`
				Name           string `json:"name"`
				ParentFolderID *int64 `json:"parentFolderId"`
			}
			if err := json.NewDecoder(request.Body).Decode(&payload); err != nil || payload.ID != "11" || payload.Name != "renamed" || payload.ParentFolderID == nil || *payload.ParentFolderID != 7 {
				t.Fatalf("folder update payload = %#v, %v", payload, err)
			}
			io.WriteString(writer, `{"id":"task-1"}`)
		case request.Method == http.MethodGet && request.URL.Path == "/files/2026-03/folders/update/async/tasks/task-1/status":
			writer.Header().Set("Retry-After", "7")
			io.WriteString(writer, `{"status":"COMPLETE","errors":[]}`)
		case request.Method == http.MethodDelete && request.URL.Path == "/files/2026-03/folders/11":
			writer.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.RequestURI())
		}
	}))
	defer server.Close()

	client := &FileFolderClient{transport: newTestTransport(t, server.URL)}
	parent := "7"
	folders, err := client.Search(context.Background(), &parent, "child")
	if err != nil || len(folders) != 1 || folders[0].ID != "11" {
		t.Fatalf("folder search = %#v, %v", folders, err)
	}
	created, err := client.Create(context.Background(), FileFolderWrite{Name: "child", ParentFolderID: &parent})
	if err != nil || created.ID != "11" {
		t.Fatalf("folder create = %#v, %v", created, err)
	}
	if _, err := client.Get(context.Background(), "11"); err != nil {
		t.Fatal(err)
	}
	renamed, err := client.Rename(context.Background(), "11", "renamed")
	if err != nil || renamed.Name != "renamed" {
		t.Fatalf("folder rename = %#v, %v", renamed, err)
	}
	task, err := client.Update(context.Background(), "11", FileFolderWrite{Name: "renamed", ParentFolderID: &parent})
	if err != nil || task.ID != "task-1" {
		t.Fatalf("folder update task = %#v, %v", task, err)
	}
	status, err := client.GetUpdateTask(context.Background(), task.ID)
	if err != nil || status.Status != "COMPLETE" || status.RetryAfter != 7*time.Second {
		t.Fatalf("folder task status = %#v, %v", status, err)
	}
	if err := client.Delete(context.Background(), "11"); err != nil {
		t.Fatal(err)
	}
	if len(requests) != 8 || !strings.Contains(requests[0], "parentFolderId=7") || !strings.Contains(requests[0], "name=child") || !strings.Contains(requests[1], "after=next") {
		t.Fatalf("folder requests = %#v", requests)
	}
}

func TestManagedFileClientUsesExactFolderDuplicateRejectionAndBoundedUpdates(t *testing.T) {
	requests := make([]string, 0)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests = append(requests, request.Method+" "+request.URL.RequestURI())
		writer.Header().Set("Content-Type", "application/json")
		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/files/2026-03/files/search":
			if request.URL.Query().Get("name") != "fixture" || request.URL.Query().Get("parentFolderId") != "7" {
				t.Fatalf("file search query = %q", request.URL.RawQuery)
			}
			io.WriteString(writer, `{"results":[`+canonicalManagedFileJSON("21", "fixture.txt", "7", "PRIVATE")+`]}`)
		case request.Method == http.MethodPost && request.URL.Path == "/files/2026-03/files":
			assertManagedFileMultipart(t, request, "fixture.txt", "7", "PRIVATE", true)
			writer.WriteHeader(http.StatusCreated)
			io.WriteString(writer, canonicalManagedFileJSON("21", "fixture.txt", "7", "PRIVATE"))
		case request.Method == http.MethodPatch && request.URL.Path == "/files/2026-03/files/21":
			var patch FilePatch
			if err := json.NewDecoder(request.Body).Decode(&patch); err != nil || patch.Name == nil || *patch.Name != "renamed" || patch.FolderID != nil || patch.Access != nil {
				t.Fatalf("file patch = %#v, %v", patch, err)
			}
			io.WriteString(writer, canonicalManagedFileJSON("21", "renamed.txt", "7", "PRIVATE"))
		case request.Method == http.MethodPut && request.URL.Path == "/files/2026-03/files/21":
			assertManagedFileMultipart(t, request, "renamed.txt", "", "PUBLIC_NOT_INDEXABLE", false)
			io.WriteString(writer, canonicalManagedFileJSON("21", "renamed.txt", "7", "PUBLIC_NOT_INDEXABLE"))
		case request.Method == http.MethodDelete && request.URL.Path == "/files/2026-03/files/21":
			writer.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.RequestURI())
		}
	}))
	defer server.Close()

	client := &FileClient{transport: newTestTransport(t, server.URL)}
	folderID := "7"
	files, err := client.Search(context.Background(), &folderID, "fixture.txt")
	if err != nil || len(files) != 1 || files[0].Name != "fixture.txt" {
		t.Fatalf("search = %#v, %v", files, err)
	}
	created, err := client.Upload(context.Background(), FileUpload{Name: "fixture.txt", FolderID: "7", Access: "PRIVATE", Bytes: []byte("bytes")})
	if err != nil || created.ID != "21" || created.Name != "fixture.txt" {
		t.Fatalf("upload = %#v, %v", created, err)
	}
	name := "renamed.txt"
	updated, err := client.Update(context.Background(), "21", FilePatch{Name: &name})
	if err != nil || updated.Name != name {
		t.Fatalf("update = %#v, %v", updated, err)
	}
	replaced, err := client.Replace(context.Background(), "21", FileReplacement{Name: name, Access: "PUBLIC_NOT_INDEXABLE", Bytes: []byte("new bytes")})
	if err != nil || replaced.Name != name {
		t.Fatalf("replace = %#v, %v", replaced, err)
	}
	if err := client.Delete(context.Background(), "21"); err != nil {
		t.Fatal(err)
	}
	want := []string{"GET /files/2026-03/files/search?limit=100&name=fixture&parentFolderId=7", "POST /files/2026-03/files", "PATCH /files/2026-03/files/21", "PUT /files/2026-03/files/21", "DELETE /files/2026-03/files/21"}
	if !reflect.DeepEqual(requests, want) {
		t.Fatalf("requests = %#v, want %#v", requests, want)
	}
}

func assertManagedFileMultipart(t *testing.T, request *http.Request, name, folderID, access string, create bool) {
	t.Helper()
	mediaType := request.Header.Get("Content-Type")
	if !strings.HasPrefix(mediaType, "multipart/form-data; boundary=") {
		t.Fatalf("content type = %q", mediaType)
	}
	reader := multipart.NewReader(request.Body, strings.TrimPrefix(mediaType, "multipart/form-data; boundary="))
	form, err := reader.ReadForm(1 << 20)
	if err != nil {
		t.Fatal(err)
	}
	if got := form.Value["fileName"]; len(got) != 1 || got[0] != name {
		t.Fatalf("fileName = %#v", got)
	}
	if create {
		if got := form.Value["folderId"]; len(got) != 1 || got[0] != folderID {
			t.Fatalf("folderId = %#v", got)
		}
		var options map[string]string
		if err := json.Unmarshal([]byte(form.Value["options"][0]), &options); err != nil || options["access"] != access || options["duplicateValidationScope"] != "EXACT_FOLDER" || options["duplicateValidationStrategy"] != "REJECT" {
			t.Fatalf("upload options = %#v, %v", options, err)
		}
	} else {
		var options map[string]string
		if err := json.Unmarshal([]byte(form.Value["options"][0]), &options); err != nil || !reflect.DeepEqual(options, map[string]string{"access": access}) {
			t.Fatalf("replacement options = %#v, %v", options, err)
		}
	}
	if len(form.File["file"]) != 1 {
		t.Fatal("multipart file part missing")
	}
}

func canonicalManagedFileJSON(id, name, folderID, access string) string {
	return `{"id":"` + id + `","name":"` + managedFileSearchName(name) + `","parentFolderId":"` + folderID + `","path":"/folder/` + name + `","access":"` + access + `","fileMd5":"4f2a91e15af2631ff9424564b8a45fb2","size":5,"extension":"txt","type":"DOCUMENT","url":"https://example.invalid/` + name + `","defaultHostingUrl":"https://example.invalid/` + name + `","createdAt":"2026-08-01T00:00:00Z","updatedAt":"2026-08-01T00:00:01Z"}`
}

func TestManagedFileWireNameCanonicalization(t *testing.T) {
	for _, test := range []struct {
		filename string
		search   string
	}{
		{filename: "guide.txt", search: "guide"},
		{filename: "archive.tar.gz", search: "archive.tar"},
		{filename: "README", search: "README"},
		{filename: ".well-known", search: ".well-known"},
	} {
		if got := managedFileSearchName(test.filename); got != test.search {
			t.Fatalf("managedFileSearchName(%q) = %q, want %q", test.filename, got, test.search)
		}
		if got := canonicalManagedFilename(test.search, "/folder/"+test.filename); got != test.filename {
			t.Fatalf("canonicalManagedFilename(%q) = %q, want %q", test.filename, got, test.filename)
		}
	}
	if got := canonicalManagedFilename("different", "/folder/guide.txt"); got != "different" {
		t.Fatalf("inconsistent wire name was canonicalized to %q", got)
	}
	var partial ManagedFile
	if err := json.Unmarshal([]byte(`{"id":"21","archived":"invalid"}`), &partial); err == nil || partial.ID != "21" {
		t.Fatalf("malformed response did not retain its known identity: %#v, %v", partial, err)
	}
}

func TestFilesSearchOmitsNamesAboveServiceQueryLimit(t *testing.T) {
	query := make(url.Values)
	setFilesSearchName(query, strings.Repeat("x", filesSearchNameLimit))
	if got := query.Get("name"); got == "" {
		t.Fatal("service-safe search name was omitted")
	}
	query = make(url.Values)
	setFilesSearchName(query, strings.Repeat("x", filesSearchNameLimit+1))
	if query.Has("name") {
		t.Fatal("over-limit search name was sent to HubSpot")
	}
}
