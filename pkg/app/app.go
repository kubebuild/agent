package app

import (
	"os"
	"os/signal"
	"syscall"
	"time"

	"k8s.io/client-go/kubernetes"

	"github.com/kubebuild/agent/pkg/graphql"

	"github.com/argoproj/argo/pkg/client/clientset/versioned/typed/workflow/v1alpha1"
	"github.com/kubebuild/agent/pkg/schedulers"
	"github.com/kubebuild/agent/pkg/utils"
	"github.com/kubebuild/agent/pkg/workflow"
	gql "github.com/shurcooL/graphql"
	"github.com/sirupsen/logrus"
)

// App provides a default app structure with Logger
type App struct {
	Config         Configurer
	Log            *logrus.Logger
	GraphqlClient  *graphql.Client
	WorkflowClient v1alpha1.WorkflowInterface
	KubeClient     kubernetes.Interface
	Schedulers     []Scheduler
}

// NewApp configures and returns an App
func NewApp(config Configurer) (*App, error) {
	app := &App{
		Config: config,
	}

	logger, err := newLogger(config)
	if err != nil {
		return nil, err
	}
	logger.WithFields(
		logrus.Fields{
			"version":     config.GetVersion(),
			"name":        config.GetName(),
			"grapqhl-url": config.GetGraphqlURL(),
		}).Info("Starting app ...")
	app.Log = logger
	app.GraphqlClient = newGraphqlClient(config, app.Log)

	wfClient, kubeClient := workflow.InitWorkflowClient(app.GraphqlClient.Cluster, config.GetKubectlPath(), logger)

	app.KubeClient = kubeClient
	app.WorkflowClient = wfClient

	buildScheduler := schedulers.NewBuildScheduler(app.WorkflowClient, app.GraphqlClient, app.KubeClient, app.Log)
	app.Schedulers = append(app.Schedulers, buildScheduler)

	return app, nil
}

func newLogger(config Configurer) (*logrus.Logger, error) {
	log := logrus.New()
	log.Formatter = utils.NewLogFormatter(config.GetName(), config.GetVersion(), &logrus.TextFormatter{TimestampFormat: time.RFC3339Nano, FullTimestamp: true})
	log.Level = config.GetLogLevel()
	log.Out = os.Stdout
	return log, nil
}

func newGraphqlClient(config Configurer, log *logrus.Logger) *graphql.Client {
	client := gql.NewClient(config.GetGraphqlURL(), nil)
	graphqlClient := graphql.NewGraphqlClient(client, log)
	graphqlClient.ConnectCluster(config.GetToken())
	return graphqlClient
}

// WaitForInterrupt starts an infinite loop only broken by Ctrl-C
func (app *App) WaitForInterrupt() {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	for _ = range signals {
		return
	}
}

// StartSchedulers starts any registered receivers receiving messages.
func (app *App) StartSchedulers() {
	for _, scheduler := range app.Schedulers {
		go scheduler.Start()
	}
}
