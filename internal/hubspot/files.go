// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

package hubspot

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/url"
	pathpkg "path"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const filesAPIRoot = "/files/2026-03"

const (
	filesRequestTimeout  = 30 * time.Second
	filesTransferTimeout = 5 * time.Minute
	filesSearchNameLimit = 19
)

// FileFolderClient owns the typed File folder boundary. Its API routing is
// deliberately private so provider consumers cannot select an unproved family.
type FileFolderClient struct{ transport *Transport }

// FileClient owns the typed Managed file boundary. Source bytes exist only in
// write inputs and are never part of the returned state representation.
type FileClient struct{ transport *Transport }

type FileFolder struct {
	ID             string  `json:"id"`
	Name           string  `json:"name"`
	ParentFolderID *string `json:"parentFolderId"`
	Path           string  `json:"path"`
	CreatedAt      string  `json:"createdAt"`
	UpdatedAt      string  `json:"updatedAt"`
	Archived       bool    `json:"archived"`
}

type FileFolderWrite struct {
	Name           string  `json:"name"`
	ParentFolderID *string `json:"parentFolderId,omitempty"`
}

type FolderUpdateTask struct {
	ID         string            `json:"id"`
	Status     string            `json:"status"`
	Errors     []json.RawMessage `json:"errors"`
	Result     *FileFolder       `json:"result"`
	RetryAfter time.Duration     `json:"-"`
}

func (t *FolderUpdateTask) UnmarshalJSON(data []byte) error {
	type folderUpdateTaskWire FolderUpdateTask
	var decoded struct {
		folderUpdateTaskWire
		TaskID string `json:"taskId"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*t = FolderUpdateTask(decoded.folderUpdateTaskWire)
	if t.ID == "" {
		t.ID = decoded.TaskID
	}
	return nil
}

type ManagedFile struct {
	ID                string  `json:"id"`
	Name              string  `json:"name"`
	FolderID          string  `json:"parentFolderId"`
	Path              string  `json:"path"`
	Access            string  `json:"access"`
	FileMD5           string  `json:"fileMd5"`
	Size              int64   `json:"size"`
	Extension         string  `json:"extension"`
	Type              string  `json:"type"`
	Encoding          *string `json:"encoding"`
	Height            *int64  `json:"height"`
	Width             *int64  `json:"width"`
	URL               string  `json:"url"`
	DefaultHostingURL string  `json:"defaultHostingUrl"`
	CreatedAt         string  `json:"createdAt"`
	UpdatedAt         string  `json:"updatedAt"`
	Archived          bool    `json:"archived"`
}

// UnmarshalJSON translates HubSpot's wire-level file stem back to the full
// filename represented by the Files path. The API accepts a full fileName on
// writes, but returns the final extension separately from name on reads.
func (f *ManagedFile) UnmarshalJSON(data []byte) error {
	type managedFileWire ManagedFile
	var decoded managedFileWire
	err := json.Unmarshal(data, &decoded)
	decoded.Name = canonicalManagedFilename(decoded.Name, decoded.Path)
	*f = ManagedFile(decoded)
	return err
}

type FileUpload struct {
	Name     string
	FolderID string
	Access   string
	Bytes    []byte
}

type FilePatch struct {
	Name     *string `json:"name,omitempty"`
	FolderID *string `json:"parentFolderId,omitempty"`
	Access   *string `json:"access,omitempty"`
}

type FileReplacement struct {
	Name   string
	Access string
	Bytes  []byte
}

type filesPage[T any] struct {
	Results []T `json:"results"`
	Paging  struct {
		Next struct {
			After string `json:"after"`
		} `json:"next"`
	} `json:"paging"`
}

func fileFoldersPath() string  { return filesAPIRoot + "/folders" }
func managedFilesPath() string { return filesAPIRoot + "/files" }

func (c *FileFolderClient) Search(ctx context.Context, parentFolderID *string, name string) ([]FileFolder, error) {
	query := url.Values{"limit": []string{"100"}}
	if parentFolderID != nil {
		query.Set("parentFolderId", *parentFolderID)
	}
	setFilesSearchName(query, name)
	return searchFilesPages[FileFolder](ctx, c.transport, "file-folder-search", fileFoldersPath()+"/search", query)
}

func (c *FileFolderClient) Create(ctx context.Context, input FileFolderWrite) (FileFolder, error) {
	body, err := json.Marshal(input)
	if err != nil {
		return FileFolder{}, err
	}
	var out FileFolder
	err = c.transport.Do(ctx, Operation{Name: "file-folder-create", Method: http.MethodPost, Path: fileFoldersPath(), Replay: ReplayNever, Timeout: filesRequestTimeout}, bytes.NewReader(body), &out)
	if err != nil {
		return out, err
	}
	if err := validateGeneratedFilesID(out.ID); err != nil {
		return out, fmt.Errorf("HubSpot File folder response: %w", err)
	}
	return out, nil
}

func (c *FileFolderClient) Get(ctx context.Context, id string) (FileFolder, error) {
	if err := validateGeneratedFilesID(id); err != nil {
		return FileFolder{}, err
	}
	var out FileFolder
	if err := c.transport.Do(ctx, Operation{Name: "file-folder-read", Method: http.MethodGet, Path: fileFoldersPath() + "/" + url.PathEscape(id), Replay: ReplaySafe, Timeout: filesRequestTimeout}, nil, &out); err != nil {
		return out, err
	}
	if err := validateGeneratedFilesID(out.ID); err != nil {
		return out, fmt.Errorf("HubSpot File folder response: %w", err)
	}
	return out, nil
}

func (c *FileFolderClient) Rename(ctx context.Context, id, name string) (FileFolder, error) {
	if err := validateGeneratedFilesID(id); err != nil {
		return FileFolder{}, err
	}
	body, err := json.Marshal(struct {
		Name string `json:"name"`
	}{Name: name})
	if err != nil {
		return FileFolder{}, err
	}
	var out FileFolder
	err = c.transport.Do(ctx, Operation{Name: "file-folder-rename", Method: http.MethodPatch, Path: fileFoldersPath() + "/" + url.PathEscape(id), Replay: ReplayNever, Timeout: filesRequestTimeout}, bytes.NewReader(body), &out)
	if err != nil {
		return out, err
	}
	if err := validateGeneratedFilesID(out.ID); err != nil {
		return out, fmt.Errorf("HubSpot File folder response: %w", err)
	}
	return out, nil
}

func (c *FileFolderClient) Update(ctx context.Context, id string, input FileFolderWrite) (FolderUpdateTask, error) {
	if err := validateGeneratedFilesID(id); err != nil {
		return FolderUpdateTask{}, err
	}
	var parentFolderID *int64
	if input.ParentFolderID != nil {
		parsed, err := strconv.ParseInt(*input.ParentFolderID, 10, 64)
		if err != nil || parsed <= 0 {
			return FolderUpdateTask{}, errors.New("parent folder id must be a non-zero decimal integer")
		}
		parentFolderID = &parsed
	}
	payload := struct {
		ID             string `json:"id"`
		Name           string `json:"name"`
		ParentFolderID *int64 `json:"parentFolderId,omitempty"`
	}{ID: id, Name: input.Name, ParentFolderID: parentFolderID}
	body, err := json.Marshal(payload)
	if err != nil {
		return FolderUpdateTask{}, err
	}
	var task FolderUpdateTask
	err = c.transport.Do(ctx, Operation{Name: "file-folder-update", Method: http.MethodPost, Path: fileFoldersPath() + "/update/async", Replay: ReplayNever, Timeout: filesRequestTimeout}, bytes.NewReader(body), &task)
	if err != nil {
		return task, err
	}
	if strings.TrimSpace(task.ID) == "" {
		return task, errors.New("HubSpot File folder update response omitted task id")
	}
	return task, nil
}

func (c *FileFolderClient) GetUpdateTask(ctx context.Context, taskID string) (FolderUpdateTask, error) {
	if strings.TrimSpace(taskID) == "" {
		return FolderUpdateTask{}, errors.New("file folder task id must not be empty")
	}
	var task FolderUpdateTask
	var retryAfter time.Duration
	operation := Operation{Name: "file-folder-update-status", Method: http.MethodGet, Path: fileFoldersPath() + "/update/async/tasks/" + url.PathEscape(taskID) + "/status", Replay: ReplaySafe, Timeout: filesRequestTimeout, HeaderSink: func(header http.Header) {
		retryAfter = parseRetryAfterAt(header.Get("Retry-After"), c.transport.clock())
	}}
	if err := c.transport.Do(ctx, operation, nil, &task); err != nil {
		return task, err
	}
	task.RetryAfter = retryAfter
	return task, nil
}

func (c *FileFolderClient) Delete(ctx context.Context, id string) error {
	if err := validateGeneratedFilesID(id); err != nil {
		return err
	}
	return c.transport.Do(ctx, Operation{Name: "file-folder-delete", Method: http.MethodDelete, Path: fileFoldersPath() + "/" + url.PathEscape(id), Replay: ReplayNever, Timeout: filesRequestTimeout}, nil, nil)
}

func (c *FileClient) Search(ctx context.Context, folderID *string, name string) ([]ManagedFile, error) {
	query := url.Values{"limit": []string{"100"}}
	if folderID != nil {
		query.Set("parentFolderId", *folderID)
	}
	setFilesSearchName(query, managedFileSearchName(name))
	return searchFilesPages[ManagedFile](ctx, c.transport, "managed-file-search", managedFilesPath()+"/search", query)
}

func (c *FileClient) Get(ctx context.Context, id string) (ManagedFile, error) {
	if err := validateGeneratedFilesID(id); err != nil {
		return ManagedFile{}, err
	}
	var out ManagedFile
	if err := c.transport.Do(ctx, Operation{Name: "managed-file-read", Method: http.MethodGet, Path: managedFilesPath() + "/" + url.PathEscape(id), Replay: ReplaySafe, Timeout: filesRequestTimeout}, nil, &out); err != nil {
		return out, err
	}
	if err := validateGeneratedFilesID(out.ID); err != nil {
		return out, fmt.Errorf("HubSpot Managed file response: %w", err)
	}
	return out, nil
}

func (c *FileClient) Upload(ctx context.Context, input FileUpload) (ManagedFile, error) {
	body, contentType, err := managedFileMultipart(input.Name, input.FolderID, input.Access, input.Bytes, true)
	if err != nil {
		return ManagedFile{}, err
	}
	var out ManagedFile
	err = c.transport.Do(ctx, Operation{Name: "managed-file-create", Method: http.MethodPost, Path: managedFilesPath(), Replay: ReplayNever, ContentType: contentType, Timeout: filesTransferTimeout}, bytes.NewReader(body), &out)
	if err != nil {
		return out, err
	}
	if err := validateGeneratedFilesID(out.ID); err != nil {
		return out, fmt.Errorf("HubSpot Managed file response: %w", err)
	}
	return out, nil
}

func (c *FileClient) Update(ctx context.Context, id string, input FilePatch) (ManagedFile, error) {
	if err := validateGeneratedFilesID(id); err != nil {
		return ManagedFile{}, err
	}
	if input.Name == nil && input.FolderID == nil && input.Access == nil {
		return ManagedFile{}, errors.New("managed file patch must contain at least one managed field")
	}
	payload := input
	if input.Name != nil {
		name := managedFileSearchName(*input.Name)
		payload.Name = &name
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return ManagedFile{}, err
	}
	var out ManagedFile
	err = c.transport.Do(ctx, Operation{Name: "managed-file-update", Method: http.MethodPatch, Path: managedFilesPath() + "/" + url.PathEscape(id), Replay: ReplayNever, Timeout: filesRequestTimeout}, bytes.NewReader(body), &out)
	if err != nil {
		return out, err
	}
	return out, nil
}

func (c *FileClient) Replace(ctx context.Context, id string, input FileReplacement) (ManagedFile, error) {
	if err := validateGeneratedFilesID(id); err != nil {
		return ManagedFile{}, err
	}
	body, contentType, err := managedFileMultipart(input.Name, "", input.Access, input.Bytes, false)
	if err != nil {
		return ManagedFile{}, err
	}
	var out ManagedFile
	err = c.transport.Do(ctx, Operation{Name: "managed-file-replace", Method: http.MethodPut, Path: managedFilesPath() + "/" + url.PathEscape(id), Replay: ReplayNever, ContentType: contentType, Timeout: filesTransferTimeout}, bytes.NewReader(body), &out)
	if err != nil {
		return out, err
	}
	return out, nil
}

func (c *FileClient) Delete(ctx context.Context, id string) error {
	if err := validateGeneratedFilesID(id); err != nil {
		return err
	}
	return c.transport.Do(ctx, Operation{Name: "managed-file-delete", Method: http.MethodDelete, Path: managedFilesPath() + "/" + url.PathEscape(id), Replay: ReplayNever, Timeout: filesRequestTimeout}, nil, nil)
}

func searchFilesPages[T any](ctx context.Context, transport *Transport, operation, route string, query url.Values) ([]T, error) {
	results := make([]T, 0)
	seen := make(map[string]struct{})
	for {
		var page filesPage[T]
		if err := transport.Do(ctx, Operation{Name: operation, Method: http.MethodGet, Path: route + "?" + query.Encode(), Replay: ReplaySafe, Timeout: filesRequestTimeout}, nil, &page); err != nil {
			return nil, err
		}
		results = append(results, page.Results...)
		next := page.Paging.Next.After
		if next == "" {
			return results, nil
		}
		if _, ok := seen[next]; ok {
			return nil, errors.New("HubSpot Files search cursor repeated")
		}
		seen[next] = struct{}{}
		query.Set("after", next)
	}
}

func managedFileMultipart(name, folderID, access string, contents []byte, create bool) ([]byte, string, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", name)
	if err != nil {
		return nil, "", err
	}
	if _, err := part.Write(contents); err != nil {
		return nil, "", err
	}
	if err := writer.WriteField("fileName", name); err != nil {
		return nil, "", err
	}
	options := map[string]string{"access": access}
	if create {
		if err := writer.WriteField("folderId", folderID); err != nil {
			return nil, "", err
		}
		options["duplicateValidationScope"] = "EXACT_FOLDER"
		options["duplicateValidationStrategy"] = "REJECT"
	}
	encoded, err := json.Marshal(options)
	if err != nil {
		return nil, "", err
	}
	if err := writer.WriteField("options", string(encoded)); err != nil {
		return nil, "", err
	}
	if err := writer.Close(); err != nil {
		return nil, "", err
	}
	return body.Bytes(), writer.FormDataContentType(), nil
}

func validateGeneratedFilesID(id string) error {
	if id == "" || id[0] == '0' {
		return errors.New("generated id must be a non-zero decimal string")
	}
	for _, character := range id {
		if character < '0' || character > '9' {
			return errors.New("generated id must be a non-zero decimal string")
		}
	}
	return nil
}

func managedFileSearchName(filename string) string {
	extension := pathpkg.Ext(filename)
	stem := strings.TrimSuffix(filename, extension)
	if stem == "" {
		return filename
	}
	return stem
}

// HubSpot rejects name filters of 20 characters or more. Omitting the filter
// keeps the paginated search valid; provider callers still exact-match results.
func setFilesSearchName(query url.Values, name string) {
	if name != "" && utf8.RuneCountInString(name) <= filesSearchNameLimit {
		query.Set("name", name)
	}
}

func canonicalManagedFilename(remoteName, remotePath string) string {
	trimmedPath := strings.TrimSuffix(remotePath, "/")
	filename := trimmedPath
	if separator := strings.LastIndex(trimmedPath, "/"); separator >= 0 {
		filename = trimmedPath[separator+1:]
	}
	if filename == "" {
		return remoteName
	}
	if remoteName == filename || remoteName == managedFileSearchName(filename) {
		return filename
	}
	return remoteName
}
