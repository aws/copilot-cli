# svc pause
```console
$ copilot svc pause [flags]
```

## What does it do?

!!! Note
  `svc pause` is only supported by services of type "Request-Driven Web Service".

`copilot svc pause` pauses the App Runner Service associated with your service within a specific environment.

## What are the flags?

```
  -a, --app string    Name of the application.
  -e, --env string    Name of the environment.
  -h, --help          help for pause
  -n, --name string   Name of the service.
      --yes           Skips confirmation prompt.
```

## Examples
Pause running App Runner service "my-svc".
```console
$ copilot svc pause -n my-svc
```