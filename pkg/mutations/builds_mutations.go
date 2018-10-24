package mutations

import (
	"context"

	"github.com/kubebuild/agent/pkg/types"
	"github.com/shurcooL/graphql"
	"github.com/sirupsen/logrus"
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
	Workflow      types.WorkflowYaml
	StartedAt     types.DateTime
	FinishedAt    types.DateTime
	State         types.String
	LogsFinalized types.Boolean
}

//UpdateClusterBuild variations
func UpdateClusterBuild(params BuildMutationParams, graphqlClient *graphql.Client, log *logrus.Logger) BuildWithID {
	buildMutation := &updateBuilMutation{}

	variables := map[string]interface{}{
		"buildId":       params.BuildID,
		"clusterToken":  params.ClusterToken,
		"workflow":      params.Workflow,
		"startedAt":     params.StartedAt,
		"finishedAt":    params.FinishedAt,
		"state":         params.State,
		"logsFinalized": params.LogsFinalized,
	}
	err := graphqlClient.Mutate(context.Background(), buildMutation, variables)
	if err != nil {
		log.WithError(err).Error("Build update request failed")
	}
	if !buildMutation.UpdateClusterBuild.Successful {
		log.Error("Update build failed")
	}
	return buildMutation.UpdateClusterBuild.Result
}
