# job deploy
```bash
$ copilot job deploy
```

## What does it do?

`job deploy` takes your local code and configuration and deploys your job. 

The steps involved in `job deploy` are:

1. Build your local Dockerfile into an image
2. Tag it with the value from `--tag` or the latest git sha (if you're in a git directory)
3. Push the image to ECR
4. Package your manifest file and addons into CloudFormation
4. Create / update your ECS task definition and job

## What are the flags?

```bash
  -a, --app string                     Name of the application.
  -e, --env string                     Name of the environment.
  -h, --help                           help for deploy
  -n, --name string                    Name of the job.
      --resource-tags stringToString   Optional. Labels with a key and value separated with commas.
                                       Allows you to categorize resources. (default [])
      --tag string                     Optional. The container image tag.
```

## Examples

Deploys a job named "report-gen" to a "test" environment.
```bash
$ copilot job deploy --name report-gen --env test
```

Deploys a job with additional resource tags.
```bash
$ copilot job deploy --resource-tags source/revision=bb133e7,deployment/initiator=manual`
```
