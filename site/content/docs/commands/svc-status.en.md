# svc status
```console
$ copilot svc status
```

## What does it do?
`copilot svc status` shows the health status of a deployed service, including service status, task status, and related CloudWatch alarms.

## What are the flags?
```
  -a, --app string    Name of the application.
  -e, --env string    Name of the environment.
  -h, --help          help for status
      --json          Optional. Output in JSON format.
  -n, --name string   Name of the service.
```

## What does it look like?

![Running copilot svc status](https://raw.githubusercontent.com/kohidave/copilot-demos/master/svc-status.svg?sanitize=true)