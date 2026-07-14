// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"net"
	"net/url"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

type apiBaseURLValidator struct{}

func (apiBaseURLValidator) Description(context.Context) string {
	return "must be an absolute HTTPS URL; HTTP is allowed only for loopback test origins"
}

func (v apiBaseURLValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (apiBaseURLValidator) ValidateString(ctx context.Context, request validator.StringRequest, response *validator.StringResponse) {
	if request.ConfigValue.IsNull() || request.ConfigValue.IsUnknown() {
		return
	}

	value := request.ConfigValue.ValueString()
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.Hostname() == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		response.Diagnostics.AddAttributeError(request.Path, "Invalid API base URL", "Use an absolute URL with a host and no userinfo, query, or fragment.")
		return
	}
	if value != strings.TrimSpace(value) {
		response.Diagnostics.AddAttributeError(request.Path, "Invalid API base URL", "The API base URL must not have surrounding whitespace.")
		return
	}

	if parsed.Scheme == "https" {
		return
	}
	if parsed.Scheme == "http" && isLoopbackHost(parsed.Hostname()) {
		return
	}

	response.Diagnostics.AddAttributeError(request.Path, "Invalid API base URL", "Use HTTPS; HTTP is permitted only for loopback test origins.")
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
