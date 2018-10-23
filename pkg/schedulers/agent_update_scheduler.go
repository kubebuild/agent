package schedulers

import (
	"github.com/Sirupsen/logrus"
	"github.com/shurcooL/graphql"
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
