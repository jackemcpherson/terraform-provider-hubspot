// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

//go:build acceptance

package acceptance

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackemcpherson/terraform-provider-hubspot/internal/hubspot"
)

type fakeManagedFile struct {
	file     hubspot.ManagedFile
	contents []byte
	patches  int
	replaces int
}

// FilesFault selects deterministic one-shot outcomes for fail-closed Files
// lifecycle tests. Fault hooks remain fake-only and never enter provider config.
type FilesFault string

const (
	FilesFaultFolderTaskCanceled  FilesFault = "folder_task_canceled"
	FilesFaultFolderTaskError     FilesFault = "folder_task_error"
	FilesFaultFolderTaskMalformed FilesFault = "folder_task_malformed"
	FilesFaultFolderTaskTimeout   FilesFault = "folder_task_timeout"
	FilesFaultFolderCreateUnknown FilesFault = "folder_create_unknown"
	FilesFaultFolderCreateKnown   FilesFault = "folder_create_known"
	FilesFaultUploadUnknown       FilesFault = "upload_unknown"
	FilesFaultUploadKnown         FilesFault = "upload_known"
	FilesFaultPatchApplied        FilesFault = "patch_applied"
	FilesFaultPatchNotApplied     FilesFault = "patch_not_applied"
	FilesFaultReplaceApplied      FilesFault = "replace_applied"
	FilesFaultReplaceNotApplied   FilesFault = "replace_not_applied"
	FilesFaultDeleteApplied       FilesFault = "delete_applied"
	FilesFaultDeleteNotApplied    FilesFault = "delete_not_applied"
	FilesFaultMalformedRead       FilesFault = "malformed_read"
	FilesFaultStalePagination     FilesFault = "stale_pagination"
)

func (f *FakeHubSpot) handleFiles(response http.ResponseWriter, request *http.Request, rest []string) {
	switch {
	case len(rest) == 1 && rest[0] == "folders":
		f.handleFileFolderCollection(response, request)
	case len(rest) == 2 && rest[0] == "folders" && rest[1] == "search":
		f.handleFileFolderSearch(response, request)
	case len(rest) == 2 && rest[0] == "folders":
		f.handleFileFolderItem(response, request, rest[1])
	case len(rest) == 3 && rest[0] == "folders" && rest[1] == "update" && rest[2] == "async":
		f.handleFileFolderUpdate(response, request)
	case len(rest) == 6 && rest[0] == "folders" && rest[1] == "update" && rest[2] == "async" && rest[3] == "tasks" && rest[5] == "status":
		f.handleFileFolderTask(response, request, rest[4])
	case len(rest) == 1 && rest[0] == "files":
		f.handleManagedFileCollection(response, request)
	case len(rest) == 2 && rest[0] == "files" && rest[1] == "search":
		f.handleManagedFileSearch(response, request)
	case len(rest) == 2 && rest[0] == "files":
		f.handleManagedFileItem(response, request, rest[1])
	default:
		writeFakeError(response, http.StatusNotFound, "OBJECT_NOT_FOUND", "", "No Files route matched this request.")
	}
}

func (f *FakeHubSpot) handleFileFolderCollection(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		writeFakeError(response, http.StatusMethodNotAllowed, "VALIDATION_ERROR", "", "Unsupported method.")
		return
	}
	var input hubspot.FileFolderWrite
	if !decodeFakeBody(response, request, &input) {
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, folder := range f.fileFolders {
		if folder.Name == input.Name && fakeNullableStringEqual(folder.ParentFolderID, input.ParentFolderID) {
			writeFakeJSON(response, http.StatusCreated, *folder)
			return
		}
	}
	if input.ParentFolderID != nil {
		if _, exists := f.fileFolders[*input.ParentFolderID]; !exists {
			writeFakeError(response, http.StatusBadRequest, "VALIDATION_ERROR", "", "Parent folder does not exist.")
			return
		}
	}
	f.nextFileFolderID++
	id := strconv.Itoa(10_000 + f.nextFileFolderID)
	timestamp := f.advanceFilesTimestamp()
	folder := &hubspot.FileFolder{ID: id, Name: input.Name, ParentFolderID: copyFakeString(input.ParentFolderID), CreatedAt: timestamp, UpdatedAt: timestamp}
	folder.Path = f.deriveFolderPath(folder)
	f.fileFolders[id] = folder
	switch f.nextFilesFault {
	case FilesFaultFolderCreateUnknown:
		f.nextFilesFault = ""
		dropFakeConnection(response)
		return
	case FilesFaultFolderCreateKnown:
		f.nextFilesFault = ""
		response.WriteHeader(http.StatusCreated)
		fmt.Fprintf(response, `{"id":%q,"archived":"invalid"}`, id)
		return
	}
	writeFakeJSON(response, http.StatusCreated, *folder)
}

func (f *FakeHubSpot) handleFileFolderSearch(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		writeFakeError(response, http.StatusMethodNotAllowed, "VALIDATION_ERROR", "", "Unsupported method.")
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	parent, hasParent := request.URL.Query()["parentFolderId"]
	name := request.URL.Query().Get("name")
	ids := make([]string, 0, len(f.fileFolders))
	for id, folder := range f.fileFolders {
		if hasParent && (folder.ParentFolderID == nil || *folder.ParentFolderID != parent[0]) {
			continue
		}
		if name != "" && folder.Name != name {
			continue
		}
		ids = append(ids, id)
	}
	sort.Strings(ids)
	results := make([]hubspot.FileFolder, 0, len(ids))
	start, end, next := fakeFilesPage(request, len(ids))
	for _, id := range ids[start:end] {
		results = append(results, *f.fileFolders[id])
	}
	body := map[string]any{"results": results}
	if f.nextFilesFault == FilesFaultStalePagination || f.staleFilesSearchCursor {
		next = "0"
		if request.URL.Query().Get("after") == "0" {
			f.nextFilesFault = ""
			f.staleFilesSearchCursor = false
		} else {
			f.staleFilesSearchCursor = true
		}
	}
	if next != "" {
		body["paging"] = map[string]any{"next": map[string]string{"after": next}}
	}
	writeFakeJSON(response, http.StatusOK, body)
}

func (f *FakeHubSpot) handleFileFolderItem(response http.ResponseWriter, request *http.Request, id string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	folder := f.fileFolders[id]
	switch request.Method {
	case http.MethodGet:
		if folder == nil {
			writeFakeError(response, http.StatusNotFound, "OBJECT_NOT_FOUND", "", "No active File folder matched this identity.")
			return
		}
		writeFakeJSON(response, http.StatusOK, *folder)
	case http.MethodPatch:
		if folder == nil {
			writeFakeError(response, http.StatusNotFound, "OBJECT_NOT_FOUND", "", "No active File folder matched this identity.")
			return
		}
		var payload struct {
			Name string `json:"name"`
		}
		if !decodeFakeBody(response, request, &payload) {
			return
		}
		if f.nextFilesFault == FilesFaultFolderTaskCanceled {
			f.nextFilesFault = ""
			writeFakeError(response, http.StatusConflict, "VALIDATION_ERROR", "", "Folder rename was rejected.")
			return
		}
		for candidateID, candidate := range f.fileFolders {
			if candidateID != id && candidate.Name == payload.Name && fakeNullableStringEqual(candidate.ParentFolderID, folder.ParentFolderID) {
				writeFakeError(response, http.StatusConflict, "VALIDATION_ERROR", "", "Target File folder already exists.")
				return
			}
		}
		folder.Name = payload.Name
		folder.UpdatedAt = f.advanceFilesTimestamp()
		f.refreshDerivedFilePaths()
		if f.nextFilesFault == FilesFaultFolderTaskMalformed {
			f.nextFilesFault = ""
			response.WriteHeader(http.StatusOK)
			fmt.Fprintf(response, `{"id":%q,"archived":"invalid"}`, id)
			return
		}
		writeFakeJSON(response, http.StatusOK, *folder)
	case http.MethodDelete:
		if folder == nil {
			writeFakeError(response, http.StatusNotFound, "OBJECT_NOT_FOUND", "", "No active File folder matched this identity.")
			return
		}
		if f.folderHasDirectChildren(id) {
			writeFakeError(response, http.StatusConflict, "VALIDATION_ERROR", "", "Folder has active children.")
			return
		}
		delete(f.fileFolders, id)
		response.WriteHeader(http.StatusNoContent)
	default:
		writeFakeError(response, http.StatusMethodNotAllowed, "VALIDATION_ERROR", "", "Unsupported method.")
	}
}

func (f *FakeHubSpot) handleFileFolderUpdate(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		writeFakeError(response, http.StatusMethodNotAllowed, "VALIDATION_ERROR", "", "Unsupported method.")
		return
	}
	var payload struct {
		ID             string `json:"id"`
		Name           string `json:"name"`
		ParentFolderID *int64 `json:"parentFolderId"`
	}
	if !decodeFakeBody(response, request, &payload) {
		return
	}
	input := hubspot.FileFolderWrite{Name: payload.Name}
	if payload.ParentFolderID != nil {
		parent := strconv.FormatInt(*payload.ParentFolderID, 10)
		input.ParentFolderID = &parent
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	folder := f.fileFolders[payload.ID]
	if folder == nil {
		writeFakeError(response, http.StatusNotFound, "OBJECT_NOT_FOUND", "", "No active File folder matched this identity.")
		return
	}
	for id, candidate := range f.fileFolders {
		if id != payload.ID && candidate.Name == input.Name && fakeNullableStringEqual(candidate.ParentFolderID, input.ParentFolderID) {
			writeFakeError(response, http.StatusConflict, "VALIDATION_ERROR", "", "Target File folder already exists.")
			return
		}
	}
	f.nextFolderTaskID++
	task := hubspot.FolderUpdateTask{ID: "folder-task-" + strconv.Itoa(f.nextFolderTaskID), Status: "PENDING", Errors: []json.RawMessage{}}
	switch f.nextFilesFault {
	case FilesFaultFolderTaskCanceled:
		task.Status = "CANCELED"
		f.nextFilesFault = ""
		f.folderTasks[task.ID] = task
		writeFakeJSON(response, http.StatusAccepted, task)
		return
	case FilesFaultFolderTaskError:
		task.Status = "FAILED"
		task.Errors = []json.RawMessage{json.RawMessage(`{"category":"FAKE_TERMINAL"}`)}
		f.nextFilesFault = ""
		f.folderTasks[task.ID] = task
		writeFakeJSON(response, http.StatusAccepted, task)
		return
	case FilesFaultFolderTaskMalformed:
		task.Status = ""
		f.nextFilesFault = ""
		f.folderTasks[task.ID] = task
		writeFakeJSON(response, http.StatusAccepted, task)
		return
	case FilesFaultFolderTaskTimeout:
		f.nextFilesFault = ""
		f.folderTasks[task.ID] = task
		writeFakeJSON(response, http.StatusAccepted, task)
		return
	}
	folder.Name = input.Name
	folder.ParentFolderID = copyFakeString(input.ParentFolderID)
	folder.UpdatedAt = f.advanceFilesTimestamp()
	f.refreshDerivedFilePaths()
	f.folderTasks[task.ID] = task
	f.pendingFolderTasks[task.ID] = true
	writeFakeJSON(response, http.StatusAccepted, task)
}

func (f *FakeHubSpot) handleFileFolderTask(response http.ResponseWriter, request *http.Request, id string) {
	if request.Method != http.MethodGet {
		writeFakeError(response, http.StatusMethodNotAllowed, "VALIDATION_ERROR", "", "Unsupported method.")
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	task, exists := f.folderTasks[id]
	if !exists {
		writeFakeError(response, http.StatusNotFound, "OBJECT_NOT_FOUND", "", "No folder task matched this identity.")
		return
	}
	if f.pendingFolderTasks[id] {
		delete(f.pendingFolderTasks, id)
		task.Status = "COMPLETE"
		f.folderTasks[id] = task
		writeFakeJSON(response, http.StatusOK, hubspot.FolderUpdateTask{ID: task.ID, Status: "PENDING", Errors: task.Errors})
		return
	}
	writeFakeJSON(response, http.StatusOK, task)
}

func (f *FakeHubSpot) handleManagedFileCollection(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		writeFakeError(response, http.StatusMethodNotAllowed, "VALIDATION_ERROR", "", "Unsupported method.")
		return
	}
	upload, ok := decodeFakeManagedFileMultipart(response, request)
	if !ok {
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, exists := f.fileFolders[upload.folderID]; !exists {
		writeFakeError(response, http.StatusBadRequest, "VALIDATION_ERROR", "", "Destination folder does not exist.")
		return
	}
	for _, existing := range f.managedFiles {
		if existing.file.FolderID != upload.folderID {
			continue
		}
		if upload.scope == "EXACT_FOLDER" && string(existing.contents) == string(upload.contents) {
			switch upload.strategy {
			case "REJECT":
				writeFakeError(response, http.StatusBadRequest, "VALIDATION_ERROR", "", "Duplicate file rejected.")
				return
			case "RETURN_EXISTING":
				writeFakeJSON(response, http.StatusCreated, existing.file)
				return
			}
		}
	}
	name := upload.name
	if f.activeFileNameExists(upload.folderID, name, "") {
		name = normalizedFakeFileName(name, f.nextManagedFileID+1)
	}
	f.nextManagedFileID++
	id := strconv.Itoa(20_000 + f.nextManagedFileID)
	timestamp := f.advanceFilesTimestamp()
	file := fakeManagedFile{contents: append([]byte(nil), upload.contents...)}
	file.file = f.newFakeManagedFile(id, name, upload.folderID, upload.access, timestamp, timestamp, upload.contents)
	f.managedFiles[id] = &file
	switch f.nextFilesFault {
	case FilesFaultUploadUnknown:
		f.nextFilesFault = ""
		dropFakeConnection(response)
		return
	case FilesFaultUploadKnown:
		f.nextFilesFault = ""
		response.WriteHeader(http.StatusCreated)
		fmt.Fprintf(response, `{"id":%q,"archived":"invalid"}`, id)
		return
	}
	writeFakeJSON(response, http.StatusCreated, file.file)
}

func (f *FakeHubSpot) handleManagedFileSearch(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		writeFakeError(response, http.StatusMethodNotAllowed, "VALIDATION_ERROR", "", "Unsupported method.")
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	folderID := request.URL.Query().Get("parentFolderId")
	name := request.URL.Query().Get("name")
	ids := make([]string, 0, len(f.managedFiles))
	for id, file := range f.managedFiles {
		if folderID != "" && file.file.FolderID != folderID {
			continue
		}
		if name != "" && file.file.Name != name {
			continue
		}
		ids = append(ids, id)
	}
	sort.Strings(ids)
	results := make([]hubspot.ManagedFile, 0, len(ids))
	start, end, next := fakeFilesPage(request, len(ids))
	for _, id := range ids[start:end] {
		results = append(results, f.managedFiles[id].file)
	}
	body := map[string]any{"results": results}
	if f.nextFilesFault == FilesFaultStalePagination || f.staleFilesSearchCursor {
		next = "0"
		if request.URL.Query().Get("after") == "0" {
			f.nextFilesFault = ""
			f.staleFilesSearchCursor = false
		} else {
			f.staleFilesSearchCursor = true
		}
	}
	if next != "" {
		body["paging"] = map[string]any{"next": map[string]string{"after": next}}
	}
	writeFakeJSON(response, http.StatusOK, body)
}

func (f *FakeHubSpot) handleManagedFileItem(response http.ResponseWriter, request *http.Request, id string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	file := f.managedFiles[id]
	switch request.Method {
	case http.MethodGet:
		if file == nil {
			writeFakeError(response, http.StatusNotFound, "OBJECT_NOT_FOUND", "", "No active Managed file matched this identity.")
			return
		}
		if f.nextFilesFault == FilesFaultMalformedRead || f.malformedNextManagedFileRead {
			f.nextFilesFault = ""
			f.malformedNextManagedFileRead = false
			response.WriteHeader(http.StatusOK)
			fmt.Fprintf(response, `{"id":%q,"archived":"invalid"}`, id)
			return
		}
		writeFakeJSON(response, http.StatusOK, file.file)
	case http.MethodPatch:
		if file == nil {
			writeFakeError(response, http.StatusNotFound, "OBJECT_NOT_FOUND", "", "No active Managed file matched this identity.")
			return
		}
		var patch hubspot.FilePatch
		if !decodeFakeBody(response, request, &patch) {
			return
		}
		if f.nextFilesFault == FilesFaultPatchNotApplied {
			f.nextFilesFault = ""
			dropFakeConnection(response)
			return
		}
		if patch.Name != nil {
			file.file.Name = *patch.Name + filepath.Ext(file.file.Name)
		}
		if patch.FolderID != nil {
			if _, exists := f.fileFolders[*patch.FolderID]; !exists {
				writeFakeError(response, http.StatusBadRequest, "VALIDATION_ERROR", "", "Destination folder does not exist.")
				return
			}
			file.file.FolderID = *patch.FolderID
		}
		if patch.Access != nil {
			file.file.Access = *patch.Access
		}
		file.file.Path = f.deriveFilePath(file.file.FolderID, file.file.Name)
		file.file.URL = fakeFileURL(file.file.Path)
		file.file.DefaultHostingURL = fakeDefaultFileURL(file.file.Path)
		file.file.UpdatedAt = f.advanceFilesTimestamp()
		file.patches++
		if f.nextFilesFault == FilesFaultPatchApplied {
			f.nextFilesFault = ""
			dropFakeConnection(response)
			return
		}
		writeFakeJSON(response, http.StatusOK, file.file)
	case http.MethodPut:
		if file == nil {
			writeFakeError(response, http.StatusNotFound, "OBJECT_NOT_FOUND", "", "No active Managed file matched this identity.")
			return
		}
		replacement, ok := decodeFakeManagedFileMultipart(response, request)
		if !ok {
			return
		}
		if f.nextFilesFault == FilesFaultReplaceNotApplied {
			f.nextFilesFault = ""
			dropFakeConnection(response)
			return
		}
		file.contents = append([]byte(nil), replacement.contents...)
		file.file.Name = replacement.name
		file.file.Access = replacement.access
		file.file.FileMD5 = fakeMD5(replacement.contents)
		file.file.Size = int64(len(replacement.contents))
		file.file.Extension = strings.TrimPrefix(strings.ToLower(filepath.Ext(file.file.Name)), ".")
		file.file.Path = f.deriveFilePath(file.file.FolderID, file.file.Name)
		file.file.URL = fakeFileURL(file.file.Path)
		file.file.DefaultHostingURL = fakeDefaultFileURL(file.file.Path)
		file.file.UpdatedAt = f.advanceFilesTimestamp()
		file.replaces++
		if f.nextFilesFault == FilesFaultReplaceApplied {
			f.nextFilesFault = ""
			dropFakeConnection(response)
			return
		}
		writeFakeJSON(response, http.StatusOK, file.file)
	case http.MethodDelete:
		if file == nil {
			writeFakeError(response, http.StatusNotFound, "OBJECT_NOT_FOUND", "", "No active Managed file matched this identity.")
			return
		}
		if f.nextFilesFault == FilesFaultDeleteNotApplied {
			f.nextFilesFault = ""
			f.malformedNextManagedFileRead = true
			dropFakeConnection(response)
			return
		}
		delete(f.managedFiles, id)
		if f.nextFilesFault == FilesFaultDeleteApplied {
			f.nextFilesFault = ""
			dropFakeConnection(response)
			return
		}
		response.WriteHeader(http.StatusNoContent)
	default:
		writeFakeError(response, http.StatusMethodNotAllowed, "VALIDATION_ERROR", "", "Unsupported method.")
	}
}

type fakeManagedFileUpload struct {
	name, folderID, access, scope, strategy string
	contents                                []byte
}

func decodeFakeManagedFileMultipart(response http.ResponseWriter, request *http.Request) (fakeManagedFileUpload, bool) {
	if err := request.ParseMultipartForm(21_000_000); err != nil {
		writeFakeError(response, http.StatusBadRequest, "VALIDATION_ERROR", "", "Multipart upload could not be decoded.")
		return fakeManagedFileUpload{}, false
	}
	file, _, err := request.FormFile("file")
	if err != nil {
		writeFakeError(response, http.StatusBadRequest, "VALIDATION_ERROR", "", "Multipart upload omitted file bytes.")
		return fakeManagedFileUpload{}, false
	}
	defer file.Close()
	contents, err := io.ReadAll(io.LimitReader(file, 20_000_001))
	if err != nil || len(contents) > 20_000_000 {
		writeFakeError(response, http.StatusBadRequest, "VALIDATION_ERROR", "", "Multipart upload exceeded the supported limit.")
		return fakeManagedFileUpload{}, false
	}
	var options map[string]string
	if err := json.Unmarshal([]byte(request.FormValue("options")), &options); err != nil {
		writeFakeError(response, http.StatusBadRequest, "VALIDATION_ERROR", "", "Multipart options could not be decoded.")
		return fakeManagedFileUpload{}, false
	}
	return fakeManagedFileUpload{name: request.FormValue("fileName"), folderID: request.FormValue("folderId"), access: options["access"], scope: options["duplicateValidationScope"], strategy: options["duplicateValidationStrategy"], contents: contents}, true
}

func (f *FakeHubSpot) newFakeManagedFile(id, name, folderID, access, createdAt, updatedAt string, contents []byte) hubspot.ManagedFile {
	path := f.deriveFilePath(folderID, name)
	return hubspot.ManagedFile{ID: id, Name: name, FolderID: folderID, Path: path, Access: access, FileMD5: fakeMD5(contents), Size: int64(len(contents)), Extension: strings.TrimPrefix(strings.ToLower(filepath.Ext(name)), "."), Type: "DOCUMENT", URL: fakeFileURL(path), DefaultHostingURL: fakeDefaultFileURL(path), CreatedAt: createdAt, UpdatedAt: updatedAt}
}

func (f *FakeHubSpot) deriveFolderPath(folder *hubspot.FileFolder) string {
	if folder.ParentFolderID == nil {
		return "/" + folder.Name
	}
	parent := f.fileFolders[*folder.ParentFolderID]
	if parent == nil {
		return "/" + folder.Name
	}
	return strings.TrimSuffix(parent.Path, "/") + "/" + folder.Name
}

func (f *FakeHubSpot) deriveFilePath(folderID, name string) string {
	folder := f.fileFolders[folderID]
	if folder == nil {
		return "/" + name
	}
	return strings.TrimSuffix(folder.Path, "/") + "/" + name
}

func (f *FakeHubSpot) refreshDerivedFilePaths() {
	remaining := len(f.fileFolders)
	for remaining > 0 {
		changed := false
		for _, folder := range f.fileFolders {
			path := f.deriveFolderPath(folder)
			if folder.Path != path {
				folder.Path = path
				changed = true
			}
		}
		if !changed {
			break
		}
		remaining--
	}
	for _, file := range f.managedFiles {
		file.file.Path = f.deriveFilePath(file.file.FolderID, file.file.Name)
		file.file.URL = fakeFileURL(file.file.Path)
		file.file.DefaultHostingURL = fakeDefaultFileURL(file.file.Path)
	}
}

func (f *FakeHubSpot) folderHasDirectChildren(id string) bool {
	for _, folder := range f.fileFolders {
		if folder.ParentFolderID != nil && *folder.ParentFolderID == id {
			return true
		}
	}
	for _, file := range f.managedFiles {
		if file.file.FolderID == id {
			return true
		}
	}
	return false
}

func (f *FakeHubSpot) activeFileNameExists(folderID, name, excludedID string) bool {
	for id, file := range f.managedFiles {
		if id != excludedID && file.file.FolderID == folderID && file.file.Name == name {
			return true
		}
	}
	return false
}

func (f *FakeHubSpot) advanceFilesTimestamp() string {
	f.nextFilesRevision++
	return time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC).Add(time.Duration(f.nextFilesRevision) * time.Second).Format(time.RFC3339)
}

func (f *FakeHubSpot) ActiveFileFolderIDs() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	ids := make([]string, 0, len(f.fileFolders))
	for id := range f.fileFolders {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func (f *FakeHubSpot) ActiveManagedFileIDs() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	ids := make([]string, 0, len(f.managedFiles))
	for id := range f.managedFiles {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func (f *FakeHubSpot) ManagedFileWriteCounts(id string) (patches, replacements int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if file := f.managedFiles[id]; file != nil {
		return file.patches, file.replaces
	}
	return 0, 0
}

func (f *FakeHubSpot) DisappearManagedFile(id string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, exists := f.managedFiles[id]; !exists {
		return false
	}
	delete(f.managedFiles, id)
	return true
}

func (f *FakeHubSpot) DriftManagedFileContent(id string, contents []byte) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	file := f.managedFiles[id]
	if file == nil {
		return false
	}
	file.contents = append([]byte(nil), contents...)
	file.file.FileMD5 = fakeMD5(contents)
	file.file.Size = int64(len(contents))
	file.file.UpdatedAt = f.advanceFilesTimestamp()
	return true
}

func (f *FakeHubSpot) DriftFileFolderName(id, name string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	folder := f.fileFolders[id]
	if folder == nil {
		return false
	}
	folder.Name = name
	folder.UpdatedAt = f.advanceFilesTimestamp()
	f.refreshDerivedFilePaths()
	return true
}

func (f *FakeHubSpot) DriftFileFolderParent(id string, parentFolderID *string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	folder := f.fileFolders[id]
	if folder == nil || (parentFolderID != nil && f.fileFolders[*parentFolderID] == nil) {
		return false
	}
	folder.ParentFolderID = copyFakeString(parentFolderID)
	folder.UpdatedAt = f.advanceFilesTimestamp()
	f.refreshDerivedFilePaths()
	return true
}

func (f *FakeHubSpot) DisappearFileFolder(id string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fileFolders[id] == nil || f.folderHasDirectChildren(id) {
		return false
	}
	delete(f.fileFolders, id)
	return true
}

func (f *FakeHubSpot) DriftManagedFileMetadata(id, name, folderID, access string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	file := f.managedFiles[id]
	if file == nil || f.fileFolders[folderID] == nil {
		return false
	}
	file.file.Name = name
	file.file.FolderID = folderID
	file.file.Access = access
	file.file.Path = f.deriveFilePath(folderID, name)
	file.file.URL = fakeFileURL(file.file.Path)
	file.file.DefaultHostingURL = fakeDefaultFileURL(file.file.Path)
	file.file.UpdatedAt = f.advanceFilesTimestamp()
	return true
}

func (f *FakeHubSpot) FailNextFilesOperation(fault FilesFault) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nextFilesFault = fault
}

func fakeNullableStringEqual(a, b *string) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func copyFakeString(value *string) *string {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func fakeMD5(contents []byte) string {
	digest := md5.Sum(contents)
	return hex.EncodeToString(digest[:])
}

func fakeFileURL(path string) string        { return "https://files.example.invalid" + path }
func fakeDefaultFileURL(path string) string { return "https://hubspot-files.example.invalid" + path }

func normalizedFakeFileName(name string, suffix int) string {
	extension := filepath.Ext(name)
	base := strings.TrimSuffix(name, extension)
	return fmt.Sprintf("%s_%d%s", base, suffix, extension)
}

func dropFakeConnection(response http.ResponseWriter) {
	hijacker, ok := response.(http.Hijacker)
	if !ok {
		panic("fake response writer cannot inject an ambiguous transport outcome")
	}
	connection, _, err := hijacker.Hijack()
	if err != nil {
		panic(fmt.Sprintf("inject ambiguous Files outcome: %v", err))
	}
	_ = connection.Close()
}

func fakeFilesPage(request *http.Request, total int) (start, end int, next string) {
	limit := 100
	if parsed, err := strconv.Atoi(request.URL.Query().Get("limit")); err == nil && parsed > 0 && parsed <= 100 {
		limit = parsed
	}
	if parsed, err := strconv.Atoi(request.URL.Query().Get("after")); err == nil && parsed >= 0 && parsed < total {
		start = parsed
	}
	end = start + limit
	if end > total {
		end = total
	}
	if end < total {
		next = strconv.Itoa(end)
	}
	return start, end, next
}
