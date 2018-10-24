package schedulers

import (
	"github.com/shurcooL/graphql"
	"github.com/sirupsen/logrus"
)

// CleanWorkflowsScheduler schedule your builds
type CleanWorkflowsScheduler struct {
	log           *logrus.Logger
	graphqlClient *graphql.Client
}

// NewCleanWorkflowsScheduler schedule builds
func NewCleanWorkflowsScheduler(log *logrus.Logger, graphqlClient *graphql.Client) *CleanWorkflowsScheduler {
	return &CleanWorkflowsScheduler{
		log:           log,
		graphqlClient: graphqlClient,
	}
}

// Start starts a timer loop querying builds.
func (b *CleanWorkflowsScheduler) Start() {
}
