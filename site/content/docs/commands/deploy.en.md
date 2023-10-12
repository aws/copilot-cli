# copilot deploy
```console
$ copilot deploy
```

## What does it do?

This command is used to get you from local manifests to deployed service and environment. It will check for deployed infrastructure and local manifests, help you initialize and deploy an environment, and deploy a workload.

If a workload is uninitialized, `--init-wkld` will initialize the workload before deploying it.

If the desired environment is uninitialized, you may initialize it with `--init-env`. 

The `--deploy-env` flag can be specified to skip environment deployment confirmation, or can be set to false (`--deploy-env=false`) to skip 
deploying the environment.

The steps involved in `copilot deploy` are as follows:

1. If your service does not exist, optionally initialize it.
2. If the target environment does not exist, optionally initialize it with custom credentials.
3. Optionally deploy the environment before service deployment.
4. When `image.build` exists in the manifest:
    1. Build your local Dockerfile into an image
    2. Tag it with the value from `--tag` or the latest git sha (if you're in a git directory)
    3. Push the image to ECR
5. Package your manifest file and addons into CloudFormation.
6. Create / update your ECS task definition and job or service.

## What are the flags?

```
      --all                            Optional. Deploy all workloads with manifests in the current Copilot workspace.
      --allow-downgrade                Optional. Allow using an older version of Copilot to update Copilot components
                                       updated by a newer version of Copilot.
  -a, --app string                     Name of the application.
      --aws-access-key-id string       Optional. An AWS access key for the environment account.
      --aws-secret-access-key string   Optional. An AWS secret access key for the environment account.
      --aws-session-token string       Optional. An AWS session token for temporary credentials.
      --deploy-env                     Deploy the target environment before deploying the workload.
      --detach                         Optional. Skip displaying CloudFormation deployment progress.
  -e, --env string                     Name of the environment.
      --force                          Optional. Force a new service deployment using the existing image.
                                       Not available with the "Static Site" service type.
  -h, --help                           help for deploy
      --init-env                       Confirm initializing the target environment if it does not exist.
      --init-wkld                      Optional. When specified with --all, initialize all local workloads before deployment.
  -n, --name strings                   Names of the service or jobs to deploy, with an optional priority tag (e.g. fe/1, be/2, my-job/1).
      --no-rollback                    Optional. Disable automatic stack 
                                       rollback in case of deployment failure.
                                       We do not recommend using this flag for a
                                       production environment.
      --profile string                 Name of the profile for the environment account.
      --region string                  Optional. An AWS region where the environment will be created.
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

Initializes and deploys an environment named "test" in us-west-2 under the "default" profile with local manifest,
then deploys a service named "api"
```console
$ copilot deploy --init-env --deploy-env --env test --name api --profile default --region us-west-2
```

Initializes and deploys a service named "backend" to a "prod" environment.
```console
$ copilot deploy --init-wkld --deploy-env=false --env prod --name backend
```

Deploys all local, initialized workloads in no particular order.
```console
$ copilot deploy --all --env prod --name backend
```

Deploys multiple workloads in a prescribed order (fe and worker, then be).
```console
$ copilot deploy -n fe/1 -n be/2 -n worker/1
```

Initializes and deploys all local workloads after (re)deploying the `prod` environment.
```console
$ copilot deploy --all --init-wkld --deploy-env -e prod
```

