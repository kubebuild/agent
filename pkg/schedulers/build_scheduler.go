package schedulers

import (
	"github.com/Sirupsen/logrus"
	"github.com/kubebuild/agent/pkg/queries"
	"github.com/kubebuild/agent/pkg/utils"
	"github.com/shurcooL/graphql"
)

// BuildScheduler schedule your builds
type BuildScheduler struct {
	log           *logrus.Logger
	graphqlClient *graphql.Client
	token         string
}

// NewBuildScheduler schedule builds
func NewBuildScheduler(token string, log *logrus.Logger, graphqlClient *graphql.Client) *BuildScheduler {
	return &BuildScheduler{
		log:           log,
		graphqlClient: graphqlClient,
		token:         token,
	}
}

// Start starts a timer loop querying builds.
func (b *BuildScheduler) Start() {
	utils.SetInterval(func() {
		result, _ := queries.GetBuilds(b.token, b.graphqlClient, b.log)
		b.log.Debug(result)
	}, 2000, false)
}
