package mutations

import (
	"context"

	"github.com/sirupsen/logrus"

	"github.com/kubebuild/agent/pkg/types"
	"github.com/kubebuild/agent/pkg/utils"
	"github.com/shurcooL/graphql"
)

//Cluster Type
type Cluster struct {
	ID        types.ID
	Name      types.String
	Token     types.String
	Connected types.Boolean
}

// ClusterMutation struct
type ClusterMutation struct {
	ConnectCluster struct {
		Successful types.Boolean
		Result     Cluster
	} `graphql:"connectCluster(token: $token, logRegion: $logRegion)"`
}

// ConnectCluster mutation
func ConnectCluster(clusterToken string, graphqlClient *graphql.Client, log *logrus.Logger) Cluster {
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
		log.WithError(err).Error("Cluster request failed")
		panic("Cannot get cluster")
	}
	if !connectClusterMutation.ConnectCluster.Successful {
		log.Error("Connect cluster failed")
		panic("connect cluster failed")
	}
	return connectClusterMutation.ConnectCluster.Result
}
