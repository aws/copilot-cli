# svc deploy
```console
$ copilot svc deploy
```

## What does it do?

`copilot svc deploy` takes your local code and configuration and deploys it.

The steps involved in service deploy are:

1. When `image.build` exists in the manifest:
    1. Build your local Dockerfile into an image
    2. Tag it with the value from `--tag` or the latest git sha (if you're in a git directory)
    3. Push the image to ECR
2. Package your manifest file and addons into CloudFormation
3. Create / update your ECS task definition and service

## What are the flags?

```
      --allow-downgrade                Optional. Allow using an older version of Copilot to update Copilot components
                                       updated by a newer version of Copilot.
  -a, --app string                     Name of the application.
      --detach                         Optional. Skip displaying CloudFormation deployment progress.
      --diff                           Compares the generated CloudFormation template to the deployed stack.
      --diff-yes                       Skip interactive approval of diff before deploying.
  -e, --env string                     Name of the environment.
      --force                          Optional. Force a new service deployment using the existing image.
  -h, --help                           help for deploy
  -n, --name string                    Name of the service.
      --no-rollback                    Optional. Disable automatic stack
                                       rollback in case of deployment failure.
                                       We do not recommend using this flag for a
                                       production environment.
      --resource-tags stringToString   Optional. Labels with a key and value separated by commas.
                                       Allows you to categorize resources. (default [])
      --tag string                     Optional. The tag for the container images Copilot builds from Dockerfiles.
```

!!!info
    The `--no-rollback` flag is **not** recommended while deploying to a production environment as it may introduce service downtime. 
    If the deployment fails when automatic stack rollback is disabled, you may be required to manually start the stack 
    rollback of the stack via the AWS console or AWS CLI before the next deployment. 

## Examples
Use `--diff` to see what will be changed before making a deployment.

```console
$ copilot svc deploy --diff
~ Resources:
    ~ TaskDefinition:
        ~ Properties:
            ~ ContainerDefinitions:
                ~ - (changed item)
                  ~ Environment:
                      (4 unchanged items)
                      + - Name: LOG_LEVEL
                      +   Value: "info"

Continue with the deployment? (y/N)
```

!!!info "`copilot svc package --diff`"
    Alternatively, if you just wish to take a peek at the diff without potentially making a deployment,
    you can run `copilot svc package --diff`, which will print the diff and exit.