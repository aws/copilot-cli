# svc exec
```
$ copilot svc exec
```

## What does it do?
`copilot svc exec` executes a command in a running container part of a service.

## What are the flags?
```
  -a, --app string         Name of the application.
  -c, --command string     Optional. The command that is passed to a running container. (default "/bin/bash")
      --container string   Optional. The specific container you want to exec in. By default the first essential container will be used.
  -e, --env string         Name of the environment.
  -h, --help               help for exec
  -n, --name string        Name of the service, job, or task group.
      --task-id string     Optional. ID of the task you want to exec in.
```

## Examples

Start an interactive bash session with a task part of the "frontend" service.

```bash
$ copilot svc exec -a my-app -e test -n frontend
```

Runs the 'ls' command in the task prefixed with ID "8c38184" within the "backend" service.

```bash
$ copilot svc exec -a my-app -e test --name backend --task-id 8c38184 --command "ls"
```

!!! info
    1. `copilot svc exec` is not enabled for your services by default. To enable this feature, please add `execute_command: true` to your manifest before deploying the service.
    2. Please note that this will update the service's Fargate Platform Version to 1.4.0.
