# svc exec
```console
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
      --yes                Optional. Whether to update the Session Manager Plugin.
```

## Examples

Start an interactive bash session with a task part of the "frontend" service.

```console
$ copilot svc exec -a my-app -e test -n frontend
```

Runs the 'ls' command in the task prefixed with ID "8c38184" within the "backend" service.

```console
$ copilot svc exec -a my-app -e test --name backend --task-id 8c38184 --command "ls"
```

## What does it look like?

<iframe width="560" height="315" src="https://www.youtube.com/embed/Evrl9Vux31k" frameborder="0" allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture" allowfullscreen></iframe>

!!! info
    1. Please make sure `exec: true` is set in your manifest before deploying the service.
    2. Please note that this will update the service's Fargate Platform Version to 1.4.0. Updating the Platform Version results in [replacing your service](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-ecs-service.html#cfn-ecs-service-platformversion) which will result in downtime for your service.
    3. `exec` is not supported for Windows containers.
