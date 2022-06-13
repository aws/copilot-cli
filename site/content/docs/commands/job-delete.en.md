# job delete
```console
$ copilot job delete [flags]
```

## What does it do?

`copilot job delete` deletes all resources associated with your job in a particular environment.

## What are the flags?

```
  -a, --app string    Name of the application.
  -e, --env string    Name of the environment.
  -h, --help          help for delete
  -n, --name string   Name of the job.
      --yes           Skips confirmation prompt.
```

## Examples

Delete the "report-generator" job from the my-app application.
```console
$ copilot job delete --name report-generator --app my-app
```

Delete the "report-generator" job from just the prod environment.
```console
$ copilot job delete --name report-generator --env prod
```

Delete the "report-generator" job from the my-app application from outside of the workspace.
```console
$ copilot job delete --name report-generator --app my-app
```

Delete the "report-generator" job without the confirmation prompt.
```console
$ copilot job delete --name report-generator --yes
```