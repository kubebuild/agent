package schedulers

import (
	"github.com/Sirupsen/logrus"
	"github.com/kubebuild/agent/pkg/mutations"
	"github.com/kubebuild/agent/pkg/queries"
	"github.com/kubebuild/agent/pkg/utils"
	"github.com/shurcooL/graphql"
)

// BuildScheduler schedule your builds
type BuildScheduler struct {
	log           *logrus.Logger
	graphqlClient *graphql.Client
	cluster       mutations.Cluster
}

// NewBuildScheduler schedule builds
func NewBuildScheduler(cluster mutations.Cluster, log *logrus.Logger, graphqlClient *graphql.Client) *BuildScheduler {
	return &BuildScheduler{
		log:           log,
		graphqlClient: graphqlClient,
		cluster:       cluster,
	}
}

// Start starts a timer loop querying builds.
func (b *BuildScheduler) Start() {
	utils.SetInterval(func() {
		result, _ := queries.GetBuilds(b.cluster, b.graphqlClient, b.log)
		for _, build := range result.Scheduled {
			if build.UploadPipeline {

			}
		}
	}, 2000, false)
}
