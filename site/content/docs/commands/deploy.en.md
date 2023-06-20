# copilot deploy
```console
$ copilot deploy
```

## What does it do?

This command is used to run either [`copilot svc deploy`](../commands/svc-deploy.en.md) or [`copilot job deploy`](../commands/job-deploy.en.md) under the hood. The steps involved in `copilot deploy` are the same as those involved in `copilot svc deploy` and `copilot job deploy`:

1. When `image.build` exists in the manifest:
    1. Build your local Dockerfile into an image
    2. Tag it with the value from `--tag` or the latest git sha (if you're in a git directory)
    3. Push the image to ECR
2. Package your manifest file and addons into CloudFormation
3. Create / update your ECS task definition and job or service.

## What are the flags?

```
  -a, --app string                     Name of the application.
  -e, --env string                     Name of the environment.
      --force                          Optional. Force a new service deployment using the existing image.
  -h, --help                           help for deploy
  -n, --name string                    Name of the service or job.
      --no-rollback bool               Optional. Disable automatic stack
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

Deploys a service named "frontend" to a "test" environment.
```console
 $ copilot deploy --name frontend --env test
```

Deploys a job named "mailer" with additional resource tags to a "prod" environment.
```console
$ copilot deploy -n mailer -e prod --resource-tags source/revision=bb133e7,deployment/initiator=manual
```
