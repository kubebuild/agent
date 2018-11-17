package schedulers

import (
	"fmt"
	"strconv"
	"time"

	wfv1 "github.com/argoproj/argo/pkg/apis/workflow/v1alpha1"
	"github.com/argoproj/argo/workflow/util"
	"github.com/kubebuild/agent/pkg/graphql"
	"github.com/kubebuild/agent/pkg/types"
	"github.com/kubebuild/agent/pkg/utils"
)

// Build labels
var (
	BuildIDLabel     = "kubebuild.com/build-id"
	BuildNumberLabel = "kubebuild.com/build-number"
)

// GetBuildOpts lol
func GetBuildOpts(cluster graphql.Cluster, build graphql.ScheduledBuild) *util.SubmitOpts {
	opts := &util.SubmitOpts{
		InstanceID:     string(cluster.Name),
		ServiceAccount: fmt.Sprintf("%s-kubebuild-agent", cluster.Name),
		Parameters:     getParams(cluster, build),
	}
	return opts
}

func getParams(cluster graphql.Cluster, build graphql.ScheduledBuild) []string {
	buildID := fmt.Sprintf("buildID=%s", build.ID)
	repo := fmt.Sprintf("repo=%s", build.Pipeline.GitURL)
	revision := fmt.Sprintf("revision=%s", build.Commit)
	buildNumber := fmt.Sprintf("buildNumber=%d", build.BuildNumber)
	branch := fmt.Sprintf("branch=%s", build.Branch)
	clusterToken := fmt.Sprintf("clusterToken=%s", cluster.Token)
	params := []string{buildID, repo, revision, buildNumber, branch, clusterToken}
	if build.Pipeline.GitSecretName != nil {
		gitSecretName := fmt.Sprintf("gitSecretName=%s", *build.Pipeline.GitSecretName)
		params = append(params, gitSecretName)
	}

	return params
}

// AddBuildLabels adds the labels for the build
func AddBuildLabels(build graphql.ScheduledBuild, wf *wfv1.Workflow) {
	labels := wf.GetLabels()
	if labels == nil {
		labels = make(map[string]string)
	}

	labels[BuildIDLabel] = build.ID.(string)
	labels[BuildNumberLabel] = strconv.Itoa(int(build.BuildNumber))
	wf.SetLabels(labels)
	ttlWf := int32(2 * 60 * 60)
	wf.Spec.TTLSecondsAfterFinished = &ttlWf
}

// FailBuild build fail on error
func (b *BuildScheduler) FailBuild(buildID string, wf *wfv1.Workflow, err error) {
	wf.Status.Message = err.Error()

	params := b.defaultParams(buildID, wf)
	params.State = types.String(utils.Failed)
	params.StartedAt = &types.DateTime{Time: time.Now().UTC()}
	params.FinishedAt = &types.DateTime{Time: time.Now().UTC()}

	b.graphqlClient.UpdateClusterBuild(params)
}
