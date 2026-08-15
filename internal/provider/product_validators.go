// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"math/big"
	"regexp"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

var (
	productDecimalPattern    = regexp.MustCompile(`^(?:0|[1-9][0-9]*)(?:\.[0-9]+)?$`)
	productRecurrencePattern = regexp.MustCompile(`^P[1-9][0-9]*M$`)
	productImportIDPattern   = regexp.MustCompile(`^[1-9][0-9]*$`)
)

type productRequiredTextValidator struct{ subject string }

func (v productRequiredTextValidator) Description(context.Context) string {
	return v.subject + " must be non-blank and have no surrounding whitespace"
}

func (v productRequiredTextValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v productRequiredTextValidator) ValidateString(_ context.Context, request validator.StringRequest, response *validator.StringResponse) {
	if request.ConfigValue.IsNull() || request.ConfigValue.IsUnknown() {
		return
	}
	value := request.ConfigValue.ValueString()
	if value == "" || strings.TrimSpace(value) != value {
		response.Diagnostics.AddAttributeError(request.Path, "Invalid Product text", v.Description(context.Background())+".")
	}
}

type productDecimalValidator struct{ allowEmpty bool }

func (v productDecimalValidator) Description(context.Context) string {
	if v.allowEmpty {
		return "value must be empty or a canonical non-negative decimal string"
	}
	return "value must be a canonical non-negative decimal string"
}

func (v productDecimalValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v productDecimalValidator) ValidateString(_ context.Context, request validator.StringRequest, response *validator.StringResponse) {
	if request.ConfigValue.IsNull() || request.ConfigValue.IsUnknown() {
		return
	}
	value := request.ConfigValue.ValueString()
	if (value == "" && v.allowEmpty) || productDecimalPattern.MatchString(value) {
		return
	}
	response.Diagnostics.AddAttributeError(request.Path, "Invalid Product decimal", v.Description(context.Background())+".")
}

type productRecurrenceValidator struct{}

func (productRecurrenceValidator) Description(context.Context) string {
	return "recurring billing period must be empty or use P#M with a positive month count"
}

func (v productRecurrenceValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v productRecurrenceValidator) ValidateString(_ context.Context, request validator.StringRequest, response *validator.StringResponse) {
	if request.ConfigValue.IsNull() || request.ConfigValue.IsUnknown() {
		return
	}
	value := request.ConfigValue.ValueString()
	if value == "" || productRecurrencePattern.MatchString(value) {
		return
	}
	response.Diagnostics.AddAttributeError(request.Path, "Invalid Product recurrence", v.Description(context.Background())+".")
}

func validProductImportID(id string) bool { return productImportIDPattern.MatchString(id) }

func productDecimalsEqual(first, second string) bool {
	if first == "" || second == "" {
		return first == second
	}
	firstValue, firstOK := new(big.Rat).SetString(first)
	secondValue, secondOK := new(big.Rat).SetString(second)
	return firstOK && secondOK && firstValue.Cmp(secondValue) == 0
}
