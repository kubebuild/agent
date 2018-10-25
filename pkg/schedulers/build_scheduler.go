package schedulers

import (
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/kubebuild/agent/pkg/workflow"

	"k8s.io/client-go/kubernetes"

	wfv1 "github.com/argoproj/argo/pkg/apis/workflow/v1alpha1"
	"github.com/argoproj/argo/pkg/client/clientset/versioned/typed/workflow/v1alpha1"
	"github.com/argoproj/argo/workflow/util"
	"github.com/argoproj/argo/workflow/validate"
	"github.com/kubebuild/agent/pkg/graphql"
	"github.com/kubebuild/agent/pkg/types"
	"github.com/kubebuild/agent/pkg/utils"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// BuildScheduler schedule your builds
type BuildScheduler struct {
	log            *logrus.Logger
	graphqlClient  *graphql.Client
	cluster        graphql.Cluster
	workflowClient v1alpha1.WorkflowInterface
	kubeClient     kubernetes.Interface
}

// NewBuildScheduler schedule builds
func NewBuildScheduler(workflowClient v1alpha1.WorkflowInterface, graphqlClient *graphql.Client, kubeClient kubernetes.Interface, log *logrus.Logger) *BuildScheduler {
	return &BuildScheduler{
		log:            log,
		graphqlClient:  graphqlClient,
		cluster:        graphqlClient.Cluster,
		workflowClient: workflowClient,
		kubeClient:     kubeClient,
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
			b.scheduleBuild(build)
		}
		for _, build := range result.Running {
			b.runningBuild(build)
		}
		for _, build := range result.Blocked {
			b.log.Debug(build)
		}
	}, 2000, false)
}

func (b *BuildScheduler) scheduleBuild(build graphql.ScheduledBuild) {
	b.log.WithField("buildID", build.ID).Info("Schedule")
	buildOps := GetBuildOpts(b.cluster, build)
	if build.UploadPipeline {
		b.buildWithUploadPipeline(build, buildOps)
	} else {
		b.scheduleBuildWithExistingWf(build, buildOps)
	}
}

func (b *BuildScheduler) runningBuild(build graphql.RunningBuild) {
	b.log.Info("Running")
	wf := build.Workflow.Workflow
	newWf, err := b.workflowClient.Get(wf.GetName(), metav1.GetOptions{})
	if err != nil {
		b.log.WithError(err).Error("cannot get wf")
	}
	params := graphql.BuildMutationParams{
		BuildID:      build.ID,
		Workflow:     &types.JSON{Workflow: newWf},
		ClusterToken: b.cluster.Token,
		State:        utils.MapPhaseToState(newWf.Status.Phase, false),
	}
	if util.IsWorkflowCompleted(newWf) {
		params.FinishedAt = &types.DateTime{Time: time.Now().UTC()}
	}
	b.graphqlClient.UpdateClusterBuild(params)
	b.uploadLogs(newWf, build)
}

func (b *BuildScheduler) buildWithUploadPipeline(build graphql.ScheduledBuild, buildOps *util.SubmitOpts) {
	uploadPipe := types.Boolean(true)
	params := graphql.BuildMutationParams{
		BuildID:        build.ID,
		ClusterToken:   b.cluster.Token,
		State:          types.String(utils.Scheduled),
		UploadPipeline: &uploadPipe,
	}
	if build.PipeupWorkflow != nil {
		wf := build.PipeupWorkflow.Workflow
		newWf, err := b.workflowClient.Get(wf.GetName(), metav1.GetOptions{})
		if err != nil {
			b.log.WithError(err).Error("cannot get wf")
		}

		params.PipeupWorkflow = &types.JSON{Workflow: newWf}
		if util.IsWorkflowCompleted(newWf) {
			finishedPipe := types.Boolean(false)
			params.UploadPipeline = &finishedPipe
		}

	} else {
		wf := b.createPipeUpTemplate(build)
		pipeResultWf, err := util.SubmitWorkflow(b.workflowClient, wf, buildOps)
		if err != nil {
			b.log.WithError(err).Error("pipe wf failed submit")
		}
		params.PipeupWorkflow = &types.JSON{Workflow: pipeResultWf}
	}
	b.graphqlClient.UpdateClusterBuild(params)
}

func (b *BuildScheduler) scheduleBuildWithExistingWf(build graphql.ScheduledBuild, buildOps *util.SubmitOpts) {
	template := build.Template
	if template == nil {
		b.log.Error("template is nil canont continue")
		return
	}
	wf := template.Workflow
	err := validate.ValidateWorkflow(wf, true)
	if err != nil {
		b.log.WithError(err).Error("workflow failed validation")
		// Todo set Error label ?
	}
	AddBuildLabels(build, false, wf)
	wfresult, err := util.SubmitWorkflow(b.workflowClient, wf, buildOps)
	if err != nil {
		b.log.WithError(err).Error("workflow failed submit")
	}
	params := graphql.BuildMutationParams{
		BuildID:      build.ID,
		ClusterToken: b.cluster.Token,
		State:        types.String(utils.Running),
		Workflow:     &types.JSON{Workflow: wfresult},
		StartedAt:    &types.DateTime{Time: time.Now().UTC()},
	}
	buildWithID, err := b.graphqlClient.UpdateClusterBuild(params)
	if err != nil {
		b.log.WithError(err).Error("Failed to update build")
	}
	b.log.WithField("buildID", buildWithID.ID).Info("updated")
}

func (b *BuildScheduler) uploadLogs(wf *wfv1.Workflow, build graphql.RunningBuild) {
	metaTime := &metav1.Time{Time: build.StartedAt.Time}
	logPrinter := workflow.NewLogPrinter(b.kubeClient, false, metaTime)

	bufferMap := logPrinter.GetWorkflowLogs(wf)

	creds := credentials.Value{
		AccessKeyID:     string(build.Cluster.LogAwsKey),
		SecretAccessKey: string(build.Cluster.LogAwsSecret),
	}

	sess := session.Must(session.NewSession(&aws.Config{
		Region:      aws.String(string(build.LogRegion)),
		Credentials: credentials.NewStaticCredentialsFromCreds(creds),
	}))

	bucket := fmt.Sprintf("kubebuild-logs-%s", build.LogRegion)
	svc := s3manager.NewUploader(sess)

	for k, v := range bufferMap {

		key := fmt.Sprintf("%s/%s/%s/main", b.cluster.Name, build.ID, k)
		_, err := svc.Upload(&s3manager.UploadInput{
			ACL:          aws.String("public-read"),
			CacheControl: aws.String("no-cache"),
			Expires:      aws.Time(time.Now().AddDate(0, 1, 0)),
			Bucket:       aws.String(bucket),
			Key:          aws.String(key),
			Body:         &v,
		})

		if err != nil {
			b.log.WithError(err).Error("failed to upload logs")
		}
	}
}
