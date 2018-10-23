package queries

import (
	"context"

	"github.com/kubebuild/agent/pkg/mutations"
	"github.com/kubebuild/agent/pkg/scalar"

	"github.com/Sirupsen/logrus"
	"github.com/shurcooL/graphql"
)

type cluster struct {
	LogRegion    scalar.String
	LogAwsKey    scalar.String
	LogAwsSecret scalar.String
}

// BuildQuery query for builds
type BuildQuery struct {
	Scheduled []struct {
		ID             scalar.ID
		BuildNumber    scalar.Int
		Branch         scalar.String
		Commit         scalar.String
		UploadPipeline scalar.Boolean
		Template       scalar.WorkflowYaml
		Pipeline       struct {
			GitURL        scalar.String
			GitSecretName scalar.String
		}
	} `graphql:"scheduled: buildsInCluster(clusterToken: $clusterToken, buildState: SCHEDULED)"`
	Running []struct {
		ID      scalar.ID
		Cluster cluster
	} `graphql:"running: buildsInCluster(clusterToken: $clusterToken, buildState: RUNNING)"`
	Blocked []struct {
		ID              scalar.ID
		FinishedAt      scalar.DateTime
		ResumeSuspended scalar.Boolean
	} `graphql:"blocked: buildsInCluster(clusterToken: $clusterToken, buildState: BLOCKED)"`
	Logs []struct {
		ID          scalar.ID
		State       scalar.String
		StartedAt   scalar.DateTime
		FinishedAt  scalar.DateTime
		ScheduledAt scalar.DateTime
		Cluster     cluster
	} `graphql:"logs: buildsForLogs(clusterToken: $clusterToken)"`
}

// GetBuilds return the builds query
func GetBuilds(cluster mutations.Cluster, client *graphql.Client, log *logrus.Logger) (*BuildQuery, error) {
	q := &BuildQuery{}
	variables := map[string]interface{}{
		"clusterToken": cluster.Token,
	}
	err := client.Query(context.Background(), q, variables)
	if err != nil {
		log.WithError(err).Error("BuildsQuery Failed")
		return nil, err
	}
	return q, nil
}
