// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

//go:build acceptance && deferred

package acceptance_test

import (
	"context"
	"net/url"
	"testing"
	"time"

	"github.com/jackemcpherson/terraform-provider-hubspot/internal/acceptance"
	"github.com/jackemcpherson/terraform-provider-hubspot/internal/hubspot"
)

func TestReleasedTicketPipelineDrift(t *testing.T) {
	clients := releasedTicketPipelineClients(t)
	prefix := requiredEnvironment(t, "HUBSPOT_ACCEPTANCE_PREFIX")
	pipelineID := requiredEnvironment(t, "HUBSPOT_RELEASED_PIPELINE_ID")
	stageID := requiredEnvironment(t, "HUBSPOT_RELEASED_STAGE_ID")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	pipeline, err := clients.Pipelines.Get(ctx, "tickets", pipelineID)
	if err != nil {
		t.Fatalf("read released ticket pipeline for drift: %s", acceptance.SanitizedHubSpotError(err))
	}
	write := hubspot.PipelineWrite{Label: prefix + "released_ticket_out_of_band", DisplayOrder: pipeline.DisplayOrder, Stages: make([]hubspot.PipelineStageWrite, 0, len(pipeline.Stages))}
	stageFound := false
	for _, stage := range pipeline.Stages {
		input := hubspot.PipelineStageWrite{StageID: stage.ID, Label: stage.Label, DisplayOrder: stage.DisplayOrder, Metadata: stage.Metadata}
		if stage.ID == stageID {
			stageFound = true
			input.Label = "Released provider out-of-band ticket stage"
			input.Metadata = map[string]string{"ticketState": "OPEN"}
		}
		write.Stages = append(write.Stages, input)
	}
	if !stageFound {
		t.Fatal("released ticket drift target was not present")
	}
	if _, err := clients.Pipelines.Update(ctx, "tickets", pipelineID, write); err != nil {
		t.Fatalf("mutate released ticket pipeline: %s", acceptance.SanitizedHubSpotError(err))
	}
	verified, err := clients.Pipelines.Get(ctx, "tickets", pipelineID)
	if err != nil {
		t.Fatalf("verify released ticket pipeline drift: %s", acceptance.SanitizedHubSpotError(err))
	}
	if verified.Label != write.Label {
		t.Fatal("released ticket pipeline scalar drift was not verified")
	}
	for _, stage := range verified.Stages {
		if stage.ID == stageID && stage.Label == "Released provider out-of-band ticket stage" && stage.Metadata["ticketState"] == "OPEN" {
			return
		}
	}
	t.Fatal("released ticket pipeline stage drift was not verified")
}

func TestReleasedTicketPipelineArchived(t *testing.T) {
	clients := releasedTicketPipelineClients(t)
	pipelineID := requiredEnvironment(t, "HUBSPOT_RELEASED_PIPELINE_ID")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	pipeline, err := clients.Pipelines.GetArchived(ctx, "tickets", pipelineID)
	if err != nil {
		t.Fatalf("verify released ticket-pipeline terminal state: %s", acceptance.SanitizedHubSpotError(err))
	}
	if pipeline.ID != pipelineID || !pipeline.Archived {
		t.Fatal("released ticket pipeline did not reach its canonical archived terminal state")
	}
}

func releasedTicketPipelineClients(t *testing.T) *hubspot.ClientSet {
	t.Helper()
	origin, err := url.Parse("https://api.hubapi.com")
	if err != nil {
		t.Fatal("parse HubSpot API origin")
	}
	clients, err := hubspot.NewClientSet(hubspot.TransportConfig{BaseURL: origin, AccessToken: requiredEnvironment(t, "HUBSPOT_ACCESS_TOKEN"), UserAgent: "terraform-provider-hubspot/released-ticket-pipeline-verification"})
	if err != nil {
		t.Fatal("configure released ticket-pipeline verification")
	}
	return clients
}
