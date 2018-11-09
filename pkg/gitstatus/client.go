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
	"golang.org/x/oauth2"

	wfv1 "github.com/argoproj/argo/pkg/apis/workflow/v1alpha1"
)

//Client Struct
type Client struct {
	Log          *logrus.Logger
	GithubClient *github.Client
}

//NewGithubClient returns client for github
func NewGithubClient(log *logrus.Logger, organization graphql.Organization) *Client {
	if len(organization.Authentications) > 0 &&
		organization.Authentications[0].Provider == "github" {
		ctx := context.Background()
		ts := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: string(organization.Authentications[0].Token)},
		)
		tc := oauth2.NewClient(ctx, ts)

		client := github.NewClient(tc)

		return &Client{
			Log:          log,
			GithubClient: client,
		}
	}
	return &Client{
		Log: log,
	}
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
func (c *Client) SendNotification(wf *wfv1.Workflow, commit types.String, buildID types.ID, pipeline graphql.Pipeline) {
	if c.GithubClient != nil {
		commit := string(commit)
		buildID := fmt.Sprintf("%s", buildID)
		owner, repo := extractRepoAndOwner(string(pipeline.GitURL))

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
}
