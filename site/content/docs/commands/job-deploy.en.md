# job deploy
```console
$ copilot job deploy
```

## What does it do?

`job deploy` takes your local code and configuration and deploys your job. 

The steps involved in `job deploy` are:

1. [Optional] Build your local Dockerfile into an image
2. [Optional] Tag it with the value from `--tag` or the latest git sha (if you're in a git directory)
3. [Optional] Push the image to ECR
4. Package your manifest file and addons into CloudFormation
4. Create / update your ECS task definition and job

## What are the flags?

```
  -a, --app string                     Name of the application.
      --diff                           Compares the generated CloudFormation template to the deployed stack.
  -e, --env string                     Name of the environment.
  -h, --help                           help for deploy
  -n, --name string                    Name of the job.
      --no-rollback                    Optional. Disable automatic stack
                                       rollback in case of deployment failure.
                                       We do not recommend using this flag for a
                                       production environment.
      --resource-tags stringToString   Optional. Labels with a key and value separated by commas.
                                       Allows you to categorize resources. (default [])
      --tag string                     Optional. The container image tag for image Copilot builds from the Dockerfile.
```

!!!info
The `--no-rollback` flag is **not** recommended while deploying to a production environment as it may introduce service downtime.
If the deployment fails when automatic stack rollback is disabled, you may be required to manually start the rollback of the stack via the AWS console or AWS CLI before the next deployment.

## Examples

Deploys a job named "report-gen" to a "test" environment.
```console
$ copilot job deploy --name report-gen --env test
```

Deploys a job with additional resource tags.
```console
$ copilot job deploy --resource-tags source/revision=bb133e7,deployment/initiator=manual`
```

Use `--diff` to see what will be changed before making a deployment.
```console
$ copilot job deploy --diff
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

!!!info "`copilot job package --diff`"
    Alternatively, if you just wish to take a peek at the diff without potentially making a deployment,
    you can run `copilot job package --diff`, which will print the diff and exit.