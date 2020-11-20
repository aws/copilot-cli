# svc deploy
```bash
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

```bash
  -e, --env string                     Name of the environment.
  -h, --help                           help for deploy
  -n, --name string                    Name of the service.
      --resource-tags stringToString   Optional. Labels with a key and value separated with commas.
                                       Allows you to categorize resources. (default [])
      --tag string                     Optional. The service's image tag.
```