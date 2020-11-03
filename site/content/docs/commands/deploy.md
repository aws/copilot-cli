# copilot deploy
```
$ copilot deploy
```

## What does it do? 

This command is used to run either [`copilot svc deploy`](../commands/svc-deploy.md) or [`copilot job deploy`](../commands/job-deploy.md) under the hood. The steps involved in `copilot deploy` are the same as those involved in `copilot svc deploy` and `copilot job deploy`:

1. Build your local Dockerfile into an image
2. Tag it with the value from `--tag` or the latest git sha (if you're in a git directory)
3. Push the image to ECR
4. Package your manifest file and addons into CloudFormation
5. Create / update your ECS task definition and job or service.

## What are the flags?

```bash
  -a, --app string                     Name of the application.
  -e, --env string                     Name of the environment.
  -h, --help                           help for deploy
  -n, --name string                    Name of the service or job.
      --resource-tags stringToString   Optional. Labels with a key and value separated with commas.
                                       Allows you to categorize resources. (default [])
      --tag string                     Optional. The container image tag.
```

## Examples

Deploys a service named "frontend" to a "test" environment.
```bash
 $ copilot deploy --name frontend --env test
```

Deploys a job named "mailer" with additional resource tags to a "prod" environment.
```bash
$ copilot deploy -n mailer -e prod --resource-tags source/revision=bb133e7,deployment/initiator=manual
```