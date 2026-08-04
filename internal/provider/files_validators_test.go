// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestGeneratedFilesIDValidator(t *testing.T) {
	for _, test := range []struct {
		value string
		valid bool
	}{{"1", true}, {"9007199254740993", true}, {"0", false}, {"01", false}, {"-1", false}, {"folder-1", false}, {" 1", false}} {
		response := validator.StringResponse{}
		generatedFilesIDValidator{}.ValidateString(context.Background(), validator.StringRequest{Path: path.Root("id"), ConfigValue: types.StringValue(test.value)}, &response)
		if got := !response.Diagnostics.HasError(); got != test.valid {
			t.Errorf("generated ID %q valid = %v, want %v", test.value, got, test.valid)
		}
	}
}

func TestFilesNameAndAccessValidation(t *testing.T) {
	for _, invalid := range []string{"", " name", "name ", ".", "..", "a/b", `a\\b`} {
		response := validator.StringResponse{}
		filesNameValidator{kind: "Managed file"}.ValidateString(context.Background(), validator.StringRequest{Path: path.Root("name"), ConfigValue: types.StringValue(invalid)}, &response)
		if !response.Diagnostics.HasError() {
			t.Errorf("name %q was accepted", invalid)
		}
	}
	for _, value := range []string{"PRIVATE", "PUBLIC_INDEXABLE", "PUBLIC_NOT_INDEXABLE"} {
		response := validator.StringResponse{}
		fileAccessValidator{}.ValidateString(context.Background(), validator.StringRequest{Path: path.Root("access"), ConfigValue: types.StringValue(value)}, &response)
		if response.Diagnostics.HasError() {
			t.Errorf("access %q was rejected", value)
		}
	}
}

func TestManagedFileSourceInspection(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "fixture.txt")
	if err := os.WriteFile(path, []byte("managed bytes\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	source, err := inspectManagedFileSource(path, "621be885d8bf21bc70b574e523652da9499b80a770b2921c6aea5e41c4b25342")
	if err != nil {
		t.Fatal(err)
	}
	if source.Size != 14 || source.MD5 != "30dd90bf540e4e580f130976fd354271" || string(source.Bytes) != "managed bytes\n" {
		t.Fatalf("source inspection = %#v", source)
	}
	if _, err := inspectManagedFileSource(directory, strings.Repeat("0", 64)); err == nil {
		t.Fatal("directory source was accepted")
	}
}

func TestManagedFileSourceEnforcesDigestAndDecimalFreeLimitWithoutDisclosure(t *testing.T) {
	directory := t.TempDir()
	sourcePath := filepath.Join(directory, "oversized-secret.bin")
	file, err := os.Create(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate(managedFileFreeLimit + 1); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	_, err = inspectManagedFileSource(sourcePath, strings.Repeat("0", 64))
	var sourceErr *managedFileSourceError
	if !errors.As(err, &sourceErr) || sourceErr.Kind != managedFileSourceLimitExceeded || strings.Contains(err.Error(), sourcePath) {
		t.Fatalf("oversized source error = %v", err)
	}

	digestPath := filepath.Join(directory, "digest-secret.txt")
	if err := os.WriteFile(digestPath, []byte("do-not-disclose"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = inspectManagedFileSource(digestPath, strings.Repeat("0", 64))
	if !errors.As(err, &sourceErr) || sourceErr.Kind != managedFileSourceDigestMismatch || strings.Contains(err.Error(), digestPath) || strings.Contains(err.Error(), "do-not-disclose") {
		t.Fatalf("digest source error = %v", err)
	}
}

func TestManagedFileExecutableExtensionsAreBlockedCaseInsensitively(t *testing.T) {
	for _, name := range []string{"run.sh", "RUN.EXE", "bundle.Jar", "package.rpm"} {
		if !managedFileNameBlocked(name) {
			t.Errorf("%q was not blocked", name)
		}
	}
	if managedFileNameBlocked("archive.tar.gz") || managedFileNameBlocked("readme") {
		t.Fatal("safe file name was blocked")
	}
}
