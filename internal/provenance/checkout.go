// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

// Package provenance verifies that evidence originates from an exact Git
// checkout rather than inferring repository identity from .git filesystem shape.
package provenance

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

var fullCommitPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)

// Checkout describes the Git observations used to bind generated evidence to
// source.
type Checkout struct {
	Commit    string
	Timestamp string
	Dirty     bool
}

// ValidateCheckout proves that root is an attached, non-bare checkout at the
// exact expected commit with no tracked or untracked changes.
func ValidateCheckout(root, expectedCommit string) (Checkout, error) {
	if !fullCommitPattern.MatchString(expectedCommit) {
		return Checkout{}, fmt.Errorf("expected commit must be a full 40-character commit")
	}
	checkout, err := InspectCheckout(root)
	if err != nil {
		return Checkout{}, err
	}
	if checkout.Commit != expectedCommit {
		return Checkout{}, fmt.Errorf("checkout HEAD does not match expected commit")
	}
	if checkout.Dirty {
		return Checkout{}, fmt.Errorf("checkout has tracked or untracked changes")
	}
	return checkout, nil
}

// InspectCheckout reads provenance from a checkout after proving root is the
// attached worktree's top level. Both standalone clones and linked worktrees
// satisfy this contract.
func InspectCheckout(root string) (Checkout, error) {
	canonicalRoot, err := canonicalPath(root)
	if err != nil {
		return Checkout{}, fmt.Errorf("resolve checkout root: %w", err)
	}
	insideWorktree, err := gitOutput(root, "rev-parse", "--is-inside-work-tree")
	if err != nil {
		return Checkout{}, fmt.Errorf("path is not a Git checkout: %w", err)
	}
	bare, err := gitOutput(root, "rev-parse", "--is-bare-repository")
	if err != nil {
		return Checkout{}, fmt.Errorf("inspect bare repository state: %w", err)
	}
	if insideWorktree != "true" || bare != "false" {
		return Checkout{}, fmt.Errorf("git repository does not have an attached worktree")
	}
	topLevel, err := gitOutput(root, "rev-parse", "--show-toplevel")
	if err != nil {
		return Checkout{}, fmt.Errorf("resolve Git checkout root: %w", err)
	}
	canonicalTopLevel, err := canonicalPath(topLevel)
	if err != nil {
		return Checkout{}, fmt.Errorf("resolve reported Git checkout root: %w", err)
	}
	if canonicalRoot != canonicalTopLevel {
		return Checkout{}, fmt.Errorf("supplied path is not the Git checkout root")
	}
	commit, err := gitOutput(root, "rev-parse", "--verify", "HEAD^{commit}")
	if err != nil {
		return Checkout{}, fmt.Errorf("resolve exact checkout HEAD: %w", err)
	}
	timestamp, err := gitOutput(root, "show", "-s", "--format=%cI", "HEAD")
	if err != nil {
		return Checkout{}, fmt.Errorf("resolve checkout commit timestamp: %w", err)
	}
	status, err := gitOutput(root, "status", "--porcelain=v1", "--untracked-files=all")
	if err != nil {
		return Checkout{}, fmt.Errorf("inspect checkout cleanliness: %w", err)
	}
	return Checkout{Commit: commit, Timestamp: timestamp, Dirty: status != ""}, nil
}

func canonicalPath(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(absolute)
}

func gitOutput(root string, arguments ...string) (string, error) {
	command := exec.Command("git", append([]string{"-C", root}, arguments...)...)
	output, err := command.CombinedOutput()
	if err != nil {
		detail := strings.TrimSpace(string(output))
		if detail == "" {
			detail = err.Error()
		}
		return "", fmt.Errorf("git %s failed: %s", strings.Join(arguments, " "), detail)
	}
	return strings.TrimSpace(string(output)), nil
}
