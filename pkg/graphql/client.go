package graphql

import (
	"github.com/shurcooL/graphql"
	"github.com/sirupsen/logrus"
)

// Client hold mutation actions
type Client struct {
	GraphqlClient *graphql.Client
	Log           *logrus.Logger
	Cluster       Cluster
}

// NewGraphqlClient Return a new graphql client
func NewGraphqlClient(gqlClient *graphql.Client, log *logrus.Logger) *Client {
	return &Client{
		GraphqlClient: gqlClient,
		Log:           log,
	}
}
