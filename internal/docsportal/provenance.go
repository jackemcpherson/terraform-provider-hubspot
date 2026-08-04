// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

package docsportal

import (
	"errors"
	"fmt"

	"github.com/jackemcpherson/terraform-provider-hubspot/internal/provenance"
)

func resolveProvenance(config *Config) error {
	if config.RequireClean && (config.ExpectedProviderCommit == "" || config.ExpectedDemoCommit == "") {
		return errors.New("clean candidate portal generation requires exact expected provider and demo commits")
	}
	if config.ProviderProvenance.Commit == "" || config.ExpectedProviderCommit != "" {
		providerProvenance, err := gitProvenance(config.ProviderRepo, config.ExpectedProviderCommit)
		if err != nil {
			return fmt.Errorf("provider provenance: %w", err)
		}
		config.ProviderProvenance = providerProvenance
	}
	if config.DemoProvenance.Commit == "" || config.ExpectedDemoCommit != "" {
		demoProvenance, err := gitProvenance(config.DemoRepo, config.ExpectedDemoCommit)
		if err != nil {
			return fmt.Errorf("demo provenance: %w", err)
		}
		config.DemoProvenance = demoProvenance
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

func gitProvenance(repo, expectedCommit string) (Provenance, error) {
	var checkout provenance.Checkout
	var err error
	if expectedCommit != "" {
		checkout, err = provenance.ValidateCheckout(repo, expectedCommit)
	} else {
		checkout, err = provenance.InspectCheckout(repo)
	}
	if err != nil {
		return Provenance{}, err
	}
	return Provenance{Commit: checkout.Commit, Timestamp: checkout.Timestamp, Dirty: checkout.Dirty}, nil
}
