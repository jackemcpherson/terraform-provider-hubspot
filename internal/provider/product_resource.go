// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/jackemcpherson/terraform-provider-hubspot/internal/hubspot"
)

// ProductResource manages one standard Product definition by generated ID.
type ProductResource struct{ client *hubspot.ProductClient }

var _ resource.ResourceWithImportState = (*ProductResource)(nil)

type productResourceModel struct {
	ID                     types.String `tfsdk:"id"`
	Name                   types.String `tfsdk:"name"`
	SKU                    types.String `tfsdk:"sku"`
	Description            types.String `tfsdk:"description"`
	Price                  types.String `tfsdk:"price"`
	Cost                   types.String `tfsdk:"cost"`
	RecurringBillingPeriod types.String `tfsdk:"recurring_billing_period"`
}

func NewProductResource() resource.Resource { return &ProductResource{} }

func (r *ProductResource) Metadata(_ context.Context, _ resource.MetadataRequest, response *resource.MetadataResponse) {
	response.TypeName = "hubspot_product"
}

func (r *ProductResource) Schema(_ context.Context, _ resource.SchemaRequest, response *resource.SchemaResponse) {
	response.Schema = schema.Schema{
		Version:             0,
		Description:         "Manages one standard HubSpot Product definition by its generated ID.",
		MarkdownDescription: "Manages one standard HubSpot Product definition by its generated `id`.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				Description:         "HubSpot-generated numeric Product ID and sole state and import identity.",
				MarkdownDescription: "HubSpot-generated numeric Product `id` and sole state and import identity.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name":        requiredString("Product name; it is managed presentation, not identity.", productRequiredTextValidator{subject: "Product name"}),
			"sku":         requiredString("Product SKU mapped to hs_sku; the generated ID remains identity.", productRequiredTextValidator{subject: "Product SKU"}),
			"description": requiredString("Product description.", productRequiredTextValidator{subject: "Product description"}),
			"price":       requiredString("Exact non-negative Product price decimal string.", productDecimalValidator{}),
			"cost": schema.StringAttribute{
				Optional:            true,
				Description:         "Optional exact cost decimal string. Null is unmanaged and empty clears the remote value.",
				MarkdownDescription: "Optional exact cost decimal string. `null` is unmanaged and an empty string clears `hs_cost_of_goods_sold`.",
				Validators:          []validator.String{productDecimalValidator{allowEmpty: true}},
			},
			"recurring_billing_period": schema.StringAttribute{
				Optional:            true,
				Description:         "Optional positive whole-month recurrence in P#M form. Null is unmanaged and empty clears it.",
				MarkdownDescription: "Optional positive whole-month recurrence in `P#M` form. `null` is unmanaged and an empty string clears it.",
				Validators:          []validator.String{productRecurrenceValidator{}},
			},
		},
	}
}

func (r *ProductResource) Configure(_ context.Context, request resource.ConfigureRequest, response *resource.ConfigureResponse) {
	if request.ProviderData == nil {
		return
	}
	clients, ok := request.ProviderData.(*hubspot.ClientSet)
	if !ok || clients == nil || clients.Products == nil {
		response.Diagnostics.AddError("Provider is not configured", "The HubSpot Products 2026-03 client was not available to hubspot_product.")
		return
	}
	r.client = clients.Products
}

func (r *ProductResource) Create(ctx context.Context, request resource.CreateRequest, response *resource.CreateResponse) {
	var plan productResourceModel
	response.Diagnostics.Append(request.Plan.Get(ctx, &plan)...)
	if response.Diagnostics.HasError() {
		return
	}
	created, createErr := r.client.Create(ctx, productWriteFromModel(plan))
	if created.ID == "" {
		if createErr != nil && !isAmbiguous(createErr) {
			appendHubSpotDiagnostic(&response.Diagnostics, "Product creation failed", createErr)
		} else {
			response.Diagnostics.AddError(
				"Product creation outcome is unknown",
				"HubSpot did not return a generated Product ID. Inspect Products in HubSpot and import the exact numeric ID if creation succeeded. The provider did not search by SKU or name.",
			)
		}
		return
	}
	if !validProductImportID(created.ID) {
		response.Diagnostics.AddError("Product creation identity is invalid", "HubSpot returned a non-canonical Product ID. The provider did not search by SKU or name.")
		return
	}
	recovery := plan
	recovery.ID = types.StringValue(created.ID)
	response.Diagnostics.Append(response.State.Set(ctx, &recovery)...)
	if response.Diagnostics.HasError() {
		return
	}
	verified, verifyErr := r.client.Get(ctx, created.ID)
	if verifyErr != nil {
		appendHubSpotDiagnostic(&response.Diagnostics, "Product creation exact-ID verification failed", verifyErr)
		return
	}
	if !productMatchesPlan(verified, plan) {
		response.Diagnostics.AddError("Product creation was not verified", "HubSpot did not return the same active generated ID with every planned managed value. The generated ID was retained in state for exact recovery.")
		return
	}
	if createErr != nil {
		response.Diagnostics.AddWarning("Create response was ambiguous", "HubSpot returned a create error, but exact read-back of the generated Product ID matched every planned managed value.")
	}
	plan.ID = types.StringValue(created.ID)
	response.Diagnostics.Append(response.State.Set(ctx, &plan)...)
}

func (r *ProductResource) Read(ctx context.Context, request resource.ReadRequest, response *resource.ReadResponse) {
	var state productResourceModel
	response.Diagnostics.Append(request.State.Get(ctx, &state)...)
	if response.Diagnostics.HasError() {
		return
	}
	id := state.ID.ValueString()
	product, err := r.client.Get(ctx, id)
	if err != nil {
		if isNotFound(err) {
			archived, archivedErr := r.client.GetArchived(ctx, id)
			if archivedErr == nil {
				if archived.ID != id || !archived.Archived {
					response.Diagnostics.AddError("Product absence was not verified", "The archived view returned a different or active Product identity. Prior state was retained.")
					return
				}
				response.State.RemoveResource(ctx)
				return
			}
			if isNotFound(archivedErr) {
				response.State.RemoveResource(ctx)
				return
			}
			appendHubSpotDiagnostic(&response.Diagnostics, "Product archived-view refresh failed", archivedErr)
			return
		}
		appendHubSpotDiagnostic(&response.Diagnostics, "Product refresh failed", err)
		return
	}
	if product.ID != id || product.Archived {
		response.Diagnostics.AddError("Product refresh was not verified", "HubSpot did not return the same active generated Product ID. Prior state was retained.")
		return
	}
	model, err := productModelFromRemote(product, &state)
	if err != nil {
		response.Diagnostics.AddError("Product runtime schema conflict", "HubSpot returned an unsupported Product property value. Prior state was retained.")
		return
	}
	response.Diagnostics.Append(response.State.Set(ctx, &model)...)
}

func (r *ProductResource) Update(ctx context.Context, request resource.UpdateRequest, response *resource.UpdateResponse) {
	var plan, state productResourceModel
	response.Diagnostics.Append(request.Plan.Get(ctx, &plan)...)
	response.Diagnostics.Append(request.State.Get(ctx, &state)...)
	if response.Diagnostics.HasError() {
		return
	}
	if state.ID.IsNull() || state.ID.IsUnknown() || !validProductImportID(state.ID.ValueString()) {
		response.Diagnostics.AddError("Product identity is missing", "The canonical generated Product ID was absent from state. No update was sent.")
		return
	}
	properties := productPatchFromModels(state, plan)
	if len(properties) == 0 {
		plan.ID = state.ID
		response.Diagnostics.Append(response.State.Set(ctx, &plan)...)
		return
	}
	_, updateErr := r.client.Patch(ctx, state.ID.ValueString(), properties)
	verified, verifyErr := r.client.Get(ctx, state.ID.ValueString())
	if verifyErr != nil {
		if updateErr != nil {
			appendHubSpotDiagnostic(&response.Diagnostics, "Product update failed", updateErr)
		} else {
			appendHubSpotDiagnostic(&response.Diagnostics, "Product update verification failed", verifyErr)
		}
		return
	}
	if !productMatchesPlan(verified, plan) {
		if updateErr != nil {
			appendHubSpotDiagnostic(&response.Diagnostics, "Product update failed", updateErr)
		} else {
			response.Diagnostics.AddError("Product update was not verified", "HubSpot did not return the same active generated ID with every planned managed value. Prior state was retained.")
		}
		return
	}
	if updateErr != nil {
		response.Diagnostics.AddWarning("Update response was ambiguous", "HubSpot returned an update error, but exact read-back matched every planned managed Product value.")
	}
	plan.ID = state.ID
	response.Diagnostics.Append(response.State.Set(ctx, &plan)...)
}

func (r *ProductResource) Delete(ctx context.Context, request resource.DeleteRequest, response *resource.DeleteResponse) {
	var state productResourceModel
	response.Diagnostics.Append(request.State.Get(ctx, &state)...)
	if response.Diagnostics.HasError() {
		return
	}
	id := state.ID.ValueString()
	presence, err := r.productPresence(ctx, id)
	if err != nil {
		appendHubSpotDiagnostic(&response.Diagnostics, "Product archival preflight failed", err)
		return
	}
	if presence != productPresenceActive {
		response.State.RemoveResource(ctx)
		return
	}
	archiveErr := r.client.Archive(ctx, id)
	presence, verifyErr := r.waitForProductTerminal(ctx, id)
	if verifyErr != nil {
		if archiveErr != nil {
			appendHubSpotDiagnostic(&response.Diagnostics, "Product archival failed", archiveErr)
		} else {
			appendHubSpotDiagnostic(&response.Diagnostics, "Product archival verification failed", verifyErr)
		}
		return
	}
	if presence == productPresenceActive {
		response.Diagnostics.AddError("Product archival was not verified", "HubSpot did not prove exact active absence and the same archived Product identity. Prior state was retained.")
		return
	}
	if archiveErr != nil && !isNotFound(archiveErr) {
		response.Diagnostics.AddWarning("Archive response was ambiguous", "HubSpot returned an archive error, but active absence and the same archived Product identity were verified.")
	}
	response.State.RemoveResource(ctx)
}

func (r *ProductResource) ImportState(ctx context.Context, request resource.ImportStateRequest, response *resource.ImportStateResponse) {
	if !validProductImportID(request.ID) {
		response.Diagnostics.AddAttributeError(path.Root("id"), "Invalid Product import ID", "Use one exact canonical numeric HubSpot Product ID. SKUs, names, and composite identifiers are not accepted.")
		return
	}
	product, err := r.client.Get(ctx, request.ID)
	if err != nil {
		if !isNotFound(err) {
			appendHubSpotDiagnostic(&response.Diagnostics, "Product import failed", err)
			return
		}
		archived, archivedErr := r.client.GetArchived(ctx, request.ID)
		if archivedErr == nil && archived.ID == request.ID && archived.Archived {
			response.Diagnostics.AddAttributeError(path.Root("id"), "Archived Product cannot be imported", "Import requires the exact generated ID of an active supported Product.")
			return
		}
		if isNotFound(archivedErr) {
			response.Diagnostics.AddAttributeError(path.Root("id"), "Product was not found", "Neither the active nor archived view contained the exact generated Product ID.")
			return
		}
		if archivedErr != nil {
			appendHubSpotDiagnostic(&response.Diagnostics, "Product archived-view import check failed", archivedErr)
			return
		}
		response.Diagnostics.AddAttributeError(path.Root("id"), "Product import identity mismatch", "HubSpot did not return the exact archived Product identity.")
		return
	}
	if product.ID != request.ID || product.Archived {
		response.Diagnostics.AddAttributeError(path.Root("id"), "Product import identity mismatch", "HubSpot did not return the same active generated Product ID.")
		return
	}
	model, modelErr := productModelFromRemote(product, nil)
	if modelErr != nil {
		response.Diagnostics.AddAttributeError(path.Root("id"), "Unsupported Product cannot be imported", "HubSpot returned an unsupported required Product property value.")
		return
	}
	response.Diagnostics.Append(response.State.Set(ctx, &model)...)
}

type productPresence int

const (
	productPresenceActive productPresence = iota
	productPresenceArchived
	productPresenceAbsent
)

func (r *ProductResource) productPresence(ctx context.Context, id string) (productPresence, error) {
	active, err := r.client.Get(ctx, id)
	if err == nil {
		if active.ID != id || active.Archived {
			return productPresenceActive, errors.New("active Product identity was not exact")
		}
		return productPresenceActive, nil
	}
	if !isNotFound(err) {
		return productPresenceActive, err
	}
	archived, archivedErr := r.client.GetArchived(ctx, id)
	if archivedErr == nil {
		if archived.ID != id || !archived.Archived {
			return productPresenceArchived, errors.New("archived Product identity was not exact")
		}
		return productPresenceArchived, nil
	}
	if isNotFound(archivedErr) {
		return productPresenceAbsent, nil
	}
	return productPresenceAbsent, archivedErr
}

func (r *ProductResource) waitForProductTerminal(ctx context.Context, id string) (productPresence, error) {
	const attempts = 5
	for attempt := 0; attempt < attempts; attempt++ {
		presence, err := r.productPresence(ctx, id)
		if err != nil || presence != productPresenceActive {
			return presence, err
		}
		if attempt+1 < attempts {
			timer := time.NewTimer(100 * time.Millisecond)
			select {
			case <-ctx.Done():
				timer.Stop()
				return productPresenceActive, ctx.Err()
			case <-timer.C:
			}
		}
	}
	return productPresenceActive, errors.New("product remained active after bounded archive verification")
}

func productWriteFromModel(model productResourceModel) hubspot.ProductWrite {
	write := hubspot.ProductWrite{
		Name: model.Name.ValueString(), SKU: model.SKU.ValueString(),
		Description: model.Description.ValueString(), Price: model.Price.ValueString(),
	}
	if !model.Cost.IsNull() && !model.Cost.IsUnknown() {
		value := model.Cost.ValueString()
		write.Cost = &value
	}
	if !model.RecurringBillingPeriod.IsNull() && !model.RecurringBillingPeriod.IsUnknown() {
		value := model.RecurringBillingPeriod.ValueString()
		write.RecurringBillingPeriod = &value
	}
	return write
}

func productPatchFromModels(state, plan productResourceModel) map[string]string {
	properties := make(map[string]string)
	if state.Name.ValueString() != plan.Name.ValueString() {
		properties["name"] = plan.Name.ValueString()
	}
	if state.SKU.ValueString() != plan.SKU.ValueString() {
		properties["hs_sku"] = plan.SKU.ValueString()
	}
	if state.Description.ValueString() != plan.Description.ValueString() {
		properties["description"] = plan.Description.ValueString()
	}
	if !productDecimalsEqual(state.Price.ValueString(), plan.Price.ValueString()) {
		properties["price"] = plan.Price.ValueString()
	}
	if !plan.Cost.IsNull() && !plan.Cost.IsUnknown() &&
		(state.Cost.IsNull() || state.Cost.IsUnknown() || !productDecimalsEqual(state.Cost.ValueString(), plan.Cost.ValueString())) {
		properties["hs_cost_of_goods_sold"] = plan.Cost.ValueString()
	}
	if !plan.RecurringBillingPeriod.IsNull() && !plan.RecurringBillingPeriod.IsUnknown() &&
		(state.RecurringBillingPeriod.IsNull() || state.RecurringBillingPeriod.IsUnknown() || state.RecurringBillingPeriod.ValueString() != plan.RecurringBillingPeriod.ValueString()) {
		properties["hs_recurring_billing_period"] = plan.RecurringBillingPeriod.ValueString()
	}
	return properties
}

func productMatchesPlan(product hubspot.Product, plan productResourceModel) bool {
	if product.Archived || product.ID == "" || product.Name != plan.Name.ValueString() ||
		product.SKU != plan.SKU.ValueString() || product.Description != plan.Description.ValueString() ||
		!productDecimalsEqual(product.Price, plan.Price.ValueString()) {
		return false
	}
	if !plan.Cost.IsNull() && !plan.Cost.IsUnknown() && !productDecimalsEqual(product.Cost, plan.Cost.ValueString()) {
		return false
	}
	return plan.RecurringBillingPeriod.IsNull() || plan.RecurringBillingPeriod.IsUnknown() ||
		product.RecurringBillingPeriod == plan.RecurringBillingPeriod.ValueString()
}

func productModelFromRemote(product hubspot.Product, prior *productResourceModel) (productResourceModel, error) {
	if !validProductImportID(product.ID) || !productDecimalPattern.MatchString(product.Price) ||
		product.Name == "" || strings.TrimSpace(product.Name) != product.Name ||
		product.SKU == "" || strings.TrimSpace(product.SKU) != product.SKU ||
		product.Description == "" || strings.TrimSpace(product.Description) != product.Description {
		return productResourceModel{}, errors.New("unsupported Product identity or price")
	}
	model := productResourceModel{
		ID: types.StringValue(product.ID), Name: types.StringValue(product.Name),
		SKU: types.StringValue(product.SKU), Description: types.StringValue(product.Description),
		Price: types.StringValue(product.Price), Cost: types.StringNull(),
		RecurringBillingPeriod: types.StringNull(),
	}
	if prior != nil {
		if !prior.Price.IsNull() && !prior.Price.IsUnknown() && productDecimalsEqual(prior.Price.ValueString(), product.Price) {
			model.Price = prior.Price
		}
		if prior.Cost.IsNull() {
			model.Cost = types.StringNull()
		} else if !prior.Cost.IsUnknown() {
			if product.Cost != "" && !productDecimalPattern.MatchString(product.Cost) {
				return productResourceModel{}, errors.New("unsupported managed Product cost")
			}
			model.Cost = types.StringValue(product.Cost)
			if productDecimalsEqual(prior.Cost.ValueString(), product.Cost) {
				model.Cost = prior.Cost
			}
		}
		if prior.RecurringBillingPeriod.IsNull() {
			model.RecurringBillingPeriod = types.StringNull()
		} else if !prior.RecurringBillingPeriod.IsUnknown() {
			if product.RecurringBillingPeriod != "" && !productRecurrencePattern.MatchString(product.RecurringBillingPeriod) {
				return productResourceModel{}, errors.New("unsupported managed Product recurrence")
			}
			model.RecurringBillingPeriod = types.StringValue(product.RecurringBillingPeriod)
		}
	} else {
		if product.Cost != "" {
			if !productDecimalPattern.MatchString(product.Cost) {
				return productResourceModel{}, errors.New("unsupported Product cost")
			}
			model.Cost = types.StringValue(product.Cost)
		}
		if product.RecurringBillingPeriod != "" {
			if !productRecurrencePattern.MatchString(product.RecurringBillingPeriod) {
				return productResourceModel{}, errors.New("unsupported Product recurrence")
			}
			model.RecurringBillingPeriod = types.StringValue(product.RecurringBillingPeriod)
		}
	}
	return model, nil
}
