// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

//go:build acceptance

package provider

import (
	"os"
	"time"
)

func init() {
	if interval, err := time.ParseDuration(os.Getenv("HUBSPOT_CONTACT_SEGMENT_TEST_POLL_INTERVAL")); err == nil && interval > 0 {
		contactSegmentDefaultPollInterval = interval
	}
	if timeout, err := time.ParseDuration(os.Getenv("HUBSPOT_CONTACT_SEGMENT_TEST_POLL_TIMEOUT")); err == nil && timeout > 0 {
		contactSegmentDefaultPollTimeout = timeout
	}
}
