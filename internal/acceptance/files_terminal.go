// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

package acceptance

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackemcpherson/terraform-provider-hubspot/internal/hubspot"
)

// VerifyFilesTerminal proves exact generated IDs and all prefix-owned Files
// configuration are inactive, then returns sanitized hashed evidence.
func VerifyFilesTerminal(ctx context.Context, clients *hubspot.ClientSet, folderIDs, fileIDs []string, ownedPrefix, digestDomain string) (string, error) {
	for _, id := range folderIDs {
		_, err := clients.FileFolders.Get(ctx, id)
		if !filesTerminalNotFound(err) {
			return "", errors.New("file folder identity remained active after teardown")
		}
	}
	for _, id := range fileIDs {
		_, err := clients.Files.Get(ctx, id)
		if !filesTerminalNotFound(err) {
			return "", errors.New("managed file identity remained active after teardown")
		}
	}
	folders, err := clients.FileFolders.Search(ctx, nil, "")
	if err != nil {
		return "", fmt.Errorf("list active File folders after teardown: %s", SanitizedHubSpotError(err))
	}
	files, err := clients.Files.Search(ctx, nil, "")
	if err != nil {
		return "", fmt.Errorf("list active Managed files after teardown: %s", SanitizedHubSpotError(err))
	}
	for _, folder := range folders {
		if strings.HasPrefix(folder.Name, ownedPrefix) {
			return "", errors.New("files teardown retained an active owned folder")
		}
	}
	for _, file := range files {
		if strings.HasPrefix(file.Name, ownedPrefix) {
			return "", errors.New("files teardown retained an active owned Managed file")
		}
	}
	identities := append(append(make([]string, 0, len(folderIDs)+len(fileIDs)), folderIDs...), fileIDs...)
	digest := sha256.Sum256([]byte(digestDomain + "\x00" + strings.Join(identities, "\x00")))
	record, err := json.Marshal(struct {
		GeneratedIdentityHash string `json:"generated_identity_hash"`
		ActiveOwnedFiles      int    `json:"active_owned_files"`
		ActiveOwnedFolders    int    `json:"active_owned_folders"`
		Cleanup               string `json:"cleanup"`
	}{hex.EncodeToString(digest[:]), 0, 0, "passed"})
	if err != nil {
		return "", errors.New("encode Files terminal record")
	}
	return string(record), nil
}

func filesTerminalNotFound(err error) bool {
	var apiError *hubspot.Error
	return errors.As(err, &apiError) && apiError.Status == 404
}
