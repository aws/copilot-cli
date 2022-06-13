# svc deploy
```console
$ copilot svc deploy
```

## What does it do?

`copilot svc deploy` takes your local code and configuration and deploys it.

The steps involved in service deploy are:

1. Build your local Dockerfile into an image
2. Tag it with the value from `--tag` or the latest git sha (if you're in a git directory)
3. Push the image to ECR
4. Package your manifest file and addons into CloudFormation
4. Create / update your ECS task definition and service

## What are the flags?

```
  -a, --app string                     Name of the application.
  -e, --env string                     Name of the environment.
      --force                          Optional. Force a new service deployment using the existing image.
  -h, --help                           help for deploy
  -n, --name string                    Name of the service.
      --resource-tags stringToString   Optional. Labels with a key and value separated by commas.
                                       Allows you to categorize resources. (default [])
      --no-rollback bool               Optional. Disable automatic stack
                                       rollback in case of deployment failure.
                                       We do not recommend using this flag for a
                                       production environment.
      --tag string                     Optional. The service's image tag.
```

!!!info
    The `--no-rollback` flag is **not** recommended while deploying to a production environment as it may introduce service downtime. 
    If the deployment fails when automatic stack rollback is disabled, you may be required to manually start the stack 
    rollback of the stack via the AWS console or AWS CLI before the next deployment. 
