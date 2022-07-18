# job ls
```console
$ copilot job ls
```

## What does it do?

`copilot job ls` lists all the Copilot jobs for a particular application.

## What are the flags?

```
  -a, --app string   Name of the application.
  -h, --help         help for ls
      --json         Optional. Output in JSON format.
      --local        Only show jobs in the workspace.
```

## Example

Lists all the jobs for the "myapp" application.
```console
$ copilot job ls --app myapp
```