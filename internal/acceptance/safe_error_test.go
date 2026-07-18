// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

package acceptance

import "testing"

func TestSanitizedEngineErrorPreservesHyphenatedTitle(t *testing.T) {
	err := sanitizedEngineError(assertionError{}, "Error: Property is discovery-only\n")
	commandErr, ok := err.(engineCommandError)
	if !ok {
		t.Fatalf("sanitized error type = %T", err)
	}
	if commandErr.title != "Property is discovery-only" {
		t.Fatalf("sanitized title = %q", commandErr.title)
	}
}

type assertionError struct{}

func (assertionError) Error() string { return "assertion failed" }
