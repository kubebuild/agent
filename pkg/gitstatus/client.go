package gitstatus

import (
	"context"
	"fmt"
	"regexp"

	"github.com/google/go-github/github"
	"github.com/gosimple/slug"
	"github.com/kubebuild/agent/pkg/graphql"
	"github.com/kubebuild/agent/pkg/types"
	"github.com/sirupsen/logrus"
	"github.com/xanzy/go-gitlab"
	"golang.org/x/oauth2"

	wfv1 "github.com/argoproj/argo/pkg/apis/workflow/v1alpha1"
)

//Constants for gitproivers
const (
	GithubProvider = "github"
	GitlabProvider = "gitlab"
)

//Client Struct
type Client struct {
	Log          *logrus.Logger
	GithubClient *github.Client
	GitlabClient *gitlab.Client
}

//NewGithubClient returns client for github
func NewGithubClient(log *logrus.Logger, organization graphql.Organization) *Client {
	client := &Client{
		Log: log,
	}
	for _, auth := range organization.Authentications {
		switch auth.Provider {
		case GithubProvider:
			ctx := context.Background()
			ts := oauth2.StaticTokenSource(
				&oauth2.Token{AccessToken: string(auth.Token)},
			)
			tc := oauth2.NewClient(ctx, ts)

			githubClient := github.NewClient(tc)
			client.GithubClient = githubClient
		case GitlabProvider:
			gitlabClient := gitlab.NewClient(nil, string(auth.Token))
			gitlabClient.SetBaseURL(string(*auth.GitlabURL))

			client.GitlabClient = gitlabClient
		default:
			log.Debug("no git provider skipping clinet")
		}

	}

	return client

}

func extractRepoAndOwner(gitURL string) (string, string) {
	var extractRepo = regexp.MustCompile(`^https|git:\/\/|@[^\/:]+[\/:]([^\/:]+)\/(.+).git$`)
	match := extractRepo.FindAllStringSubmatch(gitURL, -1)[0]
	return match[1], match[2]
}

func getURL(buildID string, pipeline graphql.Pipeline) *string {
	ret := fmt.Sprintf("https://www.kubebuild.com/dashboard/organization/%s/pipeline/%s/build/%s", pipeline.Organization.ID, pipeline.ID, buildID)
	return &ret
}

func gitlabState(phase wfv1.NodePhase) gitlab.BuildStateValue {
	switch phase {
	case wfv1.NodeError:
		return gitlab.Failed
	case wfv1.NodeFailed:
		return gitlab.Failed
	case wfv1.NodeSucceeded:
		return gitlab.Success
	case wfv1.NodeSkipped:
		return gitlab.Skipped
	default:
		return gitlab.Pending
	}
}

func getState(phase wfv1.NodePhase) *string {
	pending := "pending"
	success := "success"
	errorState := "error"
	failure := "failure"
	switch phase {
	case wfv1.NodeError:
		return &errorState
	case wfv1.NodeFailed:
		return &failure
	case wfv1.NodeSucceeded:
		return &success
	case wfv1.NodeSkipped:
		return &success
	default:
		return &pending
	}
}

//SendNotification send notification to github
func (c *Client) SendNotification(wf *wfv1.Workflow, commitSha types.String, buildIDGql types.ID, pipeline graphql.Pipeline) {
	owner, repo := extractRepoAndOwner(string(pipeline.GitURL))
	commit := string(commitSha)
	buildID := fmt.Sprintf("%s", buildIDGql)

	if c.GithubClient != nil && pipeline.Repository == GithubProvider {

		buildContext := fmt.Sprintf("kubebuild/%s", slug.Make(string(pipeline.Name)))

		status := &github.RepoStatus{
			TargetURL:   getURL(buildID, pipeline),
			State:       getState(wf.Status.Phase),
			Description: getState(wf.Status.Phase),
			Context:     &buildContext,
		}

		c.Log.WithField("status", wf.Status.Phase).Info(status)

		_, _, err := c.GithubClient.Repositories.CreateStatus(context.Background(), owner, repo, commit, status)
		if err != nil {
			c.Log.WithError(err).Error("failed to send status to github")
		}
	}
	if c.GitlabClient != nil && pipeline.Repository == GitlabProvider {
		opts := &gitlab.SetCommitStatusOptions{
			State: gitlabState(wf.Status.Phase),
		}
		c.GitlabClient.Commits.SetCommitStatus(repo, commit, opts)
	}
}
