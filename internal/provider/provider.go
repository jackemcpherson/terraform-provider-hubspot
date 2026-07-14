// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

const (
	defaultAPIBaseURL = "https://api.hubapi.com"
	tokenEnvironment  = "HUBSPOT_ACCESS_TOKEN"
)

// Provider is the protocol-6 provider skeleton. Remote clients are intentionally
// introduced by the first resource tracer, not by provider configuration.
type Provider struct {
	version string
}

type providerData struct {
	AccessToken types.String `tfsdk:"access_token"`
	APIBaseURL  types.String `tfsdk:"api_base_url"`
}

type providerRuntimeData struct {
	APIBaseURL types.String `tfsdk:"api_base_url"`
}

// New returns a fresh provider instance for each configured alias.
func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &Provider{version: version}
	}
}

func (p *Provider) Metadata(_ context.Context, _ provider.MetadataRequest, response *provider.MetadataResponse) {
	response.TypeName = "hubspot"
	response.Version = p.version
}

func (p *Provider) Schema(_ context.Context, _ provider.SchemaRequest, response *provider.SchemaResponse) {
	response.Schema = schema.Schema{
		Description: "OpenTofu-first provider for declarative HubSpot CRM configuration.",
		Attributes: map[string]schema.Attribute{
			"access_token": schema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				Description:         "HubSpot static app access token. Prefer HUBSPOT_ACCESS_TOKEN.",
				MarkdownDescription: "HubSpot static app access token. Prefer the `HUBSPOT_ACCESS_TOKEN` environment variable.",
			},
			"api_base_url": schema.StringAttribute{
				Optional:            true,
				Validators:          []validator.String{apiBaseURLValidator{}},
				Description:         "Advanced absolute API origin override for testing; defaults to the HubSpot API.",
				MarkdownDescription: "Advanced absolute API origin override for testing. Defaults to `https://api.hubapi.com`.",
			},
		},
	}
}

func (p *Provider) Configure(ctx context.Context, request provider.ConfigureRequest, response *provider.ConfigureResponse) {
	var data providerData
	response.Diagnostics.Append(request.Config.Get(ctx, &data)...)
	if response.Diagnostics.HasError() {
		return
	}

	if data.AccessToken.IsNull() || data.AccessToken.ValueString() == "" {
		if token := os.Getenv(tokenEnvironment); token != "" {
			data.AccessToken = types.StringValue(token)
		}
	}
	if data.APIBaseURL.IsNull() || data.APIBaseURL.ValueString() == "" {
		data.APIBaseURL = types.StringValue(defaultAPIBaseURL)
	}

	// Configuration is deliberately local and side-effect free. The first remote
	// tracer will validate these values when a client is actually required.
	// Do not return the resolved token through provider data. The first remote
	// tracer will pass credentials directly from this boundary into its transport.
	runtimeData := providerRuntimeData{APIBaseURL: data.APIBaseURL}
	response.DataSourceData = runtimeData
	response.ResourceData = runtimeData
}

func (p *Provider) Resources(context.Context) []func() resource.Resource {
	return nil
}

func (p *Provider) DataSources(context.Context) []func() datasource.DataSource {
	return nil
}
