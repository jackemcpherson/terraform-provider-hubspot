// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/jackemcpherson/terraform-provider-hubspot/internal/hubspot"
)

type FileFolderResource struct {
	folders *hubspot.FileFolderClient
	files   *hubspot.FileClient
}

var fileFolderUpdateMutex sync.Mutex

var errFileFolderReadBackDidNotConverge = errors.New("file folder read-back did not converge")

var (
	_ resource.ResourceWithImportState = (*FileFolderResource)(nil)
	_ resource.ResourceWithModifyPlan  = (*FileFolderResource)(nil)
)

type fileFolderResourceModel struct {
	ID             types.String `tfsdk:"id"`
	Name           types.String `tfsdk:"name"`
	ParentFolderID types.String `tfsdk:"parent_folder_id"`
	Path           types.String `tfsdk:"path"`
	CreatedAt      types.String `tfsdk:"created_at"`
	UpdatedAt      types.String `tfsdk:"updated_at"`
}

func NewFileFolderResource() resource.Resource { return &FileFolderResource{} }

func (r *FileFolderResource) Metadata(_ context.Context, _ resource.MetadataRequest, response *resource.MetadataResponse) {
	response.TypeName = "hubspot_file_folder"
}

func (r *FileFolderResource) Schema(_ context.Context, _ resource.SchemaRequest, response *resource.SchemaResponse) {
	response.Schema = schema.Schema{
		Version:             0,
		Description:         "Manages one explicit HubSpot File folder by its generated ID.",
		MarkdownDescription: "Manages one explicit HubSpot File folder by its generated `id`.",
		Attributes: map[string]schema.Attribute{
			"id":               schema.StringAttribute{Computed: true, Description: "HubSpot-generated folder ID used as state and import identity.", MarkdownDescription: "HubSpot-generated folder `id` used as the only state and import identity.", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"name":             schema.StringAttribute{Required: true, Description: "File folder name within its direct parent.", MarkdownDescription: "File folder name within its direct parent. It is mutable presentation, not identity.", Validators: []validator.String{filesNameValidator{kind: "File folder"}}},
			"parent_folder_id": schema.StringAttribute{Optional: true, Description: "Generated direct parent folder ID; null means File Manager root.", MarkdownDescription: "Generated direct parent folder `id`. `null` means File Manager root.", Validators: []validator.String{generatedFilesIDValidator{}}},
			"path":             schema.StringAttribute{Computed: true, Description: "HubSpot-derived current folder path.", MarkdownDescription: "HubSpot-derived current folder path. It is an observation, never identity."},
			"created_at":       schema.StringAttribute{Computed: true, Description: "RFC 3339 creation timestamp.", MarkdownDescription: "HubSpot RFC 3339 creation observation retained through in-place updates.", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"updated_at":       schema.StringAttribute{Computed: true, Description: "RFC 3339 update timestamp.", MarkdownDescription: "Current HubSpot RFC 3339 update observation."},
		},
	}
}

func (r *FileFolderResource) Configure(_ context.Context, request resource.ConfigureRequest, response *resource.ConfigureResponse) {
	if request.ProviderData == nil {
		return
	}
	clients, ok := request.ProviderData.(*hubspot.ClientSet)
	if !ok || clients == nil || clients.FileFolders == nil || clients.Files == nil {
		response.Diagnostics.AddError("Provider is not configured", "The HubSpot Files clients were not available to hubspot_file_folder.")
		return
	}
	r.folders = clients.FileFolders
	r.files = clients.Files
}

func (r *FileFolderResource) ModifyPlan(ctx context.Context, request resource.ModifyPlanRequest, response *resource.ModifyPlanResponse) {
	if request.Plan.Raw.IsNull() || request.State.Raw.IsNull() {
		return
	}
	var state, plan fileFolderResourceModel
	response.Diagnostics.Append(request.State.Get(ctx, &state)...)
	response.Diagnostics.Append(request.Plan.Get(ctx, &plan)...)
	if response.Diagnostics.HasError() {
		return
	}
	if knownStringValueChanged(state.Name, plan.Name) || knownStringValueChanged(state.ParentFolderID, plan.ParentFolderID) {
		plan.Path = types.StringUnknown()
		plan.UpdatedAt = types.StringUnknown()
		response.Diagnostics.Append(response.Plan.Set(ctx, &plan)...)
	}
}

func (r *FileFolderResource) Create(ctx context.Context, request resource.CreateRequest, response *resource.CreateResponse) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	var plan fileFolderResourceModel
	response.Diagnostics.Append(request.Plan.Get(ctx, &plan)...)
	if response.Diagnostics.HasError() {
		return
	}
	parent := nullableStringPointer(plan.ParentFolderID)
	collision, err := r.folderCollision(ctx, parent, plan.Name.ValueString(), "")
	if err != nil {
		appendHubSpotDiagnostic(&response.Diagnostics, "File folder collision preflight failed", err)
		return
	}
	if collision {
		response.Diagnostics.AddError("File folder collision", "An active File folder already uses the requested name under the exact parent. Import a confirmed generated ID explicitly or choose a different name; no folder was created.")
		return
	}

	created, createErr := r.folders.Create(ctx, hubspot.FileFolderWrite{Name: plan.Name.ValueString(), ParentFolderID: parent})
	if generatedFilesIDPattern.MatchString(created.ID) {
		recovery := plan
		recovery.ID = types.StringValue(created.ID)
		response.Diagnostics.Append(response.State.Set(ctx, &recovery)...)
		if response.Diagnostics.HasError() {
			return
		}
	}
	if created.ID == "" {
		if createErr != nil && !isAmbiguous(createErr) {
			appendHubSpotDiagnostic(&response.Diagnostics, "Unable to create File folder", createErr)
		} else {
			response.Diagnostics.AddError("File folder creation outcome is unknown", "HubSpot did not return a generated folder ID. Inspect the exact intended parent safely and import only a confirmed generated ID, or remove any residual before retrying. The provider did not search by name for adoption.")
		}
		return
	}
	if !generatedFilesIDPattern.MatchString(created.ID) {
		response.Diagnostics.AddError("File folder creation identity is invalid", "HubSpot returned a non-canonical generated folder ID. The provider did not search by name for adoption.")
		return
	}
	verified, verifyErr := r.folders.Get(ctx, created.ID)
	if verifyErr != nil {
		response.Diagnostics.AddError("File folder creation outcome requires recovery", "HubSpot returned generated folder ID "+created.ID+", but exact-ID read-back failed. The ID was retained in state; retry refresh or import that exact ID.")
		return
	}
	model, diagnostics := folderModelFromRemote(verified)
	response.Diagnostics.Append(diagnostics...)
	if response.Diagnostics.HasError() || !folderMatchesPlan(verified, created.ID, plan) {
		if !response.Diagnostics.HasError() {
			response.Diagnostics.AddError("File folder creation was not verified", "Exact-ID read-back did not match the requested name, parent, and canonical path. The generated ID was retained for exact recovery; no name-based adoption occurred.")
		}
		return
	}
	if createErr != nil {
		response.Diagnostics.AddWarning("Create response was ambiguous", "Exact generated-ID read-back matched the planned File folder, so creation converged without replaying the create request.")
	}
	response.Diagnostics.Append(response.State.Set(ctx, &model)...)
}

func (r *FileFolderResource) Read(ctx context.Context, request resource.ReadRequest, response *resource.ReadResponse) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	var state fileFolderResourceModel
	response.Diagnostics.Append(request.State.Get(ctx, &state)...)
	if response.Diagnostics.HasError() {
		return
	}
	folder, stale, err := r.waitForCurrentFolderRevision(ctx, state)
	if err != nil {
		if isNotFound(err) {
			response.State.RemoveResource(ctx)
			return
		}
		appendHubSpotDiagnostic(&response.Diagnostics, "File folder refresh failed", err)
		return
	}
	if stale {
		response.Diagnostics.AddWarning("File folder refresh returned a stale snapshot", "HubSpot returned an older revision for the same generated folder ID. The newer verified state was retained for this refresh.")
		return
	}
	if folder.ID != state.ID.ValueString() || folder.Archived {
		response.Diagnostics.AddError("File folder refresh was not verified", "HubSpot did not return the same active generated folder ID. Prior state was retained.")
		return
	}
	model, diagnostics := folderModelFromRemote(folder)
	response.Diagnostics.Append(diagnostics...)
	if !response.Diagnostics.HasError() {
		response.Diagnostics.Append(response.State.Set(ctx, &model)...)
	}
}

func (r *FileFolderResource) Update(ctx context.Context, request resource.UpdateRequest, response *resource.UpdateResponse) {
	fileFolderUpdateMutex.Lock()
	defer fileFolderUpdateMutex.Unlock()
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	var state, plan fileFolderResourceModel
	response.Diagnostics.Append(request.State.Get(ctx, &state)...)
	response.Diagnostics.Append(request.Plan.Get(ctx, &plan)...)
	if response.Diagnostics.HasError() {
		return
	}
	renameHasChildFolders := false
	if state.Name.ValueString() != plan.Name.ValueString() {
		id := state.ID.ValueString()
		childFiles, err := r.files.Search(ctx, &id, "")
		if err != nil {
			appendHubSpotDiagnostic(&response.Diagnostics, "File folder rename preflight failed", err)
			return
		}
		if hasActiveFileChildren(childFiles, state.ID.ValueString()) {
			response.Diagnostics.AddError("File folder rename requires no direct files", "HubSpot reports success but reverts a rename when the folder contains direct Managed files. Move or remove those files first, then rename the folder and move the files back in a later apply.")
			return
		}
		childFolders, err := r.folders.Search(ctx, &id, "")
		if err != nil {
			appendHubSpotDiagnostic(&response.Diagnostics, "File folder rename preflight failed", err)
			return
		}
		renameHasChildFolders = hasActiveFolderChildren(childFolders, id)
	}
	parent := nullableStringPointer(plan.ParentFolderID)
	collision, err := r.folderCollision(ctx, parent, plan.Name.ValueString(), state.ID.ValueString())
	if err != nil {
		appendHubSpotDiagnostic(&response.Diagnostics, "File folder collision preflight failed", err)
		return
	}
	if collision {
		response.Diagnostics.AddError("File folder collision", "Another active File folder already uses the requested target name under the exact parent. No update was sent and prior state was retained.")
		return
	}
	parentChanged := !nullableStringsEqual(nullableStringPointer(state.ParentFolderID), parent)
	if !parentChanged {
		if err := r.waitForCurrentParentPath(ctx, state.ID.ValueString(), parent); err != nil {
			response.Diagnostics.AddError("File folder update was not verified", "The current child folder path did not converge with its exact parent before rename. Prior identity and state were retained for a safe retry.")
			return
		}
	}
	var updateErr error
	var taskResult *hubspot.FileFolder
	if parentChanged || renameHasChildFolders {
		task, err := r.folders.Update(ctx, state.ID.ValueString(), hubspot.FileFolderWrite{Name: plan.Name.ValueString(), ParentFolderID: parent})
		if err != nil {
			appendHubSpotDiagnostic(&response.Diagnostics, "File folder update did not complete", err)
			return
		}
		result, err := r.waitForFolderTask(ctx, task.ID)
		if err != nil {
			response.Diagnostics.AddError("File folder update did not complete", "HubSpot did not report a valid terminal COMPLETE task. Prior identity and state were retained for a safe retry.")
			return
		}
		if len(folderPlanMismatches(result, state.ID.ValueString(), plan, state.CreatedAt.ValueString())) == 0 {
			taskResult = &result
		}
	} else {
		_, updateErr = r.folders.Rename(ctx, state.ID.ValueString(), plan.Name.ValueString())
	}
	verified, err := r.waitForFolderPlan(ctx, state.ID.ValueString(), plan, state.CreatedAt.ValueString())
	if errors.Is(err, errFileFolderReadBackDidNotConverge) && taskResult != nil {
		verified = *taskResult
		err = nil
	}
	if err != nil {
		detail := "Exact-ID read-back did not match every planned managed value. Prior identity and state were retained for a safe retry."
		if mismatches := folderPlanMismatches(verified, state.ID.ValueString(), plan, state.CreatedAt.ValueString()); len(mismatches) > 0 {
			detail += " Mismatched fields: " + strings.Join(mismatches, ", ") + "."
		}
		response.Diagnostics.AddError("File folder update was not verified", detail)
		return
	}
	if updateErr != nil {
		if !isAmbiguous(updateErr) {
			appendHubSpotDiagnostic(&response.Diagnostics, "File folder update was not verified", updateErr)
			return
		}
		response.Diagnostics.AddWarning("Folder PATCH response was ambiguous", "Exact generated-ID read-back proved the planned name, parent, path, and creation time, so the rename converged without replaying PATCH.")
	}
	model, diagnostics := folderModelFromRemote(verified)
	response.Diagnostics.Append(diagnostics...)
	if !response.Diagnostics.HasError() {
		response.Diagnostics.Append(response.State.Set(ctx, &model)...)
	}
}

func (r *FileFolderResource) Delete(ctx context.Context, request resource.DeleteRequest, response *resource.DeleteResponse) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	var state fileFolderResourceModel
	response.Diagnostics.Append(request.State.Get(ctx, &state)...)
	if response.Diagnostics.HasError() {
		return
	}
	id := state.ID.ValueString()
	empty, err := r.waitForFolderEmpty(ctx, id)
	if err != nil {
		appendHubSpotDiagnostic(&response.Diagnostics, "File folder child preflight failed", err)
		return
	}
	if !empty {
		response.Diagnostics.AddError("File folder is not empty", "The folder has at least one active direct child File folder or Managed file. Remove every child file first and child folder leaf-first; cascade deletion was not invoked.")
		return
	}
	deleteErr := r.folders.Delete(ctx, id)
	if deleteErr != nil && !isNotFound(deleteErr) && !isAmbiguous(deleteErr) {
		appendHubSpotDiagnostic(&response.Diagnostics, "File folder active absence was not verified", deleteErr)
		return
	}
	if err := r.waitForFolderAbsent(ctx, id); err != nil {
		response.Diagnostics.AddError("File folder active absence was not verified", "Exact-ID reads did not prove active absence before the operation deadline. State was retained; retry destroy after checking direct children and account access.")
		return
	}
	if deleteErr != nil && isAmbiguous(deleteErr) {
		response.Diagnostics.AddWarning("Delete response was ambiguous", "Exact generated-ID read-back proved active absence, so destroy converged without replaying DELETE. HubSpot-managed Trash retention may remain.")
	}
}

func (r *FileFolderResource) ImportState(ctx context.Context, request resource.ImportStateRequest, response *resource.ImportStateResponse) {
	if !generatedFilesIDPattern.MatchString(request.ID) {
		response.Diagnostics.AddAttributeError(path.Root("id"), "Invalid File folder import ID", "Use one non-zero decimal HubSpot-generated folder ID. Names, paths, URLs, and composite identifiers are not accepted.")
		return
	}
	folder, err := r.folders.Get(ctx, request.ID)
	if err != nil {
		if isNotFound(err) {
			response.Diagnostics.AddAttributeError(path.Root("id"), "File folder was not found", "Import requires the exact generated ID of an active File folder.")
		} else {
			appendHubSpotDiagnostic(&response.Diagnostics, "File folder import failed", err)
		}
		return
	}
	if folder.ID != request.ID || folder.Archived {
		response.Diagnostics.AddAttributeError(path.Root("id"), "File folder import identity mismatch", "HubSpot did not return the same active generated folder ID; no state was written.")
		return
	}
	model, diagnostics := folderModelFromRemote(folder)
	response.Diagnostics.Append(diagnostics...)
	if !response.Diagnostics.HasError() {
		response.Diagnostics.Append(response.State.Set(ctx, &model)...)
	}
}

func (r *FileFolderResource) folderCollision(ctx context.Context, parent *string, name, excludedID string) (bool, error) {
	folders, err := r.folders.Search(ctx, parent, name)
	if err != nil {
		return false, err
	}
	for _, folder := range folders {
		if !folder.Archived && folder.ID != excludedID && folder.Name == name && nullableStringsEqual(folder.ParentFolderID, parent) {
			return true, nil
		}
	}
	return false, nil
}

func (r *FileFolderResource) waitForFolderEmpty(ctx context.Context, id string) (bool, error) {
	const attempts = 7
	for attempt := 0; attempt < attempts; attempt++ {
		childFolders, err := r.folders.Search(ctx, &id, "")
		if err != nil {
			return false, err
		}
		childFiles, err := r.files.Search(ctx, &id, "")
		if err != nil {
			return false, err
		}
		if !hasActiveFolderChildren(childFolders, id) && !hasActiveFileChildren(childFiles, id) {
			return true, nil
		}
		if attempt+1 < attempts {
			if err := sleepResourcePoll(ctx, attempt); err != nil {
				return false, err
			}
		}
	}
	return false, nil
}

func (r *FileFolderResource) waitForCurrentParentPath(ctx context.Context, id string, parentID *string) error {
	if parentID == nil {
		return nil
	}
	const attempts = 7
	for attempt := 0; attempt < attempts; attempt++ {
		parent, parentErr := r.folders.Get(ctx, *parentID)
		folder, folderErr := r.folders.Get(ctx, id)
		if parentErr != nil {
			return parentErr
		}
		if folderErr != nil {
			return folderErr
		}
		expectedPath := strings.TrimRight(parent.Path, "/") + "/" + folder.Name
		if !parent.Archived && !folder.Archived && parent.ID == *parentID && folder.ID == id && nullableStringsEqual(folder.ParentFolderID, parentID) && folder.Path == expectedPath {
			return nil
		}
		if attempt+1 < attempts {
			if err := sleepResourcePoll(ctx, attempt); err != nil {
				return err
			}
		}
	}
	return errors.New("child folder path did not converge with current parent")
}

func (r *FileFolderResource) waitForFolderTask(ctx context.Context, id string) (hubspot.FileFolder, error) {
	for attempt := 0; ; attempt++ {
		task, err := r.folders.GetUpdateTask(ctx, id)
		if err != nil {
			return hubspot.FileFolder{}, err
		}
		if len(task.Errors) > 0 {
			return hubspot.FileFolder{}, errors.New("file folder task reported terminal errors")
		}
		switch task.Status {
		case "COMPLETE":
			if task.Result == nil {
				return hubspot.FileFolder{}, errors.New("file folder task omitted its terminal result")
			}
			return *task.Result, nil
		case "PENDING", "RUNNING", "PROCESSING":
		case "CANCELED", "ERROR", "FAILED", "":
			return hubspot.FileFolder{}, errors.New("file folder task did not complete")
		default:
			return hubspot.FileFolder{}, errors.New("file folder task returned an unknown state")
		}
		if err := sleepResourcePollAfter(ctx, attempt, task.RetryAfter); err != nil {
			return hubspot.FileFolder{}, err
		}
	}
}

func (r *FileFolderResource) waitForCurrentFolderRevision(ctx context.Context, state fileFolderResourceModel) (hubspot.FileFolder, bool, error) {
	const attempts = 7
	var observed hubspot.FileFolder
	for attempt := 0; attempt < attempts; attempt++ {
		folder, err := r.folders.Get(ctx, state.ID.ValueString())
		if err != nil {
			return folder, false, err
		}
		observed = folder
		if !folderSnapshotOlderThanState(folder, state) {
			return folder, false, nil
		}
		if attempt+1 < attempts {
			if err := sleepResourcePoll(ctx, attempt); err != nil {
				return observed, false, err
			}
		}
	}
	return observed, true, nil
}

func (r *FileFolderResource) waitForFolderPlan(ctx context.Context, id string, plan fileFolderResourceModel, createdAt string) (hubspot.FileFolder, error) {
	const attempts = 7
	const requiredConsecutiveMatches = 3
	var observed hubspot.FileFolder
	consecutiveMatches := 0
	for attempt := 0; attempt < attempts; attempt++ {
		folder, err := r.folders.Get(ctx, id)
		if err != nil {
			return folder, err
		}
		observed = folder
		if len(folderPlanMismatches(folder, id, plan, createdAt)) == 0 {
			consecutiveMatches++
			if consecutiveMatches == requiredConsecutiveMatches {
				return folder, nil
			}
		} else {
			consecutiveMatches = 0
		}
		if attempt+1 < attempts {
			if err := sleepResourcePoll(ctx, attempt); err != nil {
				return observed, err
			}
		}
	}
	return observed, errFileFolderReadBackDidNotConverge
}

func folderSnapshotOlderThanState(folder hubspot.FileFolder, state fileFolderResourceModel) bool {
	if folder.ID != state.ID.ValueString() || folder.Archived || state.UpdatedAt.IsNull() || state.UpdatedAt.IsUnknown() {
		return false
	}
	remoteUpdatedAt, remoteErr := time.Parse(time.RFC3339, folder.UpdatedAt)
	stateUpdatedAt, stateErr := time.Parse(time.RFC3339, state.UpdatedAt.ValueString())
	return remoteErr == nil && stateErr == nil && remoteUpdatedAt.Before(stateUpdatedAt)
}

func (r *FileFolderResource) waitForFolderAbsent(ctx context.Context, id string) error {
	for attempt := 0; ; attempt++ {
		folder, err := r.folders.Get(ctx, id)
		if isNotFound(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if folder.ID != id {
			return errors.New("file folder absence read returned a different identity")
		}
		if err := sleepResourcePoll(ctx, attempt); err != nil {
			return err
		}
	}
}

func folderModelFromRemote(folder hubspot.FileFolder) (fileFolderResourceModel, diag.Diagnostics) {
	var diagnostics diag.Diagnostics
	if !generatedFilesIDPattern.MatchString(folder.ID) || folder.Name == "" || strings.TrimSpace(folder.Name) != folder.Name || folder.Path == "" || !pathEndsWithName(folder.Path, folder.Name) {
		diagnostics.AddError("Unsupported File folder response", "HubSpot returned an invalid identity, name, or canonical path. Prior state was retained.")
		return fileFolderResourceModel{}, diagnostics
	}
	if folder.ParentFolderID != nil && !generatedFilesIDPattern.MatchString(*folder.ParentFolderID) {
		diagnostics.AddError("Unsupported File folder response", "HubSpot returned an invalid direct parent identity. Prior state was retained.")
		return fileFolderResourceModel{}, diagnostics
	}
	if _, err := time.Parse(time.RFC3339, folder.CreatedAt); err != nil {
		diagnostics.AddError("Unsupported File folder response", "HubSpot returned an invalid creation timestamp. Prior state was retained.")
	}
	if _, err := time.Parse(time.RFC3339, folder.UpdatedAt); err != nil {
		diagnostics.AddError("Unsupported File folder response", "HubSpot returned an invalid update timestamp. Prior state was retained.")
	}
	parent := types.StringNull()
	if folder.ParentFolderID != nil {
		parent = types.StringValue(*folder.ParentFolderID)
	}
	return fileFolderResourceModel{ID: types.StringValue(folder.ID), Name: types.StringValue(folder.Name), ParentFolderID: parent, Path: types.StringValue(folder.Path), CreatedAt: types.StringValue(folder.CreatedAt), UpdatedAt: types.StringValue(folder.UpdatedAt)}, diagnostics
}

func folderMatchesPlan(folder hubspot.FileFolder, expectedID string, plan fileFolderResourceModel) bool {
	return !folder.Archived && folder.ID == expectedID && folder.Name == plan.Name.ValueString() && nullableStringsEqual(folder.ParentFolderID, nullableStringPointer(plan.ParentFolderID)) && folder.Path != "" && pathEndsWithName(folder.Path, folder.Name)
}

func folderPlanMismatches(folder hubspot.FileFolder, expectedID string, plan fileFolderResourceModel, createdAt string) []string {
	mismatches := make([]string, 0, 6)
	if folder.Archived {
		mismatches = append(mismatches, "active status")
	}
	if folder.ID != expectedID {
		mismatches = append(mismatches, "generated identity")
	}
	if folder.Name != plan.Name.ValueString() {
		mismatches = append(mismatches, "name")
	}
	if !nullableStringsEqual(folder.ParentFolderID, nullableStringPointer(plan.ParentFolderID)) {
		mismatches = append(mismatches, "parent folder")
	}
	if folder.Path == "" || !pathEndsWithName(folder.Path, folder.Name) {
		mismatches = append(mismatches, "path")
	}
	if folder.CreatedAt != createdAt {
		mismatches = append(mismatches, "creation timestamp")
	}
	return mismatches
}

func nullableStringPointer(value types.String) *string {
	if value.IsNull() || value.IsUnknown() {
		return nil
	}
	result := value.ValueString()
	return &result
}

func nullableStringsEqual(a, b *string) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func pathEndsWithName(value, name string) bool {
	trimmed := strings.TrimSuffix(value, "/")
	return trimmed == name || strings.HasSuffix(trimmed, "/"+name)
}

func hasActiveFolderChildren(folders []hubspot.FileFolder, parent string) bool {
	for _, folder := range folders {
		if !folder.Archived && folder.ParentFolderID != nil && *folder.ParentFolderID == parent {
			return true
		}
	}
	return false
}

func hasActiveFileChildren(files []hubspot.ManagedFile, parent string) bool {
	for _, file := range files {
		if !file.Archived && file.FolderID == parent {
			return true
		}
	}
	return false
}

func sleepResourcePoll(ctx context.Context, attempt int) error {
	return sleepResourcePollAfter(ctx, attempt, 0)
}

func sleepResourcePollAfter(ctx context.Context, attempt int, retryAfter time.Duration) error {
	delays := []time.Duration{time.Second, 2 * time.Second, 4 * time.Second, 5 * time.Second}
	delay := delays[len(delays)-1]
	if attempt < len(delays) {
		delay = delays[attempt]
	}
	if retryAfter > 0 {
		delay = retryAfter
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
