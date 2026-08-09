// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

package provenance_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jackemcpherson/terraform-provider-hubspot/internal/provenance"
)

func TestValidateCheckoutAcceptsStandaloneCloneAtExactCleanCommit(t *testing.T) {
	source, commit := createRepository(t)
	clone := filepath.Join(t.TempDir(), "clone")
	runGit(t, "", "clone", "--quiet", source, clone)

	checkout, err := provenance.ValidateCheckout(clone, commit)
	if err != nil {
		t.Fatal(err)
	}
	if checkout.Commit != commit {
		t.Fatalf("commit = %q, want %q", checkout.Commit, commit)
	}
	if checkout.Timestamp == "" {
		t.Fatal("timestamp is empty")
	}
}

func TestValidateCheckoutAcceptsLinkedWorktreeAtExactCleanCommit(t *testing.T) {
	repository, commit := createRepository(t)
	worktree := filepath.Join(t.TempDir(), "worktree")
	runGit(t, repository, "worktree", "add", "--quiet", "--detach", worktree, commit)

	gitFile, err := os.Stat(filepath.Join(worktree, ".git"))
	if err != nil {
		t.Fatal(err)
	}
	if !gitFile.Mode().IsRegular() {
		t.Fatal("linked worktree fixture does not have a .git file")
	}
	checkout, err := provenance.ValidateCheckout(worktree, commit)
	if err != nil {
		t.Fatal(err)
	}
	if checkout.Commit != commit {
		t.Fatalf("commit = %q, want %q", checkout.Commit, commit)
	}
}

func TestValidateCheckoutRejectsInvalidProvenance(t *testing.T) {
	t.Run("malformed expected commit", func(t *testing.T) {
		_, err := provenance.ValidateCheckout(t.TempDir(), "abc123")
		assertErrorContains(t, err, "full 40-character commit")
	})

	t.Run("non-repository", func(t *testing.T) {
		_, err := provenance.ValidateCheckout(t.TempDir(), strings.Repeat("a", 40))
		assertErrorContains(t, err, "Git checkout")
	})

	t.Run("bare repository", func(t *testing.T) {
		source, commit := createRepository(t)
		bare := filepath.Join(t.TempDir(), "bare.git")
		runGit(t, "", "clone", "--quiet", "--bare", source, bare)
		_, err := provenance.ValidateCheckout(bare, commit)
		assertErrorContains(t, err, "attached worktree")
	})

	t.Run("subdirectory as root", func(t *testing.T) {
		repository, commit := createRepository(t)
		subdirectory := filepath.Join(repository, "nested")
		if err := os.Mkdir(subdirectory, 0o755); err != nil {
			t.Fatal(err)
		}
		_, err := provenance.ValidateCheckout(subdirectory, commit)
		assertErrorContains(t, err, "checkout root")
	})

	t.Run("wrong commit", func(t *testing.T) {
		repository, _ := createRepository(t)
		_, err := provenance.ValidateCheckout(repository, strings.Repeat("a", 40))
		assertErrorContains(t, err, "HEAD does not match")
	})

	t.Run("dirty tracked file", func(t *testing.T) {
		repository, commit := createRepository(t)
		if err := os.WriteFile(filepath.Join(repository, "tracked.txt"), []byte("changed\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		_, err := provenance.ValidateCheckout(repository, commit)
		assertErrorContains(t, err, "tracked or untracked changes")
	})

	t.Run("dirty untracked file", func(t *testing.T) {
		repository, commit := createRepository(t)
		if err := os.WriteFile(filepath.Join(repository, "untracked.txt"), []byte("new\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		_, err := provenance.ValidateCheckout(repository, commit)
		assertErrorContains(t, err, "tracked or untracked changes")
	})

	t.Run("malformed Git indirection", func(t *testing.T) {
		root := t.TempDir()
		if err := os.WriteFile(filepath.Join(root, ".git"), []byte("gitdir: missing\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		_, err := provenance.ValidateCheckout(root, strings.Repeat("a", 40))
		assertErrorContains(t, err, "Git checkout")
	})

	t.Run("failed Git plumbing command", func(t *testing.T) {
		repository, commit := createRepository(t)
		if err := os.WriteFile(filepath.Join(repository, ".git", "HEAD"), []byte("not-a-ref\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		_, err := provenance.ValidateCheckout(repository, commit)
		assertErrorContains(t, err, "git rev-parse --is-inside-work-tree failed")
	})
}

func createRepository(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	runGit(t, root, "init", "--quiet")
	runGit(t, root, "config", "user.name", "Provenance Test")
	runGit(t, root, "config", "user.email", "provenance@example.com")
	if err := os.WriteFile(filepath.Join(root, "tracked.txt"), []byte("original\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", "tracked.txt")
	runGit(t, root, "commit", "--quiet", "-m", "fixture")
	return root, runGit(t, root, "rev-parse", "HEAD")
}

func runGit(t *testing.T, repository string, arguments ...string) string {
	t.Helper()
	commandArguments := arguments
	if repository != "" {
		commandArguments = append([]string{"-C", repository}, arguments...)
	}
	output, err := exec.Command("git", commandArguments...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(commandArguments, " "), err, output)
	}
	return strings.TrimSpace(string(output))
}

func assertErrorContains(t *testing.T, err error, expected string) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), expected) {
		t.Fatalf("error = %v, want %q", err, expected)
	}
}
