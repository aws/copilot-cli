# task delete
```
$ copilot task delete
```

## What does it do?
`copilot task delete` removes the resources created by a one-off task.

Generally, the steps involved in `task delete` are:

1. Stop all running instances of the task.
2. Empty the ECR repository for the task.
3. Delete the CloudFormation stack, including the ECR repo, log group, and ECS Task Definition.

!!!info
    1. Tasks created with versions of Copilot earlier than v1.2.0 cannot be stopped by `copilot task delete`. Customers using tasks launched with earlier versions should manually stop any running tasks via the ECS console after running the command. 
    2. If you are using the `--default` flag, you cannot also specify the `--app` or `--env` flags. 

## What are the flags?
```
  -a, --app string    Name of the application.
      --default       Optional. Delete a task which was launched in the default cluster and subnets.
                      Cannot be specified with 'app' or 'env'
  -e, --env string    Name of the environment.
  -h, --help          help for delete
  -n, --name string   Name of the service.
      --yes           Optional. Skips confirmation prompt.
```
## Example
Delete the "test" task from the default cluster.
```
$ copilot task delete --name test --default
```

Delete the "db-migrate" task from the prod environment.
```
$ copilot task delete --name db-migrate --env prod
```

Delete the "test" task without confirmation prompt.
```
$ copilot task delete --name test --yes
```
