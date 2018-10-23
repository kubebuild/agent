package queries

import (
	"context"

	"github.com/Sirupsen/logrus"
	"github.com/shurcooL/graphql"
)

type cluster struct {
	LogRegion    String
	LogAwsKey    String
	LogAwsSecret String
}

// BuildQuery query for builds
type BuildQuery struct {
	Scheduled struct {
		ID             ID
		BuildNumber    Int
		Branch         String
		Commit         String
		UploadPipeline Boolean
		Template       WorkflowSpec
		Pipeline       struct {
			GitURL        String
			GitSecretName String
		}
	} `graphql:"scheduled: buildsInCluster(clusterToken: $clusterToken, buildState: SCHEDULED)"`
	Running struct {
		ID      graphql.ID
		Cluster cluster
	} `graphql:"running: buildsInCluster(clusterToken: $clusterToken, buildState: RUNNING)"`
	Blocked struct {
		ID              ID
		FinishedAt      DateTime
		ResumeSuspended Boolean
	} `graphql:"blocked: buildsInCluster(clusterToken: $clusterToken, buildState: BLOCKED)"`
	Logs struct {
		ID          ID
		State       String
		StartedAt   DateTime
		FinishedAt  DateTime
		ScheduledAt DateTime
		Cluster     cluster
	} `graphql:"logs: buildsForLogs(clusterToken: $clusterToken)"`
}

// GetBuilds return the builds query
func GetBuilds(clusterToken string, client *graphql.Client, log *logrus.Logger) *BuildQuery {
	q := &BuildQuery{}
	variables := map[string]interface{}{
		"clusterToken": clusterToken,
	}
	log.Info(variables)
	err := client.Query(context.Background(), q, variables)
	if err != nil {
		log.WithError(err).Error("Builds Query failed")
	}
	return q
}
