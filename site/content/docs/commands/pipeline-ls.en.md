# pipeline ls
```bash
$ copilot pipeline ls [flags]
```

## What does it do?
`copilot pipeline ls` lists all the deployed pipelines in an application.

## What are the flags?
```bash
-a, --app string   Name of the application.
-h, --help         help for ls
    --json         Optional. Outputs in JSON format.
```

## Examples
Lists all the pipelines for the "phonetool" application.
```bash
$ copilot pipeline ls -a phonetool
```
