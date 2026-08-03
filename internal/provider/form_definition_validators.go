// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"regexp"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	formColorPattern      = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)
	formFontFamilyPattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9 -]*(?:, [A-Za-z][A-Za-z0-9 -]*)*$`)
	formPositiveNumber    = `(?:0\.[0-9]*[1-9][0-9]*|[1-9][0-9]*(?:\.[0-9]+)?)`
	formPixelSizePattern  = regexp.MustCompile(`^` + formPositiveNumber + `px$`)
	formPercentPattern    = regexp.MustCompile(`^` + formPositiveNumber + `%$`)
	formSubmitSizePattern = regexp.MustCompile(`^` + formPositiveNumber + `px ` + formPositiveNumber + `px$`)
	formDomainPattern     = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)+$`)
	formImportIDPattern   = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
)

func validFormImportID(id string) bool { return formImportIDPattern.MatchString(id) }

type formRequiredTextValidator struct{ kind string }

func (v formRequiredTextValidator) Description(context.Context) string {
	return v.kind + " must be non-blank and have no surrounding whitespace"
}

func (v formRequiredTextValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v formRequiredTextValidator) ValidateString(_ context.Context, request validator.StringRequest, response *validator.StringResponse) {
	if request.ConfigValue.IsNull() || request.ConfigValue.IsUnknown() {
		return
	}
	value := request.ConfigValue.ValueString()
	if value == "" || strings.TrimSpace(value) != value {
		response.Diagnostics.AddAttributeError(request.Path, "Invalid form presentation", v.Description(context.Background())+".")
	}
}

type formLanguageValidator struct{}

func (formLanguageValidator) Description(context.Context) string {
	return "language must be the live-proven English code en"
}

func (v formLanguageValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v formLanguageValidator) ValidateString(_ context.Context, request validator.StringRequest, response *validator.StringResponse) {
	if request.ConfigValue.IsNull() || request.ConfigValue.IsUnknown() {
		return
	}
	if request.ConfigValue.ValueString() != "en" {
		response.Diagnostics.AddAttributeError(request.Path, "Unsupported form language", v.Description(context.Background())+".")
	}
}

type formAlignmentValidator struct{}

func (formAlignmentValidator) Description(context.Context) string {
	return "submit alignment must be left or center"
}

func (v formAlignmentValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v formAlignmentValidator) ValidateString(_ context.Context, request validator.StringRequest, response *validator.StringResponse) {
	if request.ConfigValue.IsNull() || request.ConfigValue.IsUnknown() {
		return
	}
	value := request.ConfigValue.ValueString()
	if value != "left" && value != "center" {
		response.Diagnostics.AddAttributeError(request.Path, "Unsupported submit alignment", v.Description(context.Background())+".")
	}
}

type formColorValidator struct{}

func (formColorValidator) Description(context.Context) string {
	return "color must be a six-digit hexadecimal CSS color beginning with #"
}

func (v formColorValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v formColorValidator) ValidateString(ctx context.Context, request validator.StringRequest, response *validator.StringResponse) {
	validateFormPattern(ctx, request, response, formColorPattern, "Invalid form color", v.Description(ctx))
}

type formFontFamilyValidator struct{}

func (formFontFamilyValidator) Description(context.Context) string {
	return "font family must be a comma-and-space separated simple CSS font stack"
}

func (v formFontFamilyValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v formFontFamilyValidator) ValidateString(ctx context.Context, request validator.StringRequest, response *validator.StringResponse) {
	validateFormPattern(ctx, request, response, formFontFamilyPattern, "Invalid form font family", v.Description(ctx))
}

type formPixelSizeValidator struct{}

func (formPixelSizeValidator) Description(context.Context) string {
	return "size must be a positive pixel value such as 13px"
}

func (v formPixelSizeValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v formPixelSizeValidator) ValidateString(ctx context.Context, request validator.StringRequest, response *validator.StringResponse) {
	validateFormPattern(ctx, request, response, formPixelSizePattern, "Invalid form pixel size", v.Description(ctx))
}

type formPercentageSizeValidator struct{}

func (formPercentageSizeValidator) Description(context.Context) string {
	return "width must be a positive percentage such as 100%"
}

func (v formPercentageSizeValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v formPercentageSizeValidator) ValidateString(ctx context.Context, request validator.StringRequest, response *validator.StringResponse) {
	validateFormPattern(ctx, request, response, formPercentPattern, "Invalid form percentage width", v.Description(ctx))
}

type formSubmitSizeValidator struct{}

func (formSubmitSizeValidator) Description(context.Context) string {
	return "submit size must contain two positive pixel values such as 12px 24px"
}

func (v formSubmitSizeValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v formSubmitSizeValidator) ValidateString(ctx context.Context, request validator.StringRequest, response *validator.StringResponse) {
	validateFormPattern(ctx, request, response, formSubmitSizePattern, "Invalid form submit size", v.Description(ctx))
}

func validateFormPattern(_ context.Context, request validator.StringRequest, response *validator.StringResponse, pattern *regexp.Regexp, summary, detail string) {
	if request.ConfigValue.IsNull() || request.ConfigValue.IsUnknown() {
		return
	}
	if !pattern.MatchString(request.ConfigValue.ValueString()) {
		response.Diagnostics.AddAttributeError(request.Path, summary, detail+".")
	}
}

type formBlockedEmailDomainsValidator struct{}

func (formBlockedEmailDomainsValidator) Description(context.Context) string {
	return "each blocked email domain must be a canonical lowercase DNS name"
}

func (v formBlockedEmailDomainsValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v formBlockedEmailDomainsValidator) ValidateList(_ context.Context, request validator.ListRequest, response *validator.ListResponse) {
	if request.ConfigValue.IsNull() || request.ConfigValue.IsUnknown() {
		return
	}
	for _, element := range request.ConfigValue.Elements() {
		value, ok := element.(types.String)
		if !ok || value.IsNull() || value.IsUnknown() {
			continue
		}
		domain := value.ValueString()
		if len(domain) > 253 || !formDomainPattern.MatchString(domain) {
			response.Diagnostics.AddAttributeError(request.Path, "Invalid blocked email domain", v.Description(context.Background())+".")
			return
		}
	}
}
