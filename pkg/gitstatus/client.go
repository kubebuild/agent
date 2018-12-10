package gitstatus

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"sync"

	"github.com/google/go-github/github"
	"github.com/gosimple/slug"
	"github.com/kubebuild/agent/pkg/graphql"
	"github.com/kubebuild/agent/pkg/types"
	"github.com/sirupsen/logrus"
	"github.com/xanzy/go-gitlab"
	"golang.org/x/oauth2"

	wfv1 "github.com/argoproj/argo/pkg/apis/workflow/v1alpha1"
)

var statusMap = make(map[string]string)
var statusMutex = &sync.Mutex{}

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

//DeleteFromStatusMap deletes build when finished
func (c *Client) DeleteFromStatusMap(buildIDGql types.ID) {
	buildID := fmt.Sprintf("%s", buildIDGql)
	statusMutex.Lock()
	delete(statusMap, buildID)
	statusMutex.Unlock()
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

var (
	isSchemeRegExp = regexp.MustCompile(`^http[s]?:\/\/`)
	sshURLRegex    = regexp.MustCompile(`^git:\/\/|@[^\/:]+[\/:]([^\/:]+)\/(.+).git$`)
	httpURLRegex   = regexp.MustCompile(`^http[s]?:\/\/[\w\.]+\/([^\/:]+)\/(.+).git$`)
)

func extractRepoAndOwner(gitURL string) (string, string) {
	if isSchemeRegExp.MatchString(gitURL) {
		match := httpURLRegex.FindAllStringSubmatch(gitURL, -1)[0]
		return match[1], match[2]
	}
	match := sshURLRegex.FindAllStringSubmatch(gitURL, -1)[0]
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

func statusChanged(buildID string, nodePhase wfv1.NodePhase) bool {
	if statusMap[buildID] == string(nodePhase) {
		return false
	}
	statusMutex.Lock()
	statusMap[buildID] = string(nodePhase)
	statusMutex.Unlock()
	return true
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

func urlEncoded(str string) (string, error) {
	u, err := url.Parse(str)
	if err != nil {
		return "", err
	}
	return u.String(), nil
}

//SendNotification send notification to github
func (c *Client) SendNotification(wf *wfv1.Workflow, commitSha types.String, buildIDGql types.ID, buildBranch types.String, pipeline graphql.Pipeline) {

	buildID := fmt.Sprintf("%s", buildIDGql)
	if !statusChanged(buildID, wf.Status.Phase) {
		return
	}
	owner, repo := extractRepoAndOwner(string(pipeline.GitURL))
	commit := string(commitSha)
	url := getURL(buildID, pipeline)
	buildContext := fmt.Sprintf("kubebuild/%s", slug.Make(string(pipeline.Name)))

	if c.GithubClient != nil && pipeline.Repository == "GITHUB" {

		status := &github.RepoStatus{
			TargetURL:   url,
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
	if c.GitlabClient != nil && pipeline.Repository == "GITLAB" {
		branch := string(buildBranch)
		opts := &gitlab.SetCommitStatusOptions{
			TargetURL:   url,
			Ref:         &branch,
			State:       gitlabState(wf.Status.Phase),
			Description: getState(wf.Status.Phase),
			Context:     &buildContext,
		}
		gitlabID := strings.Join([]string{owner, "/", repo}, "")
		_, _, err := c.GitlabClient.Commits.SetCommitStatus(gitlabID, commit, opts)
		if err != nil {
			c.Log.WithError(err).Error("failed to failed to push status to gitlab")
		}
	}
}
