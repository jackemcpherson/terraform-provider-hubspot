// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

package acceptance

import (
	"context"
	"errors"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/jackemcpherson/terraform-provider-hubspot/internal/hubspot"
)

func (s *Session) MutatePipeline(address, label string, displayOrder int64, stageKey, stageLabel string, stageOrder int64, metadata map[string]string) {
	s.t.Helper()
	clients, err := s.probeClients()
	if err != nil {
		s.t.Fatalf("configure sanitized pipeline probe: %v", err)
	}
	objectType := s.OpaqueStateString(address, "object_type")
	remoteID := strings.TrimPrefix(s.OpaqueStateString(address, "id"), objectType+"/")
	stageIDs := s.OpaqueStateMapNestedStrings(address, "stages", "id")
	targetStageID := stageIDs[stageKey]
	if targetStageID == "" {
		s.t.Fatal("pipeline drift probe could not resolve the target stage identity")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	current, err := clients.Pipelines.Get(ctx, objectType, remoteID)
	if err != nil {
		s.t.Fatalf("read pipeline for drift probe: %s", SanitizedHubSpotError(err))
	}
	write := hubspot.PipelineWrite{Label: label, DisplayOrder: displayOrder, Stages: make([]hubspot.PipelineStageWrite, 0, len(current.Stages))}
	for _, stage := range current.Stages {
		input := hubspot.PipelineStageWrite{StageID: stage.ID, Label: stage.Label, DisplayOrder: stage.DisplayOrder, Metadata: stage.Metadata}
		if stage.ID == targetStageID {
			input.Label = stageLabel
			input.DisplayOrder = stageOrder
			input.Metadata = metadata
		}
		write.Stages = append(write.Stages, input)
	}
	if _, err := clients.Pipelines.Update(ctx, objectType, remoteID, write); err != nil {
		s.t.Fatalf("mutate pipeline for drift probe: %s", SanitizedHubSpotError(err))
	}
	verified, err := clients.Pipelines.Get(ctx, objectType, remoteID)
	if err != nil {
		s.t.Fatalf("verify pipeline drift probe: %s", SanitizedHubSpotError(err))
	}
	if verified.Label != label || verified.DisplayOrder != displayOrder {
		s.t.Fatal("pipeline drift probe did not reach the requested scalar configuration")
	}
	for _, stage := range verified.Stages {
		if stage.ID == targetStageID && stage.Label == stageLabel && stage.DisplayOrder == stageOrder {
			return
		}
	}
	s.t.Fatal("pipeline drift probe did not reach the requested stage configuration")
}

func (s *Session) ArchivePipeline(address string) {
	s.t.Helper()
	clients, err := s.probeClients()
	if err != nil {
		s.t.Fatalf("configure sanitized pipeline probe: %v", err)
	}
	objectType := s.OpaqueStateString(address, "object_type")
	remoteID := strings.TrimPrefix(s.OpaqueStateString(address, "id"), objectType+"/")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err := clients.Pipelines.Archive(ctx, objectType, remoteID); err != nil {
		s.t.Fatalf("archive pipeline for restore probe: %s", SanitizedHubSpotError(err))
	}
	archived, err := clients.Pipelines.GetArchived(ctx, objectType, remoteID)
	if err != nil {
		s.t.Fatalf("verify archived pipeline presence: %s", SanitizedHubSpotError(err))
	}
	if !archived.Archived {
		s.t.Fatal("pipeline archive probe did not verify archived CRM configuration")
	}
}

func (s *Session) CreatePipelineStageOutOfBand(address, label string, displayOrder int64, metadata map[string]string) string {
	s.t.Helper()
	clients, err := s.probeClients()
	if err != nil {
		s.t.Fatalf("configure sanitized pipeline-stage probe: %v", err)
	}
	objectType := s.OpaqueStateString(address, "object_type")
	remoteID := strings.TrimPrefix(s.OpaqueStateString(address, "id"), objectType+"/")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	current, err := clients.Pipelines.Get(ctx, objectType, remoteID)
	if err != nil {
		s.t.Fatalf("read pipeline for stage insertion probe: %s", SanitizedHubSpotError(err))
	}
	write := hubspot.PipelineWrite{Label: current.Label, DisplayOrder: current.DisplayOrder, Stages: make([]hubspot.PipelineStageWrite, 0, len(current.Stages)+1)}
	for _, stage := range current.Stages {
		write.Stages = append(write.Stages, hubspot.PipelineStageWrite{
			StageID: stage.ID, Label: stage.Label, DisplayOrder: stage.DisplayOrder, Metadata: stage.Metadata,
		})
	}
	write.Stages = append(write.Stages, hubspot.PipelineStageWrite{Label: label, DisplayOrder: displayOrder, Metadata: metadata})
	if _, err := clients.Pipelines.Update(ctx, objectType, remoteID, write); err != nil {
		s.t.Fatalf("insert out-of-band pipeline stage: %s", SanitizedHubSpotError(err))
	}
	verified, err := clients.Pipelines.Get(ctx, objectType, remoteID)
	if err != nil {
		s.t.Fatalf("verify out-of-band pipeline stage: %s", SanitizedHubSpotError(err))
	}
	for _, stage := range verified.Stages {
		if stage.Label == label && stage.DisplayOrder == displayOrder && stage.ID != "" {
			return stage.ID
		}
	}
	s.t.Fatal("out-of-band pipeline stage insertion was not verified")
	return ""
}

func (s *Session) RequirePipelineArchived(objectType, compositeID string) {
	s.t.Helper()
	clients, err := s.probeClients()
	if err != nil {
		s.t.Fatalf("configure sanitized pipeline terminal probe: %v", err)
	}
	remoteID := strings.TrimPrefix(compositeID, objectType+"/")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	pipeline, err := clients.Pipelines.GetArchived(ctx, objectType, remoteID)
	if err != nil {
		s.t.Fatalf("verify archived pipeline terminal state: %s", SanitizedHubSpotError(err))
	}
	if !pipeline.Archived || pipeline.ID != remoteID {
		s.t.Fatal("pipeline terminal probe did not verify the canonical archived identity")
	}
}

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

func (s *Session) RequirePropertyGroupReusable(objectType, name string) {
	s.t.Helper()
	clients, err := s.probeClients()
	if err != nil {
		s.t.Fatalf("configure sanitized property group probe: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	group, err := clients.PropertyGroups.Create(ctx, objectType, hubspot.PropertyGroupCreate{
		Name:         name,
		Label:        "Acceptance archive reuse probe",
		DisplayOrder: -1,
	})
	if err != nil {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cleanupCancel()
		if cleanupErr := archivePropertyGroupAndVerifyAbsent(cleanupCtx, clients, objectType, name); cleanupErr != nil {
			s.retainCleanupLedger = true
			s.t.Fatalf("verify ambiguous property group name reuse failure: create: %s; cleanup: %s", SanitizedHubSpotError(err), SanitizedHubSpotError(cleanupErr))
		}
		s.t.Fatalf("verify archived property group name reuse: %s", SanitizedHubSpotError(err))
	}
	probeActive := true
	defer func() {
		if !probeActive {
			return
		}
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cleanupCancel()
		if err := archivePropertyGroupAndVerifyAbsent(cleanupCtx, clients, objectType, name); err != nil {
			s.retainCleanupLedger = true
			s.t.Errorf("cleanup property group name reuse probe: %s", SanitizedHubSpotError(err))
		}
	}()
	if group.Name != name || group.Archived {
		s.t.Fatal("property group name reuse probe did not create the canonical active identity")
	}
	if err := archivePropertyGroupAndVerifyAbsent(ctx, clients, objectType, name); err != nil {
		s.t.Fatalf("archive property group name reuse probe: %s", SanitizedHubSpotError(err))
	}
	probeActive = false
}

func archivePropertyGroupAndVerifyAbsent(ctx context.Context, clients *hubspot.ClientSet, objectType, name string) error {
	archiveErr := clients.PropertyGroups.Archive(ctx, objectType, name)
	_, getErr := clients.PropertyGroups.Get(ctx, objectType, name)
	var apiError *hubspot.Error
	if errors.As(getErr, &apiError) && apiError.Status == 404 {
		return nil
	}
	if archiveErr != nil {
		return archiveErr
	}
	if getErr != nil {
		return getErr
	}
	return errors.New("property group name reuse probe remained active after archive")
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

func (s *Session) RequirePropertyArchived(objectType, name string) {
	s.t.Helper()
	clients, err := s.probeClients()
	if err != nil {
		s.t.Fatalf("configure sanitized property probe: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	property, err := clients.Properties.Get(ctx, objectType, name, true, "non_sensitive", "")
	if err != nil {
		s.t.Fatalf("verify archived property definition: %s", SanitizedHubSpotError(err))
	}
	if property.Archived == nil || !*property.Archived {
		s.t.Fatal("property terminal probe did not verify archived CRM configuration")
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
