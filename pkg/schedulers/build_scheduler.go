package schedulers

import (
	"github.com/argoproj/argo/pkg/client/clientset/versioned/typed/workflow/v1alpha1"
	"github.com/argoproj/argo/workflow/util"
	"github.com/kubebuild/agent/pkg/mutations"
	"github.com/kubebuild/agent/pkg/queries"
	"github.com/kubebuild/agent/pkg/utils"
	"github.com/shurcooL/graphql"
	"github.com/sirupsen/logrus"
)

// BuildScheduler schedule your builds
type BuildScheduler struct {
	log            *logrus.Logger
	graphqlClient  *graphql.Client
	cluster        mutations.Cluster
	workflowClient v1alpha1.WorkflowInterface
}

// NewBuildScheduler schedule builds
func NewBuildScheduler(cluster mutations.Cluster, workflowClient v1alpha1.WorkflowInterface, graphqlClient *graphql.Client, log *logrus.Logger) *BuildScheduler {
	return &BuildScheduler{
		log:            log,
		graphqlClient:  graphqlClient,
		cluster:        cluster,
		workflowClient: workflowClient,
	}
}

// Start starts a timer loop querying builds.
func (b *BuildScheduler) Start() {
	utils.SetInterval(func() {
		result, _ := queries.GetBuilds(b.cluster, b.graphqlClient, b.log)
		for _, build := range result.Scheduled {
			if build.UploadPipeline {

			} else {
				wf := build.Template.Workflow
				AddBuildLabels(build, false, wf)
				buildOps := GetBuildOpts(b.cluster, build)
				wfresult, _ := util.SubmitWorkflow(b.workflowClient, wf, buildOps)
				b.log.Info(wfresult)
			}
		}
		for _, build := range result.Running {
			b.log.Debug(build)
		}
		for _, build := range result.Blocked {
			b.log.Debug(build)
		}
		for _, build := range result.Logs {
			b.log.Debug(build)
		}
	}, 2000, false)
}
