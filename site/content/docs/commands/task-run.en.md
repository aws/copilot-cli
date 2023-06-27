# task run
```console
$ copilot task run
```

## What does it do?
`copilot task run` deploys and runs one-off tasks.

Generally, the steps involved in task run are:

1. Create an ECR repository and a log group for your task
2. Build and push the image to ECR
3. Create or update your ECS task definition
4. Run and wait for the tasks to start
5. If the tasks exit with a non-zero exit code, then forward the exit code.

!!!info
    1. Tasks with the same group name share the same set of resources, including the CloudFormation stack, ECR repository, CloudWatch log group and task definition.
    2. If the tasks are deployed to a Copilot environment (i.e. by specifying `--env`), only public subnets that are created by that environment will be used. 
    3. If you are using the `--default` flag and get an error saying there's no default cluster, run `aws ecs create-cluster` and then re-run the Copilot command. 

## What are the flags?
```
Name Flags
  -n, --task-group-name string   Optional. The group name of the task. 
                                 Tasks with the same group name share the same set of resources. 
                                 (default directory name)

Build Flags
      --build-context string   Path to the Docker build context.
                               Cannot be specified with --image.
      --dockerfile string      Path to the Dockerfile.
                               Cannot be specified with --image. (default "Dockerfile")
  -i, --image string           The location of an existing Docker image.
                               Cannot be specified with --dockerfile or --build-context.
      --tag string             Optional. The container image tag in addition to "latest".

Placement Flags
      --app string                Optional. Name of the application.
                                  Cannot be specified with --default, --subnets or --security-groups.
      --cluster string            Optional. The short name or full ARN of the cluster to run the task in. 
                                  Cannot be specified with --app, --env or --default.
      --default                   Optional. Run tasks in default cluster and default subnets. 
                                  Cannot be specified with --app, --env or --subnets.
      --env string                Optional. Name of the environment.
                                  Cannot be specified with --default, --subnets or --security-groups.
      --security-groups strings   Optional. Additional security group IDs for the task to use. Can be specified multiple times.
      --subnets strings           Optional. The subnet IDs for the task to use. Can be specified multiple times.
                                  Cannot be specified with --app, --env or --default.

Task Configuration Flags
      --acknowledge-secrets-access     Optional. Skip the confirmation question and grant access to the secrets specified by --secrets flag. 
                                       This flag is useful only when 'secrets' flag is specified
      --command string                 Optional. The command that is passed to "docker run" to override the default command.
      --count int                      Optional. The number of tasks to set up. (default 1)
      --cpu int                        Optional. The number of CPU units to reserve for each task. (default 256)
      --entrypoint string              Optional. The entrypoint that is passed to "docker run" to override the default entrypoint.
      --env-file string                Optional. A path to an environment variable (.env) file with each line being of the form of VARIABLE=VALUE. Values specified with --env-vars take precedence over --env-file.
      --env-vars stringToString        Optional. Environment variables specified by key=value separated by commas. (default [])
      --execution-role string          Optional. The ARN of the role that grants the container agent permission to make AWS API calls.
      --memory int                     Optional. The amount of memory to reserve in MiB for each task. (default 512)
      --platform-arch string           Optional. Architecture of the task. Must be specified along with 'platform-os'.
      --platform-os string             Optional. Operating system of the task. Must be specified along with 'platform-arch'.
      --resource-tags stringToString   Optional. Labels with a key and value separated by commas.
                                       Allows you to categorize resources. (default [])
      --secrets stringToString         Optional. Secrets to inject into the container. Specified by key=value separated by commas. (default []). 
                                       For secrets stored in AWS Parameter Store you can either specify names or ARNs. 
                                       For the secrets stored in AWS Secrets Manager you need to specify ARNs.
      --task-role string               Optional. The ARN of the role for the task to use.

Utility Flags
      --follow                        Optional. Specifies if the logs should be streamed.
      --generate-cmd string           Optional. Generate a command with a pre-filled value for each flag.
                                      To use it for an ECS service, specify --generate-cmd <cluster name>/<service name>.
                                      Alternatively, if the service or job is created with Copilot, specify --generate-cmd <application>/<environment>/<service or job name>.
                                      Cannot be specified with any other flags.
      --acknowledge-secrets-access    Optional. Skip the confirmation question and grant access to the secrets specified by --secrets flag.
                                      This flag is useful only when '--secret' flag is specified
```

## Examples
Run a task using your local Dockerfile and display log streams after the task is running. 
You will be prompted to specify an environment for the tasks to run in.
```console
$ copilot task run --follow
```

Run a task named "db-migrate" in the "test" environment under the current workspace.
```console
$ copilot task run -n db-migrate --env test --follow
```

Run 4 tasks with 2GB memory, an existing image, and a custom task role.
```console
$ copilot task run --count 4 --memory 2048 --image=rds-migrate --task-role migrate-role --follow
```

Run a task with environment variables.
```console
$ copilot task run --env-vars name=myName,user=myUser
```

Run a task using the current workspace with specific subnets and security groups.
```console
$ copilot task run --subnets subnet-123,subnet-456 --security-groups sg-123,sg-456
```

Run a task with a command.
```console
$ copilot task run --command "python migrate-script.py"
```

Run a Windows task with the minimum cpu and memory values.
```console
$ copilot task run --platform-os WINDOWS_SERVER_2019_CORE --platform-arch X86_64 --cpu 1024 --memory 2048
```

Run a Windows 2022 task with the minimum cpu and memory values.
```console
$ copilot task run --platform-os WINDOWS_SERVER_2022_CORE --platform-arch X86_64 --cpu 1024 --memory 2048
```

Run a task with a secret from AWS Secrets Manager injected into the container.
```console
$ copilot task run --secrets AuroraSecret=arn:aws:secretsmanager:us-east-1:535307839111:secret:AuroraSecret
```