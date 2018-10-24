package app

import (
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/argoproj/argo/pkg/client/clientset/versioned/typed/workflow/v1alpha1"
	"github.com/kubebuild/agent/pkg/mutations"
	"github.com/kubebuild/agent/pkg/schedulers"
	"github.com/kubebuild/agent/pkg/utils"
	"github.com/kubebuild/agent/pkg/workflow"
	"github.com/shurcooL/graphql"
	"github.com/sirupsen/logrus"
)

// App provides a default app structure with Logger
type App struct {
	Config         Configurer
	Log            *logrus.Logger
	GraphqlClient  *graphql.Client
	WorkflowClient v1alpha1.WorkflowInterface
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
	app.GraphqlClient = newGraphqlClient(config)

	cluster := mutations.ConnectCluster(config.GetToken(), app.GraphqlClient, logger)

	client := workflow.InitWorkflowClient(cluster, config.GetInCluster(), config.GetKubectlPath(), logger)

	app.WorkflowClient = client

	buildScheduler := schedulers.NewBuildScheduler(cluster, app.WorkflowClient, app.GraphqlClient, logger)
	app.Schedulers = append(app.Schedulers, buildScheduler)

	return app, nil
}

func newLogger(config Configurer) (*logrus.Logger, error) {
	log := logrus.New()
	log.Formatter = utils.NewLogFormatter(config.GetName(), config.GetVersion(), &logrus.JSONFormatter{TimestampFormat: time.RFC3339Nano})
	log.Level = config.GetLogLevel()
	log.Out = os.Stdout
	return log, nil
}

func newGraphqlClient(config Configurer) *graphql.Client {
	client := graphql.NewClient(config.GetGraphqlURL(), nil)
	return client
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
