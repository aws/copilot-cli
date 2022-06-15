# task exec
```console
$ copilot task exec
```

## What does it do?
`copilot task exec` executes a command in a running container part of a task.

## What are the flags?
```
  -a, --app string       Name of the application.
  -c, --command string   Optional. The command that is passed to a running container. (default "/bin/bash")
      --default          Optional. Execute commands in running tasks in default cluster and default subnets.
                         Cannot be specified with 'app' or 'env'.
  -e, --env string       Name of the environment.
  -h, --help             help for exec
  -n, --name string      Name of the service, job, or task group.
      --task-id string   Optional. ID of the task you want to exec in.
```

## Examples

Start an interactive bash session with a task in task group "db-migrate" in the "test" environment under the current workspace.

```console
$ copilot task exec -e test -n db-migrate
```

Runs the 'cat progress.csv' command in the task prefixed with ID "1848c38" part of the "db-migrate" task group.

```console
$ copilot task exec --name db-migrate --task-id 1848c38 --command "cat progress.csv"
```

Start an interactive bash session with a task prefixed with ID "38c3818" in the default cluster.

```console
$ copilot task exec --default --task-id 38c3818
```

!!! info
    `copilot task exec` cannot be performed without certain task role permissions. If you are using existing task role to run the tasks, please make sure it has the following permissions in order to make `copilot task exec` work.
```json
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Action": [
                "ssmmessages:CreateControlChannel",
                "ssmmessages:OpenControlChannel",
                "ssmmessages:CreateDataChannel",
                "ssmmessages:OpenDataChannel"
            ],
            "Resource": "*",
            "Effect": "Allow"
        },
        {
            "Action": [
                "logs:CreateLogStream",
                "logs:DescribeLogGroups",
                "logs:DescribeLogStreams",
                "logs:PutLogEvents"
            ],
            "Resource": "*",
            "Effect": "Allow"
        }
    ]
}
```
