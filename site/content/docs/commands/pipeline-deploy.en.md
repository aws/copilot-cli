# pipeline deploy
```bash
$ copilot pipeline deploy [flags]
```


## What does it do?
`copilot pipeline deploy` deploys a pipeline for the services in your workspace, using the environments associated with the application from a pipeline manifest.

## What are the flags?
```bash
-h, --help   help for deploy
    --yes    Skips confirmation prompt.
```

## Examples
Deploys a pipeline for the services and jobs in your workspace.
```bash
$ copilot pipeline deploy
```