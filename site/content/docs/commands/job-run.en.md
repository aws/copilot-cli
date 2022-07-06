# job run
```bash
$ copilot job run
```

## What does it do?

`copilot job run` runs a scheduled job

## What are the flags?

```bash
  -a, --app string          Name of the application.
  -e, --env string          Name of the environment.
  -h, --help                help for package
  -n, --name string         Name of the job.
```

## Examples

Runs a job named "report-gen" in an application named "report" to a "test" environment

```bash
$ copilot job run -a report -n report-gen -e test
```

