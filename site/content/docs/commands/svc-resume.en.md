# svc resume
```console
$ copilot svc resume [flags]
```

## What does it do?

!!! Note
  `svc resume` is only supported by services of type "Request-Driven Web Service".

`copilot svc resume` resumes the App Runner Service associated with your service within a specific environment.

## What are the flags?

```
  -a, --app string    Name of the application.
  -e, --env string    Name of the environment.
  -h, --help          help for resume
  -n, --name string   Name of the service.
```

## Examples
Resume paused App Runner service "my-svc".
```console
$ copilot svc resume -n my-svc
```