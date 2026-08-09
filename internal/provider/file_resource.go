// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/jackemcpherson/terraform-provider-hubspot/internal/hubspot"
)

type FileResource struct{ client *hubspot.FileClient }

var (
	_ resource.ResourceWithImportState = (*FileResource)(nil)
	_ resource.ResourceWithModifyPlan  = (*FileResource)(nil)
)

type fileResourceModel struct {
	ID                types.String `tfsdk:"id"`
	Name              types.String `tfsdk:"name"`
	FolderID          types.String `tfsdk:"folder_id"`
	SourcePath        types.String `tfsdk:"source_path"`
	SourceSHA256      types.String `tfsdk:"source_sha256"`
	Access            types.String `tfsdk:"access"`
	Path              types.String `tfsdk:"path"`
	FileMD5           types.String `tfsdk:"file_md5"`
	Size              types.Int64  `tfsdk:"size"`
	Extension         types.String `tfsdk:"extension"`
	Type              types.String `tfsdk:"type"`
	Encoding          types.String `tfsdk:"encoding"`
	Height            types.Int64  `tfsdk:"height"`
	Width             types.Int64  `tfsdk:"width"`
	URL               types.String `tfsdk:"url"`
	DefaultHostingURL types.String `tfsdk:"default_hosting_url"`
	CreatedAt         types.String `tfsdk:"created_at"`
	UpdatedAt         types.String `tfsdk:"updated_at"`
}

func NewFileResource() resource.Resource { return &FileResource{} }

func (r *FileResource) Metadata(_ context.Context, _ resource.MetadataRequest, response *resource.MetadataResponse) {
	response.TypeName = "hubspot_file"
}

func (r *FileResource) Schema(_ context.Context, _ resource.SchemaRequest, response *resource.SchemaResponse) {
	computed := func(description string) schema.StringAttribute {
		return schema.StringAttribute{Computed: true, Description: description, MarkdownDescription: description}
	}
	response.Schema = schema.Schema{
		Version:             0,
		Description:         "Manages one HubSpot Managed file by generated ID with locally bound source bytes.",
		MarkdownDescription: "Manages one HubSpot Managed file by generated `id`, with local bytes bound to a reviewed SHA-256 digest.",
		Attributes: map[string]schema.Attribute{
			"id":                  schema.StringAttribute{Computed: true, Description: "HubSpot-generated file ID used as state and import identity.", MarkdownDescription: "HubSpot-generated file `id` used as the only state and import identity.", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"name":                schema.StringAttribute{Required: true, Description: "Managed file name in the explicit destination folder.", MarkdownDescription: "Managed file name in the explicit destination folder. It is mutable presentation, not identity.", Validators: []validator.String{managedFileNameValidator{}}},
			"folder_id":           schema.StringAttribute{Required: true, Description: "Generated destination File folder ID.", MarkdownDescription: "Generated destination File folder `id`; paths and implicit folders are not accepted.", Validators: []validator.String{generatedFilesIDValidator{}}},
			"source_path":         schema.StringAttribute{Required: true, Sensitive: true, Description: "Local regular-file path read at plan and apply.", MarkdownDescription: "Sensitive local regular-file path read at plan and apply. Bytes never enter state or diagnostics."},
			"source_sha256":       schema.StringAttribute{Required: true, Description: "Lowercase SHA-256 digest binding the planned source bytes.", MarkdownDescription: "Exactly 64 lowercase hexadecimal characters binding the planned source bytes.", Validators: []validator.String{sourceSHA256Validator{}}},
			"access":              schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("PRIVATE"), Description: "Supported HubSpot delivery access state; defaults to PRIVATE.", MarkdownDescription: "Supported HubSpot delivery access state. Defaults to `PRIVATE`.", Validators: []validator.String{fileAccessValidator{}}},
			"path":                computed("HubSpot-derived current file path; an observation, never identity."),
			"file_md5":            computed("Lowercase HubSpot MD5 content observation used with size for drift repair."),
			"size":                schema.Int64Attribute{Computed: true, Description: "Current file size in decimal bytes.", MarkdownDescription: "Current file size in decimal bytes."},
			"extension":           computed("HubSpot canonical file extension observation."),
			"type":                computed("HubSpot canonical file type classification."),
			"encoding":            computed("Nullable HubSpot content encoding observation."),
			"height":              schema.Int64Attribute{Computed: true, Description: "Nullable HubSpot media height observation.", MarkdownDescription: "Nullable HubSpot media height observation."},
			"width":               schema.Int64Attribute{Computed: true, Description: "Nullable HubSpot media width observation.", MarkdownDescription: "Nullable HubSpot media width observation."},
			"url":                 computed("Current account-domain delivery URL; an observation, never identity."),
			"default_hosting_url": computed("Current HubSpot-hosted delivery URL; an observation, never identity."),
			"created_at":          schema.StringAttribute{Computed: true, Description: "RFC 3339 creation timestamp preserved through PATCH and PUT.", MarkdownDescription: "HubSpot RFC 3339 creation observation preserved through `PATCH` and `PUT`.", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"updated_at":          computed("Current HubSpot RFC 3339 update observation."),
		},
	}
}

func (r *FileResource) Configure(_ context.Context, request resource.ConfigureRequest, response *resource.ConfigureResponse) {
	if request.ProviderData == nil {
		return
	}
	clients, ok := request.ProviderData.(*hubspot.ClientSet)
	if !ok || clients == nil || clients.Files == nil {
		response.Diagnostics.AddError("Provider is not configured", "The HubSpot Files client was not available to hubspot_file.")
		return
	}
	r.client = clients.Files
}

func (r *FileResource) ModifyPlan(ctx context.Context, request resource.ModifyPlanRequest, response *resource.ModifyPlanResponse) {
	if request.Plan.Raw.IsNull() {
		return
	}
	var plan fileResourceModel
	response.Diagnostics.Append(request.Plan.Get(ctx, &plan)...)
	if response.Diagnostics.HasError() || plan.SourcePath.IsNull() || plan.SourcePath.IsUnknown() || plan.SourceSHA256.IsNull() || plan.SourceSHA256.IsUnknown() {
		return
	}
	source, err := inspectManagedFileSource(plan.SourcePath.ValueString(), plan.SourceSHA256.ValueString())
	if err != nil {
		appendManagedFileSourceDiagnostic(&response.Diagnostics, err, path.Root("source_path"))
		return
	}
	plan.FileMD5 = types.StringValue(source.MD5)
	plan.Size = types.Int64Value(source.Size)
	if !request.State.Raw.IsNull() {
		var state fileResourceModel
		response.Diagnostics.Append(request.State.Get(ctx, &state)...)
		if response.Diagnostics.HasError() {
			return
		}
		metadataChanged := knownStringValueChanged(state.Name, plan.Name) || knownStringValueChanged(state.FolderID, plan.FolderID) || knownStringValueChanged(state.Access, plan.Access)
		contentChanged := state.FileMD5.IsNull() || state.FileMD5.IsUnknown() || state.FileMD5.ValueString() != source.MD5 || state.Size.IsNull() || state.Size.IsUnknown() || state.Size.ValueInt64() != source.Size
		if metadataChanged {
			plan.Path = types.StringUnknown()
			plan.URL = types.StringUnknown()
			plan.DefaultHostingURL = types.StringUnknown()
			plan.UpdatedAt = types.StringUnknown()
		}
		if metadataChanged || contentChanged {
			plan.Extension = types.StringUnknown()
			plan.Type = types.StringUnknown()
			plan.Encoding = types.StringUnknown()
			plan.Height = types.Int64Unknown()
			plan.Width = types.Int64Unknown()
			plan.UpdatedAt = types.StringUnknown()
		}
	}
	response.Diagnostics.Append(response.Plan.Set(ctx, &plan)...)
}

func (r *FileResource) Create(ctx context.Context, request resource.CreateRequest, response *resource.CreateResponse) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	var plan fileResourceModel
	response.Diagnostics.Append(request.Plan.Get(ctx, &plan)...)
	if response.Diagnostics.HasError() {
		return
	}
	source, err := inspectManagedFileSource(plan.SourcePath.ValueString(), plan.SourceSHA256.ValueString())
	if err != nil {
		appendManagedFileSourceDiagnostic(&response.Diagnostics, err, path.Root("source_path"))
		return
	}
	collision, err := r.fileCollision(ctx, plan.FolderID.ValueString(), plan.Name.ValueString(), "")
	if err != nil {
		appendHubSpotDiagnostic(&response.Diagnostics, "Managed file collision preflight failed", err)
		return
	}
	if collision {
		response.Diagnostics.AddError("Managed file collision", "An active Managed file already uses the requested name in the exact destination folder. Import a confirmed generated ID explicitly or choose a different name; no upload was sent.")
		return
	}

	created, createErr := r.client.Upload(ctx, hubspot.FileUpload{Name: plan.Name.ValueString(), FolderID: plan.FolderID.ValueString(), Access: plan.Access.ValueString(), Bytes: source.Bytes})
	if generatedFilesIDPattern.MatchString(created.ID) {
		recovery := plan
		recovery.ID = types.StringValue(created.ID)
		response.Diagnostics.Append(response.State.Set(ctx, &recovery)...)
		if response.Diagnostics.HasError() {
			return
		}
	}
	if created.ID == "" {
		if isFilesCollision(createErr) {
			response.Diagnostics.AddError("Managed file collision", "HubSpot rejected the exact-folder upload as a duplicate or target collision. No generated ID was adopted; inspect the intended folder and choose a collision-free name or import only a confirmed generated ID.")
		} else if createErr != nil && !isAmbiguous(createErr) {
			appendHubSpotDiagnostic(&response.Diagnostics, "Unable to create Managed file", createErr)
		} else {
			response.Diagnostics.AddError("Managed file creation outcome is unknown", "HubSpot did not return a generated file ID. Inspect the exact intended folder safely and import only a confirmed generated ID, or remove any residual before retrying. The upload was not replayed and the provider did not search by name for adoption.")
		}
		return
	}
	if !generatedFilesIDPattern.MatchString(created.ID) {
		response.Diagnostics.AddError("Managed file creation identity is invalid", "HubSpot returned a non-canonical generated file ID. The upload was not replayed and the provider did not search by name for adoption.")
		return
	}
	verified, verifyErr := r.client.Get(ctx, created.ID)
	if verifyErr != nil {
		response.Diagnostics.AddError("Managed file creation outcome requires recovery", "HubSpot returned generated file ID "+created.ID+", but exact-ID read-back failed. The ID was retained in state; retry refresh or import that exact ID.")
		return
	}
	if !managedFileMatchesPlan(verified, created.ID, plan, source) {
		r.cleanupMismatchedCreate(ctx, created.ID, response)
		return
	}
	model, diagnostics := fileModelFromRemote(verified, plan)
	response.Diagnostics.Append(diagnostics...)
	if response.Diagnostics.HasError() {
		return
	}
	if createErr != nil {
		response.Diagnostics.AddWarning("Create response was ambiguous", "Exact generated-ID read-back matched the planned Managed file, so creation converged without replaying the upload.")
	}
	response.Diagnostics.Append(response.State.Set(ctx, &model)...)
}

func (r *FileResource) Read(ctx context.Context, request resource.ReadRequest, response *resource.ReadResponse) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	var state fileResourceModel
	response.Diagnostics.Append(request.State.Get(ctx, &state)...)
	if response.Diagnostics.HasError() {
		return
	}
	file, err := r.client.Get(ctx, state.ID.ValueString())
	if err != nil {
		if isNotFound(err) {
			response.State.RemoveResource(ctx)
			return
		}
		appendHubSpotDiagnostic(&response.Diagnostics, "Managed file refresh failed", err)
		return
	}
	if file.ID != state.ID.ValueString() || file.Archived {
		response.Diagnostics.AddError("Managed file refresh was not verified", "HubSpot did not return the same active generated file ID. Prior state was retained.")
		return
	}
	model, diagnostics := fileModelFromRemote(file, state)
	response.Diagnostics.Append(diagnostics...)
	if !response.Diagnostics.HasError() {
		response.Diagnostics.Append(response.State.Set(ctx, &model)...)
	}
}

func (r *FileResource) Update(ctx context.Context, request resource.UpdateRequest, response *resource.UpdateResponse) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	var state, plan fileResourceModel
	response.Diagnostics.Append(request.State.Get(ctx, &state)...)
	response.Diagnostics.Append(request.Plan.Get(ctx, &plan)...)
	if response.Diagnostics.HasError() {
		return
	}
	source, err := inspectManagedFileSource(plan.SourcePath.ValueString(), plan.SourceSHA256.ValueString())
	if err != nil {
		appendManagedFileSourceDiagnostic(&response.Diagnostics, err, path.Root("source_path"))
		return
	}
	current, err := r.client.Get(ctx, state.ID.ValueString())
	if err != nil {
		appendHubSpotDiagnostic(&response.Diagnostics, "Managed file update was not verified", err)
		return
	}
	if current.ID != state.ID.ValueString() || current.Archived {
		response.Diagnostics.AddError("Managed file update was not verified", "Exact-ID read-back did not return the same active Managed file. Prior state was retained.")
		return
	}
	metadataChanged := current.Name != plan.Name.ValueString() || current.FolderID != plan.FolderID.ValueString() || current.Access != plan.Access.ValueString()
	if metadataChanged {
		collision, err := r.fileCollision(ctx, plan.FolderID.ValueString(), plan.Name.ValueString(), state.ID.ValueString())
		if err != nil {
			appendHubSpotDiagnostic(&response.Diagnostics, "Managed file collision preflight failed", err)
			return
		}
		if collision {
			response.Diagnostics.AddError("Managed file collision", "Another active Managed file already uses the requested target name in the exact destination folder. No PATCH or PUT was sent and prior state was retained.")
			return
		}
		patch := hubspot.FilePatch{}
		if current.Name != plan.Name.ValueString() {
			value := plan.Name.ValueString()
			patch.Name = &value
		}
		if current.FolderID != plan.FolderID.ValueString() {
			value := plan.FolderID.ValueString()
			patch.FolderID = &value
		}
		if current.Access != plan.Access.ValueString() {
			value := plan.Access.ValueString()
			patch.Access = &value
		}
		_, patchErr := r.client.Update(ctx, state.ID.ValueString(), patch)
		verified, verifyErr := r.waitForManagedFile(ctx, state.ID.ValueString(), func(file hubspot.ManagedFile) bool {
			return managedFileMetadataMatches(file, state.ID.ValueString(), plan) && file.CreatedAt == current.CreatedAt
		})
		if verifyErr != nil {
			response.Diagnostics.AddError("Managed file update was not verified", "PATCH outcome could not be proven by exact-ID read-back. Prior identity and state were retained for a safe retry.")
			return
		}
		if patchErr != nil {
			if !isAmbiguous(patchErr) {
				appendHubSpotDiagnostic(&response.Diagnostics, "Managed file update was not verified", patchErr)
				return
			}
			response.Diagnostics.AddWarning("PATCH response was ambiguous", "Exact-ID read-back proved every targeted metadata value, so the update converged without replaying PATCH.")
		}
		current = verified
	}

	contentChanged := current.FileMD5 != source.MD5 || current.Size != source.Size
	if contentChanged {
		_, replaceErr := r.client.Replace(ctx, state.ID.ValueString(), hubspot.FileReplacement{Name: plan.Name.ValueString(), Access: plan.Access.ValueString(), Bytes: source.Bytes})
		verified, verifyErr := r.waitForManagedFile(ctx, state.ID.ValueString(), func(file hubspot.ManagedFile) bool {
			return managedFileMatchesPlan(file, state.ID.ValueString(), plan, source) && file.CreatedAt == current.CreatedAt
		})
		if verifyErr != nil {
			response.Diagnostics.AddError("Managed file update was not verified", "PUT outcome could not be proven with preserved identity, creation time, planned MD5, and size. Prior state was retained for a safe retry.")
			return
		}
		if replaceErr != nil {
			if !isAmbiguous(replaceErr) {
				appendHubSpotDiagnostic(&response.Diagnostics, "Managed file update was not verified", replaceErr)
				return
			}
			response.Diagnostics.AddWarning("PUT response was ambiguous", "Exact-ID read-back proved in-place byte convergence, so replacement succeeded without replaying PUT.")
		}
		current = verified
	}
	model, diagnostics := fileModelFromRemote(current, plan)
	response.Diagnostics.Append(diagnostics...)
	if !response.Diagnostics.HasError() {
		response.Diagnostics.Append(response.State.Set(ctx, &model)...)
	}
}

func (r *FileResource) Delete(ctx context.Context, request resource.DeleteRequest, response *resource.DeleteResponse) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	var state fileResourceModel
	response.Diagnostics.Append(request.State.Get(ctx, &state)...)
	if response.Diagnostics.HasError() {
		return
	}
	id := state.ID.ValueString()
	deleteErr := r.client.Delete(ctx, id)
	if deleteErr != nil && !isNotFound(deleteErr) && !isAmbiguous(deleteErr) {
		appendHubSpotDiagnostic(&response.Diagnostics, "Managed file active absence was not verified", deleteErr)
		return
	}
	if err := r.waitForFileAbsent(ctx, id); err != nil {
		response.Diagnostics.AddError("Managed file active absence was not verified", "Exact-ID reads did not prove active absence before the operation deadline. State was retained; retry destroy after checking account access.")
		return
	}
	if deleteErr != nil && isAmbiguous(deleteErr) {
		response.Diagnostics.AddWarning("Delete response was ambiguous", "Exact generated-ID read-back proved active absence, so destroy converged without replaying DELETE. HubSpot-managed Trash retention may remain.")
	}
}

func (r *FileResource) ImportState(ctx context.Context, request resource.ImportStateRequest, response *resource.ImportStateResponse) {
	if !generatedFilesIDPattern.MatchString(request.ID) {
		response.Diagnostics.AddAttributeError(path.Root("id"), "Invalid Managed file import ID", "Use one non-zero decimal HubSpot-generated file ID. Names, paths, URLs, hashes, and composite identifiers are not accepted.")
		return
	}
	file, err := r.client.Get(ctx, request.ID)
	if err != nil {
		if isNotFound(err) {
			response.Diagnostics.AddAttributeError(path.Root("id"), "Managed file was not found", "Import requires the exact generated ID of an active Managed file.")
		} else {
			appendHubSpotDiagnostic(&response.Diagnostics, "Managed file import failed", err)
		}
		return
	}
	if file.ID != request.ID || file.Archived {
		response.Diagnostics.AddAttributeError(path.Root("id"), "Managed file import identity mismatch", "HubSpot did not return the same active generated file ID; no state was written.")
		return
	}
	base := fileResourceModel{SourcePath: types.StringNull(), SourceSHA256: types.StringNull()}
	model, diagnostics := fileModelFromRemote(file, base)
	response.Diagnostics.Append(diagnostics...)
	if !response.Diagnostics.HasError() {
		response.Diagnostics.Append(response.State.Set(ctx, &model)...)
	}
}

func (r *FileResource) fileCollision(ctx context.Context, folderID, name, excludedID string) (bool, error) {
	files, err := r.client.Search(ctx, &folderID, name)
	if err != nil {
		return false, err
	}
	for _, file := range files {
		if !file.Archived && file.ID != excludedID && file.FolderID == folderID && file.Name == name {
			return true, nil
		}
	}
	return false, nil
}

func (r *FileResource) waitForManagedFile(ctx context.Context, id string, matches func(hubspot.ManagedFile) bool) (hubspot.ManagedFile, error) {
	const attempts = 7
	var observed hubspot.ManagedFile
	for attempt := 0; attempt < attempts; attempt++ {
		file, err := r.client.Get(ctx, id)
		if err != nil {
			return file, err
		}
		observed = file
		if matches(file) {
			return file, nil
		}
		if attempt+1 < attempts {
			if err := sleepResourcePoll(ctx, attempt); err != nil {
				return observed, err
			}
		}
	}
	return observed, errors.New("managed file read-back did not converge")
}

func (r *FileResource) cleanupMismatchedCreate(ctx context.Context, id string, response *resource.CreateResponse) {
	deleteErr := r.client.Delete(ctx, id)
	if deleteErr == nil || isNotFound(deleteErr) || isAmbiguous(deleteErr) {
		if err := r.waitForFileAbsent(ctx, id); err == nil {
			response.State.RemoveResource(ctx)
			response.Diagnostics.AddError("Managed file collision", "HubSpot normalized or otherwise changed the requested file name, path, folder, access, or content. The known generated ID was deleted and active absence was verified; choose a collision-free target before retrying.")
			return
		}
	}
	response.Diagnostics.AddError("Managed file collision", "HubSpot returned a non-convergent Managed file and cleanup could not be verified. Generated ID "+id+" was retained for exact recovery; refresh or import that ID, then remove it safely before retrying.")
}

func (r *FileResource) waitForFileAbsent(ctx context.Context, id string) error {
	for attempt := 0; ; attempt++ {
		file, err := r.client.Get(ctx, id)
		if isNotFound(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if file.ID != id {
			return errors.New("managed file absence read returned a different identity")
		}
		if err := sleepResourcePoll(ctx, attempt); err != nil {
			return err
		}
	}
}

func fileModelFromRemote(file hubspot.ManagedFile, base fileResourceModel) (fileResourceModel, diag.Diagnostics) {
	var diagnostics diag.Diagnostics
	if !generatedFilesIDPattern.MatchString(file.ID) || file.Name == "" || strings.TrimSpace(file.Name) != file.Name || !generatedFilesIDPattern.MatchString(file.FolderID) || file.Path == "" || !pathEndsWithName(file.Path, file.Name) || !validMD5(file.FileMD5) || file.Size < 0 || file.Extension == "" || file.Type == "" || file.URL == "" || file.DefaultHostingURL == "" {
		diagnostics.AddError("Unsupported Managed file response", "HubSpot returned an invalid identity, placement, content observation, classification, or delivery observation. Prior state was retained.")
		return fileResourceModel{}, diagnostics
	}
	if !supportedFileAccess(file.Access) {
		diagnostics.AddError("Unsupported Managed file response", "HubSpot returned an access state outside PRIVATE, PUBLIC_INDEXABLE, and PUBLIC_NOT_INDEXABLE. Prior state was retained without rewriting hidden or sensitive content.")
		return fileResourceModel{}, diagnostics
	}
	if _, err := time.Parse(time.RFC3339, file.CreatedAt); err != nil {
		diagnostics.AddError("Unsupported Managed file response", "HubSpot returned an invalid creation timestamp. Prior state was retained.")
	}
	if _, err := time.Parse(time.RFC3339, file.UpdatedAt); err != nil {
		diagnostics.AddError("Unsupported Managed file response", "HubSpot returned an invalid update timestamp. Prior state was retained.")
	}
	encoding := types.StringNull()
	if file.Encoding != nil {
		encoding = types.StringValue(*file.Encoding)
	}
	height := types.Int64Null()
	if file.Height != nil {
		height = types.Int64Value(*file.Height)
	}
	width := types.Int64Null()
	if file.Width != nil {
		width = types.Int64Value(*file.Width)
	}
	return fileResourceModel{
		ID: types.StringValue(file.ID), Name: types.StringValue(file.Name), FolderID: types.StringValue(file.FolderID),
		SourcePath: base.SourcePath, SourceSHA256: base.SourceSHA256, Access: types.StringValue(file.Access),
		Path: types.StringValue(file.Path), FileMD5: types.StringValue(file.FileMD5), Size: types.Int64Value(file.Size),
		Extension: types.StringValue(file.Extension), Type: types.StringValue(file.Type), Encoding: encoding, Height: height, Width: width,
		URL: types.StringValue(file.URL), DefaultHostingURL: types.StringValue(file.DefaultHostingURL),
		CreatedAt: types.StringValue(file.CreatedAt), UpdatedAt: types.StringValue(file.UpdatedAt),
	}, diagnostics
}

func managedFileMatchesPlan(file hubspot.ManagedFile, expectedID string, plan fileResourceModel, source managedFileSource) bool {
	return managedFileMetadataMatches(file, expectedID, plan) && file.FileMD5 == source.MD5 && file.Size == source.Size && file.Path != "" && pathEndsWithName(file.Path, file.Name)
}

func managedFileMetadataMatches(file hubspot.ManagedFile, expectedID string, plan fileResourceModel) bool {
	return !file.Archived && file.ID == expectedID && file.Name == plan.Name.ValueString() && file.FolderID == plan.FolderID.ValueString() && file.Access == plan.Access.ValueString()
}

func validMD5(value string) bool {
	if len(value) != 32 {
		return false
	}
	for _, character := range value {
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
			return false
		}
	}
	return true
}

func supportedFileAccess(value string) bool {
	switch value {
	case "PRIVATE", "PUBLIC_INDEXABLE", "PUBLIC_NOT_INDEXABLE":
		return true
	default:
		return false
	}
}

func isFilesCollision(err error) bool {
	var apiError *hubspot.Error
	return errors.As(err, &apiError) && (apiError.Status == 400 || apiError.Status == 409)
}

func appendManagedFileSourceDiagnostic(diagnostics *diag.Diagnostics, err error, sourcePath path.Path) {
	var sourceErr *managedFileSourceError
	if !errors.As(err, &sourceErr) {
		diagnostics.AddAttributeError(sourcePath, "Managed file source is unavailable", "The local source could not be verified. No source bytes were included in this diagnostic.")
		return
	}
	switch sourceErr.Kind {
	case managedFileSourceDigestMismatch:
		diagnostics.AddAttributeError(sourcePath, "Managed file source digest mismatch", "The local source bytes do not match source_sha256. No source bytes were uploaded or included in diagnostics.")
	case managedFileSourceLimitExceeded:
		diagnostics.AddAttributeError(sourcePath, "Managed file exceeds the Free file limit", "The resolved regular file exceeds the 20,000,000-byte HubSpot Free per-file limit.")
	default:
		diagnostics.AddAttributeError(sourcePath, "Managed file source is unavailable", "source_path must resolve to a readable regular file. No source bytes or local path were included in this diagnostic.")
	}
}

func knownStringValueChanged(state, plan types.String) bool {
	if state.IsUnknown() || plan.IsUnknown() {
		return false
	}
	if state.IsNull() || plan.IsNull() {
		return state.IsNull() != plan.IsNull()
	}
	return state.ValueString() != plan.ValueString()
}
