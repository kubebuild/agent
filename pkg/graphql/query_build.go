package graphql

import (
	"context"

	"github.com/kubebuild/agent/pkg/types"
)

type cluster struct {
	LogAwsKey    types.String
	LogAwsSecret types.String
}

//Authentication auth for github
type Authentication struct {
	Provider  types.String
	Token     types.String
	GitlabURL *types.String
}

//Organization with auth
type Organization struct {
	ID              types.ID
	Name            types.String
	Authentications []*Authentication
}

//Pipeline struct containing pipe data
type Pipeline struct {
	ID            types.ID
	Name          types.String
	GitURL        types.String
	Repository    types.String
	GitSecretName *types.String
	EnvConfigMap  *types.String
	Organization  Organization
}

//ScheduledBuild type for scheduled builds
type ScheduledBuild struct {
	ID             types.ID
	BuildNumber    types.Int
	Branch         types.String
	Commit         types.String
	UploadPipeline types.Boolean
	IsPullRequest  types.Boolean
	Template       *types.WorkflowYaml
	PipeupWorkflow *types.JSON
	Pipeline       Pipeline
}

//CancelingBuild struct for running build info
type CancelingBuild struct {
	ID        types.ID
	Cluster   cluster
	Commit    types.String
	StartedAt types.DateTime
	Workflow  types.JSON
	Pipeline  Pipeline
}

//RunningBuild struct for running build info
type RunningBuild struct {
	ID        types.ID
	Cluster   cluster
	Commit    types.String
	LogRegion types.String
	StartedAt types.DateTime
	Workflow  types.JSON
	Pipeline  Pipeline
}

//BlockedBuild to resume suspended
type BlockedBuild struct {
	ID              types.ID
	ResumeSuspended types.Boolean
	Workflow        types.JSON
	StartedAt       types.DateTime
}

//RetryBuild build to retry
type RetryBuild struct {
	ID        types.ID
	Workflow  types.JSON
	StartedAt types.DateTime
}

// BuildQuery query for builds
type BuildQuery struct {
	Scheduled []ScheduledBuild `graphql:"scheduled: buildsInCluster(clusterToken: $clusterToken, buildState: SCHEDULED)"`
	Running   []RunningBuild   `graphql:"running: buildsInCluster(clusterToken: $clusterToken, buildState: RUNNING)"`
	Blocked   []BlockedBuild   `graphql:"blocked: buildsInCluster(clusterToken: $clusterToken, buildState: BLOCKED)"`
	Canceling []CancelingBuild `graphql:"canceling: buildsInCluster(clusterToken: $clusterToken, buildState: CANCELING)"`
	Retry     []RetryBuild     `graphql:"retry: buildsInCluster(clusterToken: $clusterToken, buildState: RETRYING)"`
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
