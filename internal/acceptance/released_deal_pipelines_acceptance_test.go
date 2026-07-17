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

func TestReleasedDealPipelineDrift(t *testing.T) {
	clients := releasedDealPipelineClients(t)
	prefix := requiredEnvironment(t, "HUBSPOT_ACCEPTANCE_PREFIX")
	pipelineID := requiredEnvironment(t, "HUBSPOT_RELEASED_PIPELINE_ID")
	stageID := requiredEnvironment(t, "HUBSPOT_RELEASED_STAGE_ID")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	pipeline, err := clients.Pipelines.Get(ctx, "deals", pipelineID)
	if err != nil {
		t.Fatalf("read released deal pipeline for drift: %s", acceptance.SanitizedHubSpotError(err))
	}
	write := hubspot.PipelineWrite{Label: prefix + "released_out_of_band", DisplayOrder: pipeline.DisplayOrder, Stages: make([]hubspot.PipelineStageWrite, 0, len(pipeline.Stages))}
	stageFound := false
	for _, stage := range pipeline.Stages {
		input := hubspot.PipelineStageWrite{StageID: stage.ID, Label: stage.Label, DisplayOrder: stage.DisplayOrder, Metadata: stage.Metadata}
		if stage.ID == stageID {
			stageFound = true
			input.Label = "Released provider out-of-band stage"
		}
		write.Stages = append(write.Stages, input)
	}
	if !stageFound {
		t.Fatal("released deal drift target was not present")
	}
	if _, err := clients.Pipelines.Update(ctx, "deals", pipelineID, write); err != nil {
		t.Fatalf("mutate released deal pipeline: %s", acceptance.SanitizedHubSpotError(err))
	}
	verified, err := clients.Pipelines.Get(ctx, "deals", pipelineID)
	if err != nil {
		t.Fatalf("verify released deal pipeline drift: %s", acceptance.SanitizedHubSpotError(err))
	}
	if verified.Label != write.Label {
		t.Fatal("released deal pipeline scalar drift was not verified")
	}
	for _, stage := range verified.Stages {
		if stage.ID == stageID && stage.Label == "Released provider out-of-band stage" {
			return
		}
	}
	t.Fatal("released deal pipeline stage drift was not verified")
}

func TestReleasedDealPipelineArchived(t *testing.T) {
	clients := releasedDealPipelineClients(t)
	pipelineID := requiredEnvironment(t, "HUBSPOT_RELEASED_PIPELINE_ID")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	pipeline, err := clients.Pipelines.GetArchived(ctx, "deals", pipelineID)
	if err != nil {
		t.Fatalf("verify released deal-pipeline terminal state: %s", acceptance.SanitizedHubSpotError(err))
	}
	if pipeline.ID != pipelineID || !pipeline.Archived {
		t.Fatal("released deal pipeline did not reach its canonical archived terminal state")
	}
}

func releasedDealPipelineClients(t *testing.T) *hubspot.ClientSet {
	t.Helper()
	origin, err := url.Parse("https://api.hubapi.com")
	if err != nil {
		t.Fatal("parse HubSpot API origin")
	}
	clients, err := hubspot.NewClientSet(hubspot.TransportConfig{
		BaseURL: origin, AccessToken: requiredEnvironment(t, "HUBSPOT_ACCESS_TOKEN"),
		UserAgent: "terraform-provider-hubspot/released-deal-pipeline-verification",
	})
	if err != nil {
		t.Fatal("configure released deal-pipeline verification")
	}
	return clients
}
