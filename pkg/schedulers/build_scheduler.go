package schedulers

import (
	"fmt"
	"sync"
	"time"

	"github.com/kubebuild/agent/pkg/gitstatus"
	"github.com/kubebuild/agent/pkg/workflow"

	"k8s.io/client-go/kubernetes"

	wfv1 "github.com/argoproj/argo/pkg/apis/workflow/v1alpha1"
	"github.com/argoproj/argo/pkg/client/clientset/versioned/typed/workflow/v1alpha1"
	"github.com/argoproj/argo/workflow/util"
	"github.com/kubebuild/agent/pkg/graphql"
	"github.com/kubebuild/agent/pkg/types"
	"github.com/kubebuild/agent/pkg/utils"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var shaMutex = &sync.Mutex{}

// BuildScheduler schedule your builds
type BuildScheduler struct {
	log            *logrus.Logger
	graphqlClient  *graphql.Client
	cluster        graphql.Cluster
	workflowClient v1alpha1.WorkflowInterface
	kubeClient     kubernetes.Interface
	logUploader    *workflow.LogUploader
}

var shaMap = make(map[string]map[string]string)

// NewBuildScheduler schedule builds
func NewBuildScheduler(workflowClient v1alpha1.WorkflowInterface, graphqlClient *graphql.Client, kubeClient kubernetes.Interface, log *logrus.Logger) *BuildScheduler {
	logUploader := workflow.NewLogUploader(graphqlClient.Cluster, kubeClient, log)
	return &BuildScheduler{
		log:            log,
		graphqlClient:  graphqlClient,
		cluster:        graphqlClient.Cluster,
		workflowClient: workflowClient,
		kubeClient:     kubeClient,
		logUploader:    logUploader,
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
		var cancelingWg sync.WaitGroup
		cancelingWg.Add(len(result.Canceling))
		for i := range result.Canceling {
			go func(i int) {
				defer cancelingWg.Done()
				build := result.Canceling[i]
				b.cancelBuild(build)
			}(i)
		}
		cancelingWg.Wait()
		var scheduledWg sync.WaitGroup
		scheduledWg.Add(len(result.Scheduled))
		for i := range result.Scheduled {
			go func(i int) {
				defer scheduledWg.Done()
				build := result.Scheduled[i]
				b.scheduleBuild(build)
			}(i)
		}
		scheduledWg.Wait()
		var runningWg sync.WaitGroup
		runningWg.Add(len(result.Running))
		for i := range result.Running {
			go func(i int) {
				defer runningWg.Done()
				build := result.Running[i]
				b.runningBuild(build)
			}(i)
		}
		runningWg.Wait()
		var retryWg sync.WaitGroup
		retryWg.Add(len(result.Retry))
		for i := range result.Retry {
			go func(i int) {
				defer retryWg.Done()
				build := result.Retry[i]
				b.retryBuild(build)
			}(i)
		}
		retryWg.Wait()
		var blockedWg sync.WaitGroup
		blockedWg.Add(len(result.Blocked))
		for i := range result.Blocked {
			go func(i int) {
				defer blockedWg.Done()
				build := result.Blocked[i]
				b.resumeSuspended(build)
			}(i)
		}
		blockedWg.Wait()
	}, 1000, false)
}

func (b *BuildScheduler) defaultParams(buildID types.ID, wf *wfv1.Workflow) graphql.BuildMutationParams {
	return graphql.BuildMutationParams{
		BuildID:      buildID,
		Workflow:     &types.JSON{Workflow: wf},
		ClusterToken: b.cluster.Token,
		State:        utils.MapPhaseToState(wf.Status.Phase, false),
	}
}

func (b *BuildScheduler) ensureTerminated(wf *wfv1.Workflow) bool {
	var podNodes []wfv1.NodeStatus
	for _, node := range wf.Status.Nodes {
		if node.Type == wfv1.NodeTypePod &&
			(node.Phase == wfv1.NodeError || node.Phase == wfv1.NodeFailed) {
			podNodes = append(podNodes, node)
		}
	}

	var mux sync.Mutex
	var terminatedPods []string
	var wg sync.WaitGroup
	wg.Add(len(podNodes))

	for i := range podNodes {
		node := podNodes[i]
		podName := node.ID
		podNamespace := wf.Namespace
		go func() {
			defer wg.Done()

			pod, err := b.kubeClient.CoreV1().Pods(podNamespace).Get(podName, metav1.GetOptions{})
			if err != nil {
				mux.Lock()
				terminatedPods = append(terminatedPods, pod.Name)
				mux.Unlock()
				return
			}
			if pod.Status.Phase == "Pending" {
				return
			}
			mux.Lock()
			terminatedPods = append(terminatedPods, pod.Name)
			mux.Unlock()
			return
		}()

	}
	wg.Wait()
	return len(podNodes) == len(terminatedPods)
}

func (b *BuildScheduler) retryBuild(build graphql.RetryBuild) {
	b.log.WithField("buildID", build.ID).Debug("retry")
	wf := build.Workflow.Workflow

	if b.ensureTerminated(wf) {
		newWf, err := util.RetryWorkflow(b.kubeClient, b.workflowClient, wf)
		if err != nil {
			b.updateRetry(build, wf, types.String(utils.Failed))
			b.log.WithError(err).Error("cannot build retry wf")
			return
		}

		b.updateRetry(build, newWf, types.String(utils.Running))
	}
}

func (b *BuildScheduler) updateRetry(build graphql.RetryBuild, wf *wfv1.Workflow, state types.String) {
	params := b.defaultParams(build.ID, wf)
	params.StartedAt = &types.DateTime{Time: time.Now().UTC()}
	params.FinishedAt = nil
	params.State = state
	b.graphqlClient.UpdateClusterBuild(params)
}

func (b *BuildScheduler) cancelBuild(build graphql.CancelingBuild) {
	b.log.WithField("buildID", build.ID).Debug("canceling")
	wf := build.Workflow.Workflow
	err := util.TerminateWorkflow(b.workflowClient, wf.GetName())
	if err != nil {
		b.log.WithError(err).Error("cannot terminate wf")
	}
	for {
		newWf, err := b.workflowClient.Get(wf.GetName(), metav1.GetOptions{})
		if err != nil {
			b.log.WithError(err).Error("cannot get canceled wf")
			b.updateCanceled(build, wf)
			break
		}
		if util.IsWorkflowCompleted(newWf) {
			gitclient := gitstatus.NewGithubClient(b.log, build.Pipeline.Organization)
			if newWf.Status.Phase == "Succeeded" {
				params := b.defaultParams(build.ID, newWf)
				params.State = utils.MapPhaseToState(newWf.Status.Phase, false)
				params.FinishedAt = &types.DateTime{Time: time.Now().UTC()}
				b.graphqlClient.UpdateClusterBuild(params)
				go gitclient.SendNotification(wf, build.Commit, build.ID, build.Pipeline)
				break
			}
			b.updateCanceled(build, newWf)
			break
		}
	}

}

func (b *BuildScheduler) updateCanceled(build graphql.CancelingBuild, wf *wfv1.Workflow) {
	params := b.defaultParams(build.ID, wf)
	params.StartedAt = &types.DateTime{Time: build.StartedAt.Time.UTC()}
	params.State = types.String(utils.Canceled)
	params.CanceledAt = &types.DateTime{Time: time.Now().UTC()}
	b.graphqlClient.UpdateClusterBuild(params)
}

func (b *BuildScheduler) scheduleBuild(build graphql.ScheduledBuild) {
	b.log.WithField("buildID", build.ID).Debug("schedule")
	buildOps := GetBuildOpts(b.cluster, build)
	if build.UploadPipeline {
		b.buildWithUploadPipeline(build, buildOps)
	} else {
		b.scheduleBuildWithExistingWf(build, buildOps)
	}
}

func (b *BuildScheduler) runningBuild(build graphql.RunningBuild) {
	b.log.WithField("buildID", build.ID).Debug("running")
	wf := build.Workflow.Workflow
	newWf, err := b.workflowClient.Get(wf.GetName(), metav1.GetOptions{})
	if err != nil {
		b.log.WithError(err).Error("cannot get wf")
	}
	params := b.defaultParams(build.ID, newWf)
	params.StartedAt = &types.DateTime{Time: build.StartedAt.Time.UTC()}
	if util.IsWorkflowSuspended(newWf) {
		params.State = utils.MapPhaseToState(newWf.Status.Phase, true)
		params.FinishedAt = &types.DateTime{Time: time.Now().UTC()}
	}
	if util.IsWorkflowCompleted(newWf) {
		params.FinishedAt = &types.DateTime{Time: newWf.Status.FinishedAt.Time.UTC()}
	}
	b.graphqlClient.UpdateClusterBuild(params)
	buildID := fmt.Sprintf("%s", build.ID)
	if shaMap[buildID] == nil {
		shaMutex.Lock()
		shaMap[buildID] = make(map[string]string)
		shaMutex.Unlock()
	}

	gitclient := gitstatus.NewGithubClient(b.log, build.Pipeline.Organization)
	go gitclient.SendNotification(newWf, build.Commit, build.ID, build.Pipeline)

	b.logUploader.UploadWorkflowLogs(newWf, build, shaMap[buildID], shaMutex)
	if util.IsWorkflowCompleted(newWf) {
		shaMutex.Lock()
		delete(shaMap, buildID)
		shaMutex.Unlock()
	}
}

func (b *BuildScheduler) resumeSuspended(build graphql.BlockedBuild) {
	if build.ResumeSuspended {
		b.log.WithField("buildID", build.ID).Debug("resuming build")
		wf := build.Workflow.Workflow
		err := util.ResumeWorkflow(b.workflowClient, wf.GetName())
		if err != nil {
			b.log.WithError(err).Error("could not resume build")
		}
		newWf, err := b.workflowClient.Get(wf.GetName(), metav1.GetOptions{})
		if err != nil {
			b.log.WithError(err).Error("cannot get workflow")
		}
		params := b.defaultParams(build.ID, newWf)
		params.StartedAt = &types.DateTime{Time: build.StartedAt.Time.UTC()}
		if util.IsWorkflowCompleted(newWf) {
			params.FinishedAt = &types.DateTime{Time: time.Now().UTC()}
		}
		b.graphqlClient.UpdateClusterBuild(params)
	}
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
			b.log.WithError(err).Error("cannot get workflow")
			b.FailBuild(build.ID, wf, err)
			return
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
			b.log.WithError(err).Error("pipeup workflow failed submit")
			b.FailBuild(build.ID, wf, err)
			return
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
	PrepareWorkflow(build, wf)
	newWf, err := util.SubmitWorkflow(b.workflowClient, wf, buildOps)
	if err != nil {
		b.log.WithError(err).Error("workflow failed submit")
		b.FailBuild(build.ID, wf, err)
		return
	}
	params := b.defaultParams(build.ID, newWf)
	params.State = types.String(utils.Running)
	params.StartedAt = &types.DateTime{Time: time.Now().UTC()}

	buildWithID, err := b.graphqlClient.UpdateClusterBuild(params)
	gitclient := gitstatus.NewGithubClient(b.log, build.Pipeline.Organization)
	go gitclient.SendNotification(newWf, build.Commit, build.ID, build.Pipeline)
	if err != nil {
		b.log.WithError(err).Error("failed to update build")
	}
	b.log.WithField("buildID", buildWithID.ID).Debug("updated")
}
