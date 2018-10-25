package schedulers

import (
	"bytes"
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
		for _, build := range result.Logs {
			b.log.Debug(build)
		}
	}, 2000, false)
}

func (b *BuildScheduler) scheduleBuild(build graphql.ScheduledBuild) {
	b.log.WithField("buildID", build.ID).Info("Schedule")
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
		params := graphql.BuildMutationParams{
			BuildID:      build.ID,
			ClusterToken: b.cluster.Token,
			State:        types.String(utils.Running),
			Workflow:     &types.JSON{Workflow: wfresult},
			StartedAt:    &types.DateTime{Time: time.Now().UTC()},
		}
		build, err := b.graphqlClient.UpdateClusterBuild(params)
		if err != nil {
			b.log.WithError(err).Error("Failed to update build")
		}
		b.log.WithField("buildID", build.ID).Info("updated")
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

func (b *BuildScheduler) uploadLogs(wf *wfv1.Workflow, build graphql.RunningBuild) {
	metaTime := &metav1.Time{Time: build.StartedAt.Time}
	logPrinter := workflow.NewLogPrinter(b.kubeClient, false, metaTime)

	logEntries := logPrinter.PrintWorkflowLogs(wf)

	bufferMap := make(map[string]bytes.Buffer)

	for _, logEntry := range logEntries {
		buffer := bufferMap[logEntry.Pod]
		buffer.WriteString(logEntry.Line)
		buffer.WriteString("\n")
		bufferMap[logEntry.Pod] = buffer
	}

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
		result, err := svc.Upload(&s3manager.UploadInput{
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
		b.log.Debug(result)
	}
}
