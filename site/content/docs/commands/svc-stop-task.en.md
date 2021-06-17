# svc stop-task
```bash
$ copilot svc stop-task [flags]
```

## What does it do?

`copilot svc stop-task` stops all the tasks associated with your service in a particular environment.

## What are the flags?

```bash
  -e, --env string    Name of the environment.
  -h, --help          help for delete
  -n, --name string   Name of the service.
      --all           Stops all the running tasks
      --tasks         Stops tasks based on passed in taskARN
```

## Examples
Stop all tasks associated with service "my-svc" in "test" environment.
```bash
$ copilot svc stop-task -n my-svc -e test --all
```

Stop particular tasks based on `taskArn` associated with service "my-svc" in "test" environment.
```bash
$ copilot svc stop-task -n my-svc -e test --tasks <<taskARN>>
```