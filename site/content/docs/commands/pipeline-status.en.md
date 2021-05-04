# pipeline status
```bash
$ copilot pipeline status [flags]
```

## What does it do?
`copilot pipeline status` shows the status of the stages in a deployed pipeline.

## What are the flags?
```bash
-a, --app string    Name of the application.
-h, --help          help for status
    --json          Optional. Outputs in JSON format.
-n, --name string   Name of the pipeline.
```

## Examples
Shows status of the pipeline "pipeline-myapp-myrepo".
```bash
$ copilot pipeline status -n pipeline-myapp-myrepo
```

## What does it look like?

![Running copilot pipeline status](https://raw.githubusercontent.com/kohidave/copilot-demos/master/pipeline-status.svg?sanitize=true)