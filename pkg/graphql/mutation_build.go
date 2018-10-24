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
	} `graphql:"updateClusterBuild(clusterToken: $clusterToken, buildId: $buildId, workflow: $workflow, startedAt: $startedAt, state: $state, finishedAt: $finishedAt, logsFinalized: $logsFinalized)"`
}

// BuildMutationParams some params for build mutation
type BuildMutationParams struct {
	BuildID       types.ID
	ClusterToken  types.String
	Workflow      types.JSON
	StartedAt     types.DateTime
	FinishedAt    types.DateTime
	State         types.String
	LogsFinalized types.Boolean
}

//UpdateClusterBuild variations
func (m *Client) UpdateClusterBuild(params BuildMutationParams) (BuildWithID, error) {
	buildMutation := &updateBuilMutation{}

	variables := map[string]interface{}{
		"buildId":       params.BuildID,
		"clusterToken":  params.ClusterToken,
		"workflow":      params.Workflow,
		"state":         params.State,
		"logsFinalized": params.LogsFinalized,
	}
	if !params.StartedAt.IsZero() {
		variables["startedAt"] = params.StartedAt
	}
	if !params.FinishedAt.IsZero() {
		variables["finishedAt"] = params.FinishedAt
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
