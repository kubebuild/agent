package graphql

import (
	"context"

	"github.com/kubebuild/agent/pkg/types"
)

type cluster struct {
	LogRegion    types.String
	LogAwsKey    types.String
	LogAwsSecret types.String
}

//ScheduledBuild type for scheduled builds
type ScheduledBuild struct {
	ID             types.ID
	BuildNumber    types.Int
	Branch         types.String
	Commit         types.String
	UploadPipeline types.Boolean
	Template       types.WorkflowYaml
	Pipeline       struct {
		GitURL        types.String
		GitSecretName types.String
	}
}

//RunningBuild struct for running build info
type RunningBuild struct {
	ID      types.ID
	Cluster cluster
}

// BuildQuery query for builds
type BuildQuery struct {
	Scheduled []ScheduledBuild `graphql:"scheduled: buildsInCluster(clusterToken: $clusterToken, buildState: SCHEDULED)"`
	Running   []RunningBuild   `graphql:"running: buildsInCluster(clusterToken: $clusterToken, buildState: RUNNING)"`
	Blocked   []struct {
		ID              types.ID
		FinishedAt      types.DateTime
		ResumeSuspended types.Boolean
	} `graphql:"blocked: buildsInCluster(clusterToken: $clusterToken, buildState: BLOCKED)"`
	Logs []struct {
		ID          types.ID
		State       types.String
		StartedAt   types.DateTime
		FinishedAt  types.DateTime
		ScheduledAt types.DateTime
		Cluster     cluster
	} `graphql:"logs: buildsForLogs(clusterToken: $clusterToken)"`
}

// GetBuilds return the builds query
func (c *Client) GetBuilds() (*BuildQuery, error) {
	q := &BuildQuery{}
	variables := map[string]interface{}{
		"clusterToken": c.Cluster.Token,
	}
	err := c.GraphqlClient.Query(context.Background(), q, variables)
	if err != nil {
		c.Log.WithError(err).Error("BuildsQuery Failed")
		return nil, err
	}
	return q, nil
}
