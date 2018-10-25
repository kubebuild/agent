package schedulers

import (
	"fmt"
	"strconv"

	wfv1 "github.com/argoproj/argo/pkg/apis/workflow/v1alpha1"
	"github.com/argoproj/argo/workflow/util"
	"github.com/kubebuild/agent/pkg/graphql"
)

// Build labels
var (
	BuildIDLabel       = "kubebuild.com/build-id"
	BuildNumberLabel   = "kubebuild.com/build-number"
	BuildUploaderLabel = "kubebuild.com/is-uploader"
	BuildBranchLabel   = "kubebuild.com/build-branch"
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
	gitSecretName := fmt.Sprintf("gitSecretName=%s", *build.Pipeline.GitSecretName)

	return []string{buildID, repo, revision, buildNumber, branch, clusterToken, gitSecretName}
}

// AddBuildLabels adds the labels for the build
func AddBuildLabels(build graphql.ScheduledBuild, isUploading bool, wf *wfv1.Workflow) {
	labels := wf.GetLabels()
	if labels == nil {
		labels = make(map[string]string)
	}

	labels[BuildIDLabel] = build.ID.(string)
	labels[BuildNumberLabel] = strconv.Itoa(int(build.BuildNumber))
	labels[BuildUploaderLabel] = strconv.FormatBool(isUploading)
	labels[BuildBranchLabel] = string(build.Branch)
	wf.SetLabels(labels)
	ttlWf := int32(2 * 60 * 60)
	wf.Spec.TTLSecondsAfterFinished = &ttlWf
}
