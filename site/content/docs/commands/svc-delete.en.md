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
Force delete the application with environments "test" and "prod".
```console
$ copilot svc delete --name test --yes
```