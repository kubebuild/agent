package schedulers

import (
	"time"

	"github.com/argoproj/argo/pkg/client/clientset/versioned/typed/workflow/v1alpha1"
	"github.com/argoproj/argo/workflow/util"
	"github.com/argoproj/argo/workflow/validate"
	"github.com/kubebuild/agent/pkg/graphql"
	"github.com/kubebuild/agent/pkg/types"
	"github.com/kubebuild/agent/pkg/utils"
	"github.com/sirupsen/logrus"
)

// BuildScheduler schedule your builds
type BuildScheduler struct {
	log            *logrus.Logger
	graphqlClient  *graphql.Client
	cluster        graphql.Cluster
	workflowClient v1alpha1.WorkflowInterface
}

// NewBuildScheduler schedule builds
func NewBuildScheduler(workflowClient v1alpha1.WorkflowInterface, graphqlClient *graphql.Client, log *logrus.Logger) *BuildScheduler {
	return &BuildScheduler{
		log:            log,
		graphqlClient:  graphqlClient,
		cluster:        graphqlClient.Cluster,
		workflowClient: workflowClient,
	}
}

// Start starts a timer loop querying builds.
func (b *BuildScheduler) Start() {
	utils.SetInterval(func() {
		result, err := b.graphqlClient.GetBuilds()
		if err != nil {
			b.log.WithError(err).Error("can't get builds")
			return
		}
		for _, build := range result.Scheduled {
			if build.UploadPipeline {

			} else {
				wf := build.Template.Workflow
				err := validate.ValidateWorkflow(wf, true)
				if err != nil {
					b.log.WithError(err).Error("Workflow failed validation")
					// Todo set Error label ?
				}
				AddBuildLabels(build, false, wf)
				buildOps := GetBuildOpts(b.cluster, build)
				wfresult, _ := util.SubmitWorkflow(b.workflowClient, wf, buildOps)
				b.log.Info(wfresult)
				params := graphql.BuildMutationParams{
					BuildID:      build.ID,
					Workflow:     types.JSON{wfresult},
					ClusterToken: b.cluster.Token,
					StartedAt:    types.DateTime{time.Now().UTC()},
					State:        "running",
				}
				build, err := b.graphqlClient.UpdateClusterBuild(params)
				if err != nil {
					b.log.WithError(err).Error("Failed to update build")
				}
				b.log.WithField("build", build.ID).Info("updated")
			}
		}
		for _, build := range result.Running {
			b.log.Info("Running")
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
