# pipeline status
```console
$ copilot pipeline status [flags]
```

## What does it do?
`copilot pipeline status` shows the status of the stages in a deployed pipeline.

## What are the flags?
```
-a, --app string    Name of the application.
-h, --help          help for status
    --json          Optional. Output in JSON format.
-n, --name string   Name of the pipeline.
```

## Examples
Shows status of the pipeline "my-repo-my-branch".
```console
$ copilot pipeline status -n my-repo-my-branch
```

## What does it look like?

![Running copilot pipeline status](https://raw.githubusercontent.com/kohidave/copilot-demos/master/pipeline-status.svg?sanitize=true)