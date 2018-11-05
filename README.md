# KubeBuild Agent 

The kubebuild-agent is a small, reliable, and kubernetes build runner that makes it easy to run automated builds on your own infrastructure. It’s main responsibilities are polling [kubebuild.com](https://www.kubebuild.com/) for work, running build jobs, reporting back the status code and output log of the job, and uploading the job's artifacts.

Full documentation is available at [docs.kubebuild.com/agent](https://docs.kubebuild.com/agent)

```
$ kubebuild-agent -h
kubebuild-agent - kubebuild agent and scheduler

OPTIONS:
  --token value         cluster token used to schedule builds
  --graphql-url value   api url for graphql (default: "https://api.kubebuild.com/graphql")
  --log-level value     log level (default: "info")
  --kubectl-path value  path of kubectl for non in cluster
  --help, -h            show help
  --version, -v         print the version
```

## Installing

The KubeBuild agent is available via docker [via Docker](https://hub.docker.com/r/kubebuild/agent).

## Starting

To start an agent all you need is your agent token, which you can find on your Cluster page within Kubebuild.

```bash
kubebuild-agent --token "your-kubebuild-token"
```

## Development

These instructions assume you are running a recent macOS, but could easily be adapted to Linux and Windows.

```bash
# Make sure you have go 1.10+ installed.
brew install go

# Setup your GOPATH
export GOPATH="$HOME/go"
export PATH="$HOME/go/bin:$PATH"

# Checkout the code
go get github.com/kubebuild/agent
cd "$HOME/go/src/github.com/kubebuild/agent"

# Start the agent
go run cmd/kubebuild-agent/kubebuild-agent.go --token get-cluster-token --log-level debug --kubectl-path ~/.kube/config
```

### Dependency management

We're using [dep](https://github.com/golang/dep) to manage our Go dependencies. Install it with:

```bash
dep ensure -vendor-only
```

If you introduce a new package, just add the import to your source file and run:

```bash
dep ensure
```

Or explicitly fetch it with a version using:

```bash
dep ensure -add github.com/kubebuild/package
```

## Contributing

1. Fork it
1. Create your feature branch (`git checkout -b my-new-feature`)
1. Commit your changes (`git commit -am 'Add some feature'`)
1. Push to the branch (`git push origin my-new-feature`)
1. Create new Pull Request

## Running it locally

```go run cmd/kubebuild-agent/kubebuild-agent.go --token dev-token --graphql-url http://localhost:4000/graphql --log-level debug --kubectl-path ~/.kube/config```

## Contributors

## Copyright

Copyright (c) 2017-2019 KubeBuild @ (Meerkat Technologies Pty Ltd). See [LICENSE](./LICENSE) for details.
