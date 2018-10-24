package graphql

import (
	"context"

	"github.com/kubebuild/agent/pkg/types"
	"github.com/kubebuild/agent/pkg/utils"
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
func (m *Client) ConnectCluster(clusterToken string) Cluster {
	regions := utils.CalcLatency(1, false, "s3")
	logRegion := regions.Get(0).Code
	m.Log.WithField("logRegion", logRegion).Debug("Log region")
	connectClusterMutation := &ClusterMutation{}
	variables := map[string]interface{}{
		"token":     clusterToken,
		"logRegion": logRegion,
	}
	err := m.GraphqlClient.Mutate(context.Background(), connectClusterMutation, variables)
	if err != nil {
		m.Log.WithError(err).Error("Cluster request failed")
		panic("Cannot get cluster")
	}
	if !connectClusterMutation.ConnectCluster.Successful {
		m.Log.Error("Connect cluster failed")
		panic("connect cluster failed")
	}
	m.Cluster = connectClusterMutation.ConnectCluster.Result
	return m.Cluster
}
