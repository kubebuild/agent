package mutations

import (
	"context"

	"github.com/Sirupsen/logrus"

	"github.com/kubebuild/agent/pkg/scalar"
	"github.com/kubebuild/agent/pkg/utils"
	"github.com/shurcooL/graphql"
)

//Cluster Type
type Cluster struct {
	ID        scalar.ID
	Name      scalar.String
	Token     scalar.String
	Connected scalar.Boolean
}

// ClusterMutation struct
type ClusterMutation struct {
	ConnectCluster struct {
		Successful scalar.Boolean
		Result     Cluster
	} `graphql:"connectCluster(token: $token, logRegion: $logRegion)"`
}

// ConnectCluster mutation
func ConnectCluster(clusterToken string, graphqlClient *graphql.Client, log *logrus.Logger) *ClusterMutation {
	regions := utils.CalcLatency(1, false, "s3")
	logRegion := regions.Get(0).Code
	log.WithField("logRegion", logRegion).Debug("Log region")
	connectClusterMutation := &ClusterMutation{}
	variables := map[string]interface{}{
		"token":     clusterToken,
		"logRegion": logRegion,
	}
	err := graphqlClient.Mutate(context.Background(), connectClusterMutation, variables)
	if err != nil {
		log.Error(err)
	}
	return connectClusterMutation
}
