// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

package acceptance

import (
	"context"
	"errors"
	"net/url"
	"os"
	"time"

	"github.com/jackemcpherson/terraform-provider-hubspot/internal/hubspot"
)

func (s *Session) MutatePropertyGroupLabel(objectType, name, label string) {
	s.MutatePropertyGroup(objectType, name, label, nil)
}

func (s *Session) MutatePropertyGroup(objectType, name, label string, displayOrder *int64) {
	s.t.Helper()
	clients, err := s.probeClients()
	if err != nil {
		s.t.Fatalf("configure sanitized property group probe: %v", err)
	}
	client := clients.PropertyGroups
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	current, err := client.Get(ctx, objectType, name)
	if err != nil {
		s.t.Fatalf("read property group for drift probe: %s", SanitizedHubSpotError(err))
	}
	order := current.DisplayOrder
	if displayOrder != nil {
		order = *displayOrder
	}
	if _, err := client.Update(ctx, objectType, name, hubspot.PropertyGroupUpdate{
		Label:        label,
		DisplayOrder: order,
	}); err != nil {
		s.t.Fatalf("mutate property group for drift probe: %s", SanitizedHubSpotError(err))
	}
	verified, err := client.Get(ctx, objectType, name)
	if err != nil {
		s.t.Fatalf("verify property group drift probe: %s", SanitizedHubSpotError(err))
	}
	if verified.Label != label || verified.DisplayOrder != order {
		s.t.Fatal("property group drift probe did not reach the requested safe configuration")
	}
}

func (s *Session) ArchivePropertyGroup(objectType, name string) {
	s.t.Helper()
	clients, err := s.probeClients()
	if err != nil {
		s.t.Fatalf("configure sanitized property group probe: %v", err)
	}
	client := clients.PropertyGroups
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	if err := client.Archive(ctx, objectType, name); err != nil {
		s.t.Fatalf("archive property group for absence probe: %s", SanitizedHubSpotError(err))
	}
	if _, err := client.Get(ctx, objectType, name); err == nil {
		s.t.Fatal("property group absence probe found active CRM configuration after archive")
	} else {
		var apiError *hubspot.Error
		if !errors.As(err, &apiError) || apiError.Status != 404 {
			s.t.Fatalf("verify property group absence probe: %s", SanitizedHubSpotError(err))
		}
	}
}

func (s *Session) RequirePropertyGroupAbsent(objectType, name string) {
	s.t.Helper()
	clients, err := s.probeClients()
	if err != nil {
		s.t.Fatalf("configure sanitized property group probe: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if _, err := clients.PropertyGroups.Get(ctx, objectType, name); err == nil {
		s.t.Fatal("property group absence probe found active CRM configuration")
	} else {
		var apiError *hubspot.Error
		if !errors.As(err, &apiError) || apiError.Status != 404 {
			s.t.Fatalf("verify property group absence: %s", SanitizedHubSpotError(err))
		}
	}
}

func (s *Session) MutatePropertyLabel(objectType, name, label string) {
	s.t.Helper()
	clients, err := s.probeClients()
	if err != nil {
		s.t.Fatalf("configure sanitized property probe: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	current, err := clients.Properties.Get(ctx, objectType, name, false, "non_sensitive", "")
	if err != nil {
		s.t.Fatalf("read property definition for drift probe: %s", SanitizedHubSpotError(err))
	}
	_, err = clients.Properties.Update(ctx, objectType, name, hubspot.PropertyWrite{
		Name:                 current.Name,
		Label:                label,
		GroupName:            current.GroupName,
		Type:                 current.Type,
		FieldType:            current.FieldType,
		Description:          current.Description,
		DisplayOrder:         current.DisplayOrder,
		FormField:            current.FormField,
		Hidden:               current.Hidden,
		HasUniqueValue:       current.HasUniqueValue,
		DataSensitivity:      current.DataSensitivity,
		ExternalOptions:      current.ExternalOptions,
		ShowCurrencySymbol:   current.ShowCurrencySymbol,
		CalculationFormula:   current.CalculationFormula,
		CurrencyPropertyName: current.CurrencyPropertyName,
		NumberDisplayHint:    current.NumberDisplayHint,
		TextDisplayHint:      current.TextDisplayHint,
		ReferencedObjectType: current.ReferencedObjectType,
		Options:              current.Options,
	})
	if err != nil {
		s.t.Fatalf("mutate property definition for drift probe: %s", SanitizedHubSpotError(err))
	}
	verified, err := clients.Properties.Get(ctx, objectType, name, false, "non_sensitive", "")
	if err != nil {
		s.t.Fatalf("verify property definition drift probe: %s", SanitizedHubSpotError(err))
	}
	if verified.Label != label {
		s.t.Fatal("property definition drift probe did not reach the requested safe configuration")
	}
}

func (s *Session) ArchiveProperty(objectType, name string) {
	s.t.Helper()
	clients, err := s.probeClients()
	if err != nil {
		s.t.Fatalf("configure sanitized property probe: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	if err := clients.Properties.Archive(ctx, objectType, name); err != nil {
		s.t.Fatalf("archive property definition for absence probe: %s", SanitizedHubSpotError(err))
	}
	if _, err := clients.Properties.Get(ctx, objectType, name, false, "non_sensitive", ""); err == nil {
		s.t.Fatal("property definition absence probe found active CRM configuration after archive")
	} else {
		var apiError *hubspot.Error
		if !errors.As(err, &apiError) || apiError.Status != 404 {
			s.t.Fatalf("verify active property definition absence: %s", SanitizedHubSpotError(err))
		}
	}
	archived, err := clients.Properties.Get(ctx, objectType, name, true, "non_sensitive", "")
	if err != nil {
		s.t.Fatalf("verify archived property definition presence: %s", SanitizedHubSpotError(err))
	}
	if archived.Archived == nil || !*archived.Archived {
		s.t.Fatal("property definition archive probe did not verify archived CRM configuration")
	}
}

func (s *Session) RequirePropertyAbsent(objectType, name string) {
	s.t.Helper()
	clients, err := s.probeClients()
	if err != nil {
		s.t.Fatalf("configure sanitized property probe: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if _, err := clients.Properties.Get(ctx, objectType, name, false, "non_sensitive", ""); err == nil {
		s.t.Fatal("property definition absence probe found active CRM configuration")
	} else {
		var apiError *hubspot.Error
		if !errors.As(err, &apiError) || apiError.Status != 404 {
			s.t.Fatalf("verify property definition absence: %s", SanitizedHubSpotError(err))
		}
	}
}

func (s *Session) probeClients() (*hubspot.ClientSet, error) {
	accessToken := os.Getenv("HUBSPOT_ACCESS_TOKEN")
	if accessToken == "" {
		return nil, errors.New("HUBSPOT_ACCESS_TOKEN is required")
	}
	baseURL := s.probeURL
	if baseURL == "" {
		baseURL = "https://api.hubapi.com"
	}
	origin, err := url.Parse(baseURL)
	if err != nil {
		return nil, errors.New("invalid HubSpot probe origin")
	}
	clients, err := hubspot.NewClientSet(hubspot.TransportConfig{
		BaseURL:     origin,
		AccessToken: accessToken,
		UserAgent:   "terraform-provider-hubspot/acceptance-probe",
	})
	if err != nil {
		return nil, errors.New("invalid HubSpot probe configuration")
	}
	return clients, nil
}
