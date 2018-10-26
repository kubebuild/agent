package graphql

import (
	"context"

	"github.com/kubebuild/agent/pkg/types"
)

//BuildWithID Build just with ID
type BuildWithID struct {
	ID types.ID
}

type updateBuilMutation struct {
	UpdateClusterBuild struct {
		Successful types.Boolean
		Result     BuildWithID
	} `graphql:"updateClusterBuild(clusterToken: $clusterToken, buildId: $buildId, workflow: $workflow, pipeupWorkflow: $pipeupWorkflow, startedAt: $startedAt, state: $state, finishedAt: $finishedAt, uploadPipeline: $uploadPipeline, resumeSuspended: $resumeSuspended)"`
}

// BuildMutationParams some params for build mutation
type BuildMutationParams struct {
	BuildID         types.ID
	ClusterToken    types.String
	Workflow        *types.JSON
	StartedAt       *types.DateTime
	FinishedAt      *types.DateTime
	State           types.String
	UploadPipeline  *types.Boolean
	ResumeSuspended *types.Boolean
	PipeupWorkflow  *types.JSON
}

//UpdateClusterBuild variations
func (m *Client) UpdateClusterBuild(params BuildMutationParams) (BuildWithID, error) {
	buildMutation := &updateBuilMutation{}

	if params.StartedAt != nil && params.StartedAt.IsZero() {
		params.StartedAt = nil
	}
	if params.FinishedAt != nil && params.FinishedAt.IsZero() {
		params.FinishedAt = nil
	}
	variables := map[string]interface{}{
		"buildId":         params.BuildID,
		"clusterToken":    params.ClusterToken,
		"workflow":        params.Workflow,
		"state":           params.State,
		"uploadPipeline":  params.UploadPipeline,
		"startedAt":       params.StartedAt,
		"finishedAt":      params.FinishedAt,
		"pipeupWorkflow":  params.PipeupWorkflow,
		"resumeSuspended": params.ResumeSuspended,
	}
	err := m.GraphqlClient.Mutate(context.Background(), buildMutation, variables)
	if err != nil {
		m.Log.WithError(err).Error("Build update request failed")
		return BuildWithID{}, err
	}
	if !buildMutation.UpdateClusterBuild.Successful {
		m.Log.Error("Update build failed")
	}
	return buildMutation.UpdateClusterBuild.Result, nil
}
