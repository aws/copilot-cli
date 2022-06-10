# task delete
```console
$ copilot task delete
```

## What does it do?
`copilot task delete` stops running instances of the task, and deletes associated resources.

!!!info
    Tasks created with versions of Copilot earlier than v1.2.0 cannot be stopped by `copilot task delete`. Customers using tasks launched with earlier versions should manually stop any running tasks via the ECS console after running the command. 

## What are the flags?
```
  -a, --app string    Name of the application.
      --default       Optional. Delete a task which was launched in the default cluster and subnets.
                      Cannot be specified with 'app' or 'env'.
  -e, --env string    Name of the environment.
  -h, --help          help for delete
  -n, --name string   Name of the service.
      --yes           Optional. Skips confirmation prompt.
```
## Example
Delete the "test" task from the default cluster.
```console
$ copilot task delete --name test --default
```

Delete the "db-migrate" task from the prod environment.
```console
$ copilot task delete --name db-migrate --env prod
```

Delete the "test" task without confirmation prompt.
```console
$ copilot task delete --name test --yes
```
