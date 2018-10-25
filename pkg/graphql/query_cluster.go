package graphql

import (
	"context"

	"github.com/kubebuild/agent/pkg/types"
)

//ClusterConfig cluster config
type ClusterConfig struct {
	ExecutorImage types.String
	LauncherImage types.String
	WorkflowImage types.String
}

// ClusterByToken args for updater
type ClusterByToken struct {
	ClusterConfig ClusterConfig
}

// ClusterQuery query for builds
type ClusterQuery struct {
	Cluster ClusterByToken `graphql:"cluster: clusterByToken(token: $token)"`
}

// GetClusterConfig return the builds query
func (c *Client) GetClusterConfig() (ClusterConfig, error) {
	q := &ClusterQuery{}
	variables := map[string]interface{}{
		"token": c.Cluster.Token,
	}
	err := c.GraphqlClient.Query(context.Background(), q, variables)
	if err != nil {
		c.Log.WithError(err).Error("ClusterQuery Failed")
		return ClusterConfig{}, err
	}
	return q.Cluster.ClusterConfig, nil
}
