// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

package acceptance

import (
	"errors"
	"fmt"

	"github.com/jackemcpherson/terraform-provider-hubspot/internal/hubspot"
)

// SanitizedHubSpotError reports only the HTTP status and vetted category enums.
func SanitizedHubSpotError(err error) string {
	var apiError *hubspot.Error
	if errors.As(err, &apiError) {
		result := fmt.Sprintf("HubSpot HTTP %d", apiError.Status)
		if apiError.Category != "" {
			result += " (" + apiError.Category + ")"
		}
		if apiError.SubCategory != "" {
			result += " [" + apiError.SubCategory + "]"
		}
		return result
	}
	return "HubSpot operation could not be verified"
}
