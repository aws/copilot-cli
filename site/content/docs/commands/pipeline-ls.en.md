# pipeline ls
```console
$ copilot pipeline ls [flags]
```

## What does it do?
`copilot pipeline ls` lists all the deployed pipelines in an application.

## What are the flags?
```
-a, --app string   Name of the application.
-h, --help         help for ls
    --json         Optional. Output in JSON format.
    --local        Only show pipelines in the workspace.
```

## Examples
Lists all the pipelines for the "phonetool" application.
```console
$ copilot pipeline ls -a phonetool
```
