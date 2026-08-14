// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

type accountMembershipNameValidator struct{}

func (accountMembershipNameValidator) Description(context.Context) string {
	return "configured account membership names must contain a non-whitespace character"
}

func (v accountMembershipNameValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (accountMembershipNameValidator) ValidateString(_ context.Context, request validator.StringRequest, response *validator.StringResponse) {
	if request.ConfigValue.IsNull() || request.ConfigValue.IsUnknown() {
		return
	}
	if strings.TrimSpace(request.ConfigValue.ValueString()) == "" {
		response.Diagnostics.AddAttributeError(request.Path, "Account membership name is blank", "A configured first_name or last_name must contain a non-whitespace character.")
	}
}
