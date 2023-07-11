# svc show
```console
$ copilot svc show
```

## What does it do?

`copilot svc show` shows info about a deployed service. Depending on the service type, output may include endpoints, configuration, variables, and/or associated S3 objects per environment.

## What are the flags?

```
-a, --app string        Name of the application.
-h, --help              help for show
    --json              Optional. Output in JSON format.
    --manifest string   Optional. Name of the environment in which the service was deployed;
                        output the manifest file used for that deployment.
-n, --name string       Name of the service.
    --resources         Optional. Show the resources in your service.
```

## Examples
Print service configuration in deployed environments.
```console
$ copilot svc show -n api
```

Print manifest file used for deploying service "api" in the "prod" environment.
```console
$ copilot svc show -n api --manifest prod
```

## What does it look like?

![Running copilot svc show](https://raw.githubusercontent.com/kohidave/copilot-demos/master/svc-show.svg?sanitize=true)