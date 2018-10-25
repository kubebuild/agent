package main

import (
	"os"

	agent "github.com/kubebuild/agent/pkg/app"
	"github.com/urfave/cli"
)

var appHelpTemplate = `{{.Name}} - {{.Usage}}

OPTIONS:
  {{range .Flags}}{{.}}
  {{end}}
`

func main() {
	cli.AppHelpTemplate = appHelpTemplate

	app := cli.NewApp()

	app.Name = "kubebuild-agent"
	app.Version = "1.0.0"
	app.Usage = "kubebuild agent and scheduler"
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "token",
			Value: "",
			Usage: "cluster token used to schedule builds",
		},
		cli.StringFlag{
			Name:  "graphql-url",
			Value: "https://api.kubebuild.com/graphql",
			Usage: "api url for graphql",
		},
		cli.StringFlag{
			Name:  "log-level",
			Value: "info",
			Usage: "log level",
		},
		cli.StringFlag{
			Name:  "kubectl-path",
			Value: "",
			Usage: "path of kubectl for non in cluster",
		},
	}
	app.Action = func(c *cli.Context) {
		token := c.String("token")
		if token == "" {
			token = os.Getenv("CLUSTER_TOKEN")
		}
		kubectlPath := c.String("kubectl-path")
		logLevel := c.String("log-level")
		version := c.App.Version
		name := c.App.Name
		graphqlURL := c.String("graphql-url")
		config := &agent.Config{
			Name:        name,
			Version:     version,
			GraphqlURL:  graphqlURL,
			Token:       token,
			LogLevel:    logLevel,
			KubectlPath: kubectlPath,
		}
		app, err := agent.NewApp(config)
		if err != nil {
			panic("Error occured exiting...")
		}
		app.StartSchedulers()
		app.WaitForInterrupt()
	}
	app.Run(os.Args)
}
