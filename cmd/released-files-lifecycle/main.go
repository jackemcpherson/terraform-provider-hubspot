// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/jackemcpherson/terraform-provider-hubspot/internal/acceptance"
	"github.com/jackemcpherson/terraform-provider-hubspot/internal/hubspot"
)

var releasedPrefixPattern = regexp.MustCompile(`^tf_acc_[A-Za-z0-9_]+_$`)

func main() {
	if len(os.Args) != 6 {
		fatal(errors.New("usage: released-files-lifecycle verify-active|drift|cleanup|verify-terminal root-folder-id leaf-folder-id file-id acceptance-prefix"))
	}
	token := os.Getenv("HUBSPOT_ACCESS_TOKEN")
	if token == "" {
		fatal(errors.New("HUBSPOT_ACCESS_TOKEN is required"))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	clients, err := acceptance.NewRealPortalClientSet(ctx, token, "terraform-provider-hubspot/released-files-lifecycle")
	if err != nil {
		fatal(err)
	}
	record, err := execute(ctx, os.Args[1], os.Args[2:5], os.Args[5], clients)
	if err != nil {
		fatal(err)
	}
	if record != "" {
		fmt.Println(record)
	}
}

func execute(ctx context.Context, action string, ids []string, prefix string, clients *hubspot.ClientSet) (string, error) {
	if !releasedPrefixPattern.MatchString(prefix) {
		return "", errors.New("unsafe released Files prefix")
	}
	if len(ids) != 3 {
		return "", errors.New("three released Files generated IDs are required")
	}
	for _, id := range ids {
		if !isGeneratedFilesID(id) {
			return "", errors.New("released Files identity must be a non-zero decimal generated ID")
		}
	}
	switch action {
	case "verify-active":
		return "", verifyActive(ctx, clients, ids, prefix)
	case "drift":
		return "", drift(ctx, clients, ids, prefix)
	case "cleanup":
		return "", cleanup(ctx, clients, ids)
	case "verify-terminal":
		return verifyTerminal(ctx, clients, ids, prefix)
	default:
		return "", errors.New("action must be verify-active, drift, cleanup, or verify-terminal")
	}
}

func verifyActive(ctx context.Context, clients *hubspot.ClientSet, ids []string, prefix string) error {
	root, err := clients.FileFolders.Get(ctx, ids[0])
	if err != nil {
		return fmt.Errorf("read released root folder identity: %s", acceptance.SanitizedHubSpotError(err))
	}
	leaf, err := clients.FileFolders.Get(ctx, ids[1])
	if err != nil {
		return fmt.Errorf("read released leaf folder identity: %s", acceptance.SanitizedHubSpotError(err))
	}
	file, err := clients.Files.Get(ctx, ids[2])
	if err != nil {
		return fmt.Errorf("read released Managed file identity: %s", acceptance.SanitizedHubSpotError(err))
	}
	if root.ID != ids[0] || root.ParentFolderID != nil || !strings.HasPrefix(root.Name, prefix) || root.Archived {
		return errors.New("released root folder active identity was not exact")
	}
	if leaf.ID != ids[1] || leaf.ParentFolderID == nil || *leaf.ParentFolderID != ids[0] || !strings.HasPrefix(leaf.Name, prefix) || leaf.Archived {
		return errors.New("released leaf folder active identity was not exact")
	}
	if file.ID != ids[2] || file.FolderID != ids[1] || !strings.HasPrefix(file.Name, prefix) || file.Archived {
		return errors.New("released Managed file active identity was not exact")
	}
	folders, err := clients.FileFolders.Search(ctx, nil, "")
	if err != nil {
		return fmt.Errorf("list active released folders: %s", acceptance.SanitizedHubSpotError(err))
	}
	files, err := clients.Files.Search(ctx, nil, "")
	if err != nil {
		return fmt.Errorf("list active released files: %s", acceptance.SanitizedHubSpotError(err))
	}
	ownedFolders, ownedFiles := 0, 0
	for _, candidate := range folders {
		if strings.HasPrefix(candidate.Name, prefix) {
			ownedFolders++
			if candidate.ID != ids[0] && candidate.ID != ids[1] {
				return errors.New("released Files journey created a second folder identity")
			}
		}
	}
	for _, candidate := range files {
		if strings.HasPrefix(candidate.Name, prefix) {
			ownedFiles++
			if candidate.ID != ids[2] {
				return errors.New("released Files journey created a second Managed file identity")
			}
		}
	}
	if ownedFolders != 2 || ownedFiles != 1 {
		return errors.New("released Files journey active owned identity count was not exact")
	}
	return nil
}

func drift(ctx context.Context, clients *hubspot.ClientSet, ids []string, prefix string) error {
	if err := verifyActive(ctx, clients, ids, prefix); err != nil {
		return err
	}
	current, err := clients.Files.Get(ctx, ids[2])
	if err != nil {
		return fmt.Errorf("read released Managed file drift target: %s", acceptance.SanitizedHubSpotError(err))
	}
	name, access := prefix+"released_file_drift.txt", "PUBLIC_NOT_INDEXABLE"
	updated, err := clients.Files.Update(ctx, ids[2], hubspot.FilePatch{Name: &name, Access: &access})
	if err != nil {
		return fmt.Errorf("author released Managed file metadata drift: %s", acceptance.SanitizedHubSpotError(err))
	}
	updated, err = clients.Files.Replace(ctx, ids[2], hubspot.FileReplacement{Name: updated.Name, Access: updated.Access, Bytes: []byte("out-of-band released content\n")})
	if err != nil {
		return fmt.Errorf("author released Managed file content drift: %s", acceptance.SanitizedHubSpotError(err))
	}
	if updated.ID != ids[2] || updated.Name != name || updated.Access != access || updated.FileMD5 == current.FileMD5 || updated.CreatedAt != current.CreatedAt {
		return errors.New("released Managed file drift was not observable with preserved identity")
	}
	return verifyActive(ctx, clients, ids, prefix)
}

func cleanup(ctx context.Context, clients *hubspot.ClientSet, ids []string) error {
	if _, err := clients.Files.Get(ctx, ids[2]); err == nil {
		if err := clients.Files.Delete(ctx, ids[2]); err != nil {
			return fmt.Errorf("delete released Managed file: %s", acceptance.SanitizedHubSpotError(err))
		}
	} else if !isNotFound(err) {
		return fmt.Errorf("read released Managed file before cleanup: %s", acceptance.SanitizedHubSpotError(err))
	}
	for _, id := range []string{ids[1], ids[0]} {
		if _, err := clients.FileFolders.Get(ctx, id); err == nil {
			if err := clients.FileFolders.Delete(ctx, id); err != nil {
				return fmt.Errorf("delete released File folder leaf-first: %s", acceptance.SanitizedHubSpotError(err))
			}
		} else if !isNotFound(err) {
			return fmt.Errorf("read released File folder before cleanup: %s", acceptance.SanitizedHubSpotError(err))
		}
	}
	return nil
}

func verifyTerminal(ctx context.Context, clients *hubspot.ClientSet, ids []string, prefix string) (string, error) {
	for index, id := range ids {
		var err error
		if index < 2 {
			_, err = clients.FileFolders.Get(ctx, id)
		} else {
			_, err = clients.Files.Get(ctx, id)
		}
		if !isNotFound(err) {
			return "", errors.New("released Files identity remained active after teardown")
		}
	}
	folders, err := clients.FileFolders.Search(ctx, nil, "")
	if err != nil {
		return "", fmt.Errorf("list active released folders after teardown: %s", acceptance.SanitizedHubSpotError(err))
	}
	files, err := clients.Files.Search(ctx, nil, "")
	if err != nil {
		return "", fmt.Errorf("list active released files after teardown: %s", acceptance.SanitizedHubSpotError(err))
	}
	for _, candidate := range folders {
		if strings.HasPrefix(candidate.Name, prefix) {
			return "", errors.New("released Files teardown retained an active owned folder")
		}
	}
	for _, candidate := range files {
		if strings.HasPrefix(candidate.Name, prefix) {
			return "", errors.New("released Files teardown retained an active owned Managed file")
		}
	}
	digest := sha256.Sum256([]byte("released-files-identities\x00" + strings.Join(ids, "\x00")))
	record, err := json.Marshal(struct {
		GeneratedIdentityHash string `json:"generated_identity_hash"`
		ActiveOwnedFiles      int    `json:"active_owned_files"`
		ActiveOwnedFolders    int    `json:"active_owned_folders"`
		Cleanup               string `json:"cleanup"`
	}{hex.EncodeToString(digest[:]), 0, 0, "passed"})
	if err != nil {
		return "", errors.New("encode released Files terminal record")
	}
	return string(record), nil
}

func isGeneratedFilesID(id string) bool {
	if id == "" || id[0] == '0' {
		return false
	}
	for _, character := range id {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

func isNotFound(err error) bool {
	var apiError *hubspot.Error
	return errors.As(err, &apiError) && apiError.Status == 404
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
