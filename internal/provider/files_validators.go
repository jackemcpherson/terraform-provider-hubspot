// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

const managedFileFreeLimit int64 = 20_000_000

var (
	generatedFilesIDPattern      = regexp.MustCompile(`^[1-9][0-9]*$`)
	managedFileSHA256Pattern     = regexp.MustCompile(`^[0-9a-f]{64}$`)
	blockedManagedFileExtensions = map[string]struct{}{
		"sh": {}, "bat": {}, "com": {}, "elf": {}, "bin": {}, "exe": {}, "jar": {}, "rpm": {}, "deb": {},
	}
)

type generatedFilesIDValidator struct{}

func (generatedFilesIDValidator) Description(context.Context) string {
	return "value must be a non-zero generated decimal HubSpot ID"
}
func (v generatedFilesIDValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}
func (generatedFilesIDValidator) ValidateString(_ context.Context, request validator.StringRequest, response *validator.StringResponse) {
	if request.ConfigValue.IsNull() || request.ConfigValue.IsUnknown() {
		return
	}
	if !generatedFilesIDPattern.MatchString(request.ConfigValue.ValueString()) {
		response.Diagnostics.AddAttributeError(request.Path, "Invalid generated Files ID", "Use one non-zero decimal HubSpot-generated ID. Names, paths, URLs, hashes, and composite identifiers are not accepted.")
	}
}

type filesNameValidator struct{ kind string }

func (v filesNameValidator) Description(context.Context) string {
	return v.kind + " name must be nonblank, unpadded, not a dot segment, and contain no path separator"
}
func (v filesNameValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}
func (v filesNameValidator) ValidateString(_ context.Context, request validator.StringRequest, response *validator.StringResponse) {
	if request.ConfigValue.IsNull() || request.ConfigValue.IsUnknown() {
		return
	}
	value := request.ConfigValue.ValueString()
	if value == "" || strings.TrimSpace(value) != value || value == "." || value == ".." || strings.ContainsAny(value, `/\\`) {
		response.Diagnostics.AddAttributeError(request.Path, "Invalid "+v.kind+" name", v.Description(context.Background())+".")
	}
}

type managedFileNameValidator struct{}

func (managedFileNameValidator) Description(context.Context) string {
	return "Managed file name must satisfy the Files name contract and must not use a documented executable extension"
}
func (v managedFileNameValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}
func (managedFileNameValidator) ValidateString(ctx context.Context, request validator.StringRequest, response *validator.StringResponse) {
	filesNameValidator{kind: "Managed file"}.ValidateString(ctx, request, response)
	if request.ConfigValue.IsNull() || request.ConfigValue.IsUnknown() || response.Diagnostics.HasError() {
		return
	}
	if managedFileNameBlocked(request.ConfigValue.ValueString()) {
		response.Diagnostics.AddAttributeError(request.Path, "Managed file type is blocked", "HubSpot Free blocks this executable filename extension. Use inert, supported content instead.")
	}
}

type fileAccessValidator struct{}

func (fileAccessValidator) Description(context.Context) string {
	return "access must be PRIVATE, PUBLIC_INDEXABLE, or PUBLIC_NOT_INDEXABLE"
}
func (v fileAccessValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}
func (fileAccessValidator) ValidateString(_ context.Context, request validator.StringRequest, response *validator.StringResponse) {
	if request.ConfigValue.IsNull() || request.ConfigValue.IsUnknown() {
		return
	}
	switch request.ConfigValue.ValueString() {
	case "PRIVATE", "PUBLIC_INDEXABLE", "PUBLIC_NOT_INDEXABLE":
	default:
		response.Diagnostics.AddAttributeError(request.Path, "Unsupported Managed file access", "Use PRIVATE, PUBLIC_INDEXABLE, or PUBLIC_NOT_INDEXABLE. Hidden and sensitive access states are outside the supported Files configuration surface.")
	}
}

type sourceSHA256Validator struct{}

func (sourceSHA256Validator) Description(context.Context) string {
	return "source_sha256 must be exactly 64 lowercase hexadecimal characters"
}
func (v sourceSHA256Validator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}
func (sourceSHA256Validator) ValidateString(_ context.Context, request validator.StringRequest, response *validator.StringResponse) {
	if request.ConfigValue.IsNull() || request.ConfigValue.IsUnknown() {
		return
	}
	if !managedFileSHA256Pattern.MatchString(request.ConfigValue.ValueString()) {
		response.Diagnostics.AddAttributeError(request.Path, "Invalid Managed file source digest", "source_sha256 must be exactly 64 lowercase hexadecimal characters.")
	}
}

type managedFileSource struct {
	Bytes  []byte
	SHA256 string
	MD5    string
	Size   int64
}

type managedFileSourceError struct {
	Kind managedFileSourceErrorKind
	Err  error
}

type managedFileSourceErrorKind uint8

const (
	managedFileSourceUnavailable managedFileSourceErrorKind = iota
	managedFileSourceLimitExceeded
	managedFileSourceDigestMismatch
)

func (e *managedFileSourceError) Error() string { return e.Err.Error() }
func (e *managedFileSourceError) Unwrap() error { return e.Err }

func inspectManagedFileSource(path, declaredSHA256 string) (managedFileSource, error) {
	if path == "" {
		return managedFileSource{}, &managedFileSourceError{Kind: managedFileSourceUnavailable, Err: errors.New("source path is empty")}
	}
	info, err := os.Stat(path)
	if err != nil {
		return managedFileSource{}, &managedFileSourceError{Kind: managedFileSourceUnavailable, Err: errors.New("source cannot be read")}
	}
	if !info.Mode().IsRegular() {
		return managedFileSource{}, &managedFileSourceError{Kind: managedFileSourceUnavailable, Err: errors.New("source does not resolve to a regular file")}
	}
	file, err := os.Open(path)
	if err != nil {
		return managedFileSource{}, &managedFileSourceError{Kind: managedFileSourceUnavailable, Err: errors.New("source cannot be read")}
	}
	defer file.Close()
	contents, err := io.ReadAll(io.LimitReader(file, managedFileFreeLimit+1))
	if err != nil {
		return managedFileSource{}, &managedFileSourceError{Kind: managedFileSourceUnavailable, Err: errors.New("source cannot be read")}
	}
	if int64(len(contents)) > managedFileFreeLimit {
		return managedFileSource{}, &managedFileSourceError{Kind: managedFileSourceLimitExceeded, Err: fmt.Errorf("source exceeds the %d-byte HubSpot Free limit", managedFileFreeLimit)}
	}
	shaDigest := sha256.Sum256(contents)
	actualSHA := hex.EncodeToString(shaDigest[:])
	if actualSHA != declaredSHA256 {
		return managedFileSource{}, &managedFileSourceError{Kind: managedFileSourceDigestMismatch, Err: errors.New("source bytes do not match source_sha256")}
	}
	md5Digest := md5.Sum(contents)
	return managedFileSource{Bytes: contents, SHA256: actualSHA, MD5: hex.EncodeToString(md5Digest[:]), Size: int64(len(contents))}, nil
}

func managedFileNameBlocked(name string) bool {
	extension := strings.TrimPrefix(strings.ToLower(filepath.Ext(name)), ".")
	_, blocked := blockedManagedFileExtensions[extension]
	return blocked
}
