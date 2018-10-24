package schedulers

import (
	"github.com/shurcooL/graphql"
	"github.com/sirupsen/logrus"
)

// AgentUpdateScheduler schedule your agent updates
type AgentUpdateScheduler struct {
	log           *logrus.Logger
	graphqlClient *graphql.Client
}

// NewAgentUpdateScheduler schedule builds
func NewAgentUpdateScheduler(log *logrus.Logger, graphqlClient *graphql.Client) *AgentUpdateScheduler {
	return &AgentUpdateScheduler{
		log:           log,
		graphqlClient: graphqlClient,
	}
}

// Start starts a timer loop querying builds.
func (b *AgentUpdateScheduler) Start() {
}
