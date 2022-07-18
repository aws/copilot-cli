# pipeline show
```console
$ copilot pipeline show [flags]
```

## What does it do?
`copilot pipeline show` shows configuration information about a deployed pipeline for an application, including the account, region, and stages.

## What are the flags?
```
-a, --app string    Name of the application.
-h, --help          help for show
    --json          Optional. Output in JSON format.
-n, --name string   Name of the pipeline.
    --resources     Optional. Show the resources in your pipeline.
```

## Examples
Shows info, including resources, about the pipeline "myrepo-mybranch."
```console
$ copilot pipeline show --name myrepo-mybranch --resources
```

## What does it look like?

![Running copilot pipeline show](https://raw.githubusercontent.com/kohidave/copilot-demos/master/pipeline-show.svg?sanitize=true)