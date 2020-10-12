# pipeline show
```bash
$ copilot pipeline show [flags]
```

## What does it do?
`copilot pipeline show` shows configuration information about a deployed pipeline for an application, including the account, region, and stages.

## What are the flags?
```bash
-a, --app string    Name of the application.
-h, --help          help for show
    --json          Optional. Outputs in JSON format.
-n, --name string   Name of the pipeline.
    --resources     Optional. Show the resources in your pipeline.
```

## Examples
Shows info about the pipeline in the "myapp" application.
```bash
$ copilot pipeline show --app myapp --resources
```

## What does it look like?

![Running copilot pipeline show](https://raw.githubusercontent.com/kohidave/copilot-demos/master/pipeline-show.svg?sanitize=true)