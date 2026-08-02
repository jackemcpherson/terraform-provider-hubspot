// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

package docsportal

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

func resolveProvenance(config *Config) error {
	if config.ProviderProvenance.Commit == "" {
		provenance, err := gitProvenance(config.ProviderRepo)
		if err != nil {
			return fmt.Errorf("provider provenance: %w", err)
		}
		config.ProviderProvenance = provenance
	}
	if config.DemoProvenance.Commit == "" {
		provenance, err := gitProvenance(config.DemoRepo)
		if err != nil {
			return fmt.Errorf("demo provenance: %w", err)
		}
		config.DemoProvenance = provenance
	}
	if config.RequireClean && (config.ProviderProvenance.Dirty || config.DemoProvenance.Dirty) {
		return errors.New("candidate portal generation requires clean provider and demo checkouts")
	}
	if config.ExpectedProviderCommit != "" && config.ProviderProvenance.Commit != config.ExpectedProviderCommit {
		return errors.New("provider checkout does not match the candidate evidence anchor")
	}
	if config.ExpectedDemoCommit != "" && config.DemoProvenance.Commit != config.ExpectedDemoCommit {
		return errors.New("demo checkout does not match the candidate evidence anchor")
	}
	return nil
}

func gitProvenance(repo string) (Provenance, error) {
	commit, err := gitOutput(repo, "rev-parse", "HEAD")
	if err != nil {
		return Provenance{}, err
	}
	timestamp, err := gitOutput(repo, "show", "-s", "--format=%cI", "HEAD")
	if err != nil {
		return Provenance{}, err
	}
	dirty, err := gitOutput(repo, "status", "--porcelain")
	if err != nil {
		return Provenance{}, err
	}
	return Provenance{Commit: commit, Timestamp: timestamp, Dirty: dirty != ""}, nil
}

func gitOutput(repo string, arguments ...string) (string, error) {
	command := exec.Command("git", append([]string{"-C", repo}, arguments...)...)
	output, err := command.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}
