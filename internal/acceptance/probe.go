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

func (s *Session) RequireFormArchived(id string) {
	s.t.Helper()
	clients, err := s.probeClients()
	if err != nil {
		s.t.Fatalf("configure sanitized form terminal probe: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if _, err := clients.Forms.Get(ctx, id); err == nil {
		s.t.Fatal("form terminal probe found active configuration after archive")
	} else {
		var apiError *hubspot.Error
		if !errors.As(err, &apiError) || apiError.Status != 404 {
			s.t.Fatalf("verify active form absence: %s", SanitizedHubSpotError(err))
		}
	}
	form, err := clients.Forms.GetArchived(ctx, id)
	if err != nil {
		s.t.Fatalf("verify archived form terminal state: %s", SanitizedHubSpotError(err))
	}
	if form.ID != id || !form.Archived {
		s.t.Fatal("form terminal probe did not verify the exact archived generated ID")
	}
}

// MutateFormPresentation applies one supported out-of-band presentation change
// by exact generated ID. Live acceptance uses it to prove ordinary drift and
// repair without exercising destructive or unsupported discovery fixtures.
func (s *Session) MutateFormPresentation(address string) {
	s.t.Helper()
	clients, err := s.probeClients()
	if err != nil {
		s.t.Fatalf("configure sanitized form drift probe: %v", err)
	}
	id := s.OpaqueStateString(address, "id")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	current, err := clients.Forms.Get(ctx, id)
	if err != nil {
		s.t.Fatalf("read form for drift probe: %s", SanitizedHubSpotError(err))
	}
	if len(current.FieldGroups) != 1 || len(current.FieldGroups[0].Fields) != 1 {
		s.t.Fatal("form drift probe found unsupported managed structure")
	}
	current.Name = s.prefix + "external_drift"
	current.FieldGroups[0].Fields[0].Label = "Out-of-band email"
	current.Configuration.PostSubmitAction.Value = "Out-of-band thank you"
	current.DisplayOptions.SubmitButtonText = "Out-of-band submit"
	updated, err := clients.Forms.Update(ctx, id, hubspot.FormDefinitionPatch{
		Name:           &current.Name,
		FieldGroups:    &current.FieldGroups,
		Configuration:  &current.Configuration,
		DisplayOptions: &current.DisplayOptions,
	})
	if err != nil {
		s.t.Fatalf("mutate form presentation for drift probe: %s", SanitizedHubSpotError(err))
	}
	if len(updated.FieldGroups) != 1 || len(updated.FieldGroups[0].Fields) != 1 || updated.ID != id || updated.Name != current.Name || updated.FieldGroups[0].Fields[0].Label != "Out-of-band email" ||
		updated.Configuration.PostSubmitAction.Value != "Out-of-band thank you" || updated.DisplayOptions.SubmitButtonText != "Out-of-band submit" {
		s.t.Fatal("form drift probe did not reach the requested safe presentation")
	}
}

// ArchiveForm archives the state identity out of band and verifies the exact
// terminal tombstone before returning the opaque identity to the caller.
func (s *Session) ArchiveForm(address string) string {
	s.t.Helper()
	clients, err := s.probeClients()
	if err != nil {
		s.t.Fatalf("configure sanitized form archive probe: %v", err)
	}
	id := s.OpaqueStateString(address, "id")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err := clients.Forms.Archive(ctx, id); err != nil {
		s.t.Fatalf("archive form for recreation probe: %s", SanitizedHubSpotError(err))
	}
	s.RequireFormArchived(id)
	return id
}

// RequireFormsTerminal proves that the engine-specific owned prefix has no
// active Forms and exactly the expected retained tombstone identities.
func (s *Session) RequireFormsTerminal(prefix string, expectedIDs ...string) {
	s.t.Helper()
	clients, err := s.probeClients()
	if err != nil {
		s.t.Fatalf("configure sanitized form cleanup probe: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	active, err := clients.Forms.List(ctx, false)
	if err != nil {
		s.t.Fatalf("list active forms for terminal probe: %s", SanitizedHubSpotError(err))
	}
	for _, form := range active {
		if strings.HasPrefix(form.Name, prefix) {
			s.t.Fatal("form terminal probe found active prefix-owned configuration")
		}
	}
	archived, err := clients.Forms.List(ctx, true)
	if err != nil {
		s.t.Fatalf("list archived forms for terminal probe: %s", SanitizedHubSpotError(err))
	}
	expected := make(map[string]struct{}, len(expectedIDs))
	for _, id := range expectedIDs {
		expected[id] = struct{}{}
	}
	matched := 0
	for _, form := range archived {
		if !strings.HasPrefix(form.Name, prefix) {
			continue
		}
		if _, ok := expected[form.ID]; !ok || !form.Archived {
			s.t.Fatal("form terminal probe found an unexpected prefix-owned tombstone")
		}
		matched++
	}
	if matched != len(expected) {
		s.t.Fatal("form terminal probe did not find every exact retained tombstone identity")
	}
	for id := range expected {
		s.RequireFormArchived(id)
	}
}

// MutateManagedFileContent replaces bytes through the exact generated ID while
// preserving the provider-owned metadata. Live acceptance uses this external
// boundary to prove content drift and repair without exposing the bytes.
func (s *Session) MutateManagedFileContent(address string, contents []byte) {
	s.t.Helper()
	clients, err := s.probeClients()
	if err != nil {
		s.t.Fatalf("configure sanitized Managed file drift probe: %v", err)
	}
	id := s.OpaqueStateString(address, "id")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	current, err := clients.Files.Get(ctx, id)
	if err != nil {
		s.t.Fatalf("read Managed file for content drift probe: %s", SanitizedHubSpotError(err))
	}
	if _, err := clients.Files.Replace(ctx, id, hubspot.FileReplacement{Name: current.Name, Access: current.Access, Bytes: contents}); err != nil {
		s.t.Fatalf("replace Managed file content for drift probe: %s", SanitizedHubSpotError(err))
	}
	updated, err := clients.Files.Get(ctx, id)
	if err != nil {
		s.t.Fatalf("verify Managed file content drift probe: %s", SanitizedHubSpotError(err))
	}
	if updated.ID != id || updated.CreatedAt != current.CreatedAt || updated.Size != int64(len(contents)) || updated.FileMD5 == current.FileMD5 {
		s.t.Fatal("Managed file content drift probe did not preserve identity and change content")
	}
}

// RequireManagedFileDuplicateRejected submits the already-owned target through
// the typed upload boundary and proves HubSpot's exact-folder REJECT behavior
// did not return or create another generated identity.
func (s *Session) RequireManagedFileDuplicateRejected(address string, contents []byte) {
	s.t.Helper()
	clients, err := s.probeClients()
	if err != nil {
		s.t.Fatalf("configure sanitized Managed file duplicate probe: %v", err)
	}
	id := s.OpaqueStateString(address, "id")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	current, err := clients.Files.Get(ctx, id)
	if err != nil {
		s.t.Fatalf("read Managed file for duplicate probe: %s", SanitizedHubSpotError(err))
	}
	before, err := clients.Files.Search(ctx, &current.FolderID, "")
	if err != nil {
		s.t.Fatalf("capture Managed files before duplicate probe: %s", SanitizedHubSpotError(err))
	}
	beforeIDs := make(map[string]struct{}, len(before))
	for _, file := range before {
		if !file.Archived {
			beforeIDs[file.ID] = struct{}{}
		}
	}
	if _, exists := beforeIDs[id]; !exists {
		s.t.Fatal("Managed file duplicate probe could not verify the owned identity before upload")
	}
	_, uploadErr := clients.Files.Upload(ctx, hubspot.FileUpload{Name: current.Name, FolderID: current.FolderID, Access: current.Access, Bytes: contents})
	var apiError *hubspot.Error
	if !errors.As(uploadErr, &apiError) || apiError.Status != 400 {
		s.t.Fatal("Managed file duplicate probe was not rejected by the exact-folder boundary")
	}
	files, err := clients.Files.Search(ctx, &current.FolderID, "")
	if err != nil {
		s.t.Fatalf("verify Managed file duplicate rejection: %s", SanitizedHubSpotError(err))
	}
	afterIDs := make(map[string]struct{}, len(files))
	for _, file := range files {
		if file.Archived {
			continue
		}
		afterIDs[file.ID] = struct{}{}
	}
	if len(afterIDs) != len(beforeIDs) {
		s.t.Fatal("Managed file duplicate rejection changed the active folder contents")
	}
	for beforeID := range beforeIDs {
		if _, exists := afterIDs[beforeID]; !exists {
			s.t.Fatal("Managed file duplicate rejection changed an active generated identity")
		}
	}
}

// DeleteManagedFileOutOfBand removes one exact active generated identity and
// verifies active absence before returning it to the lifecycle test.
func (s *Session) DeleteManagedFileOutOfBand(address string) string {
	s.t.Helper()
	clients, err := s.probeClients()
	if err != nil {
		s.t.Fatalf("configure sanitized Managed file disappearance probe: %v", err)
	}
	id := s.OpaqueStateString(address, "id")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	deleteErr := clients.Files.Delete(ctx, id)
	_, readErr := clients.Files.Get(ctx, id)
	if isFilesNotFound(readErr) {
		return id
	}
	if deleteErr != nil {
		s.t.Fatalf("delete Managed file for disappearance probe: %s", SanitizedHubSpotError(deleteErr))
	}
	if readErr != nil {
		s.t.Fatalf("verify Managed file disappearance probe: %s", SanitizedHubSpotError(readErr))
	}
	s.t.Fatal("Managed file disappearance probe did not verify active absence")
	return ""
}

// RequireFilesTerminal verifies exact-ID active absence and zero active
// prefix-owned Files configuration. HubSpot-managed Trash retention is not
// treated as a failure because the active Files API is the lifecycle boundary.
func (s *Session) RequireFilesTerminal(prefix string, fileIDs, folderIDs []string) (int, int) {
	s.t.Helper()
	clients, err := s.probeClients()
	if err != nil {
		s.t.Fatalf("configure sanitized Files cleanup probe: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	activeFiles, activeFolders, err := CountActiveFilesConfiguration(ctx, clients, prefix)
	if err != nil {
		s.t.Fatalf("search Files configuration for terminal probe: %s", SanitizedHubSpotError(err))
	}
	if activeFiles != 0 || activeFolders != 0 {
		s.t.Fatal("Files terminal probe found active prefix-owned configuration")
	}
	for _, id := range fileIDs {
		if _, err := clients.Files.Get(ctx, id); !isFilesNotFound(err) {
			if err != nil {
				s.t.Fatalf("verify exact Managed file active absence: %s", SanitizedHubSpotError(err))
			}
			s.t.Fatal("Files terminal probe found an expected Managed file identity active")
		}
	}
	for _, id := range folderIDs {
		if _, err := clients.FileFolders.Get(ctx, id); !isFilesNotFound(err) {
			if err != nil {
				s.t.Fatalf("verify exact File folder active absence: %s", SanitizedHubSpotError(err))
			}
			s.t.Fatal("Files terminal probe found an expected File folder identity active")
		}
	}
	return activeFiles, activeFolders
}

// CountActiveFilesConfiguration reports active prefix-owned Managed files and
// File folders through the same paginated search boundary used by terminal and
// janitor verification.
func CountActiveFilesConfiguration(ctx context.Context, clients *hubspot.ClientSet, prefix string) (int, int, error) {
	files, err := clients.Files.Search(ctx, nil, "")
	if err != nil {
		return 0, 0, err
	}
	folders, err := clients.FileFolders.Search(ctx, nil, "")
	if err != nil {
		return 0, 0, err
	}
	activeFiles := 0
	for _, file := range files {
		if !file.Archived && strings.HasPrefix(file.Name, prefix) {
			activeFiles++
		}
	}
	activeFolders := 0
	for _, folder := range folders {
		if !folder.Archived && strings.HasPrefix(folder.Name, prefix) {
			activeFolders++
		}
	}
	return activeFiles, activeFolders, nil
}

func isFilesNotFound(err error) bool {
	var apiError *hubspot.Error
	return errors.As(err, &apiError) && apiError.Status == 404
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
	if s.probeURL == "" {
		return NewRealPortalClientSet(context.Background(), accessToken, "terraform-provider-hubspot/acceptance-probe")
	}
	origin, err := url.Parse(s.probeURL)
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
