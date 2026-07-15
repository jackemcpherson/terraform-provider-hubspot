// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

//go:build acceptance

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
	for _, stage := range pipeline.Stages {
		input := hubspot.PipelineStageWrite{StageID: stage.ID, Label: stage.Label, DisplayOrder: stage.DisplayOrder, Metadata: stage.Metadata}
		if stage.ID == stageID {
			input.Label = "Released provider out-of-band stage"
		}
		write.Stages = append(write.Stages, input)
	}
	if _, err := clients.Pipelines.Update(ctx, "deals", pipelineID, write); err != nil {
		t.Fatalf("mutate released deal pipeline: %s", acceptance.SanitizedHubSpotError(err))
	}
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
