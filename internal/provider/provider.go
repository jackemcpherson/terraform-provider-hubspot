// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"net/url"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/jackemcpherson/terraform-provider-hubspot/internal/hubspot"
)

const (
	defaultAPIBaseURL = "https://api.hubapi.com"
	tokenEnvironment  = "HUBSPOT_ACCESS_TOKEN"
)

// Provider is the protocol-6 provider. Configure creates an alias-local typed
// client set so resources never need to handle credentials directly.
type Provider struct {
	version            string
	developmentSurface bool
}

type providerData struct {
	AccessToken types.String `tfsdk:"access_token"`
	APIBaseURL  types.String `tfsdk:"api_base_url"`
}

// New returns a fresh provider instance for each configured alias.
func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &Provider{version: version}
	}
}

// NewDevelopment returns the full in-progress provider used by offline tests.
// Release binaries always use New so unqualified resources cannot enter their
// public protocol schema.
func NewDevelopment(version string) func() provider.Provider {
	return func() provider.Provider {
		return &Provider{version: version, developmentSurface: true}
	}
}

func (p *Provider) Metadata(_ context.Context, _ provider.MetadataRequest, response *provider.MetadataResponse) {
	response.TypeName = "hubspot"
	response.Version = p.version
}

func (p *Provider) Schema(_ context.Context, _ provider.SchemaRequest, response *provider.SchemaResponse) {
	response.Schema = schema.Schema{
		Description: "Free alpha of an OpenTofu-first provider for HubSpot CRM property configuration.",
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

	baseURL, err := url.Parse(data.APIBaseURL.ValueString())
	if err != nil {
		response.Diagnostics.AddAttributeError(path.Root("api_base_url"), "Invalid API base URL", "The configured API base URL could not be parsed.")
		return
	}
	clients, err := hubspot.NewClientSet(hubspot.TransportConfig{
		BaseURL:     baseURL,
		AccessToken: data.AccessToken.ValueString(),
		UserAgent:   "terraform-provider-hubspot/" + p.version + " protocol/6",
	})
	if err != nil {
		response.Diagnostics.AddAttributeError(path.Root("api_base_url"), "Invalid API base URL", err.Error())
		return
	}
	response.DataSourceData = clients
	response.ResourceData = clients
}

func (p *Provider) Resources(context.Context) []func() resource.Resource {
	if p.developmentSurface {
		return []func() resource.Resource{newDevelopmentPropertyResource, NewPropertyGroupResource, NewPipelineResource, NewCustomSchemaResource}
	}
	return []func() resource.Resource{NewPropertyResource, NewPropertyGroupResource}
}

func (p *Provider) DataSources(context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{NewPropertyDefinitionDataSource, NewPropertyDefinitionsDataSource}
}
