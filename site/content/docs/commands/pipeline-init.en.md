# pipeline init
```console
$ copilot pipeline init [flags]
```

## What does it do?
`copilot pipeline init` creates a pipeline manifest for the services in your workspace, using the environments associated with the application.

## What are the flags?
```
  -a, --app string             Name of the application.
  -e, --environments strings   Environments to add to the pipeline.
  -b, --git-branch string      Branch used to trigger your pipeline.
  -h, --help                   help for init
  -n, --name string            Name of the pipeline.
  -p, --pipeline-type string   The type of pipeline. Must be either "Workloads" or "Environments".
  -u, --url string             The repository URL to trigger your pipeline.
```

## Examples
Create a pipeline for the services in your workspace.
```console
$ copilot pipeline init \
--name frontend-main \
--url https://github.com/gitHubUserName/frontend.git \
--git-branch main \
--environments "test,prod" 
```