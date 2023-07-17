# pipeline deploy
```console
$ copilot pipeline deploy [flags]
```

## What does it do?
`copilot pipeline deploy` deploys a pipeline for the services in your workspace, using the environments associated with the application from a pipeline manifest.

## What are the flags?
```
      --allow-downgrade   Optional. Allow using an older version of Copilot to update Copilot components
                          updated by a newer version of Copilot.
  -a, --app string        Name of the application.
      --diff              Compares the generated CloudFormation template to the deployed stack.
  -h, --help              help for deploy
  -n, --name string       Name of the pipeline.
      --yes               Skips confirmation prompt.
```

## Examples
Deploys a pipeline for the services and jobs in your workspace.
```console
$ copilot pipeline deploy
```