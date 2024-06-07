# svc delete
```console
$ copilot svc delete [flags]
```

## What does it do?

`copilot svc delete` deletes all resources associated with your service in a particular environment.

## What are the flags?

```
  -e, --env string    Name of the environment.
  -h, --help          help for delete
  -n, --name string   Name of the service.
      --yes           Skips confirmation prompt.
```

## Examples
Force delete a service from environment "test".
```console
$ copilot svc delete --env test --yes
```
