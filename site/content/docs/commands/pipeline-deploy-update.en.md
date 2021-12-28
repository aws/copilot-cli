# pipeline deploy/update
```bash
$ copilot pipeline deploy [flags]
```
or
```bash
$ copilot pipeline update [flags]
```

## What do they do?
`copilot pipeline deploy` and `copilot pipeline update` deploy/update a pipeline for the services in your workspace, using the environments associated with the application from a pipeline manifest.

## What are the flags?
```bash
-h, --help   help for deploy/update
    --yes    Skips confirmation prompt.
```

## Examples
Deploys a pipeline for the services and jobs in your workspace.
```bash
$ copilot pipeline deploy
```
or
```bash
$ copilot pipeline update
```