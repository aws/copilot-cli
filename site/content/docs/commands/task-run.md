# task run
```
$ copilot task run
```

## What does it do?
`copilot task run` deploys and runs one-off tasks.

Generally, the steps involved in task run are:

1. Create an ECR repository and a log group for your task
2. Build and push the image to ECR
3. Create or update your ECS task definition
4. Run and wait for the tasks to start

!!!info
    1. Tasks with the same group name share the same set of resources, including the CloudFormation stack, ECR repository, CloudWatch log group and task definition.
    2. If the tasks are deployed to a Copilot environment (i.e. by specifying `--env`), only public subnets that are created by that environment will be used. 
    3. The `--env` flag only works with environments created with v0.3.0 of Copilot or later. Customers using environments created with v0.2.0 or earlier can update their environment manager role with [this](https://github.com/aws/copilot-cli/blob/mainline/templates/environment/cf/environment-manager-role.yml) policy. 
    4. If you are using the `--default` flag and get an error saying there's no default cluster, run `aws ecs create-cluster` and then re-run the Copilot command. 

## What are the flags?
```
  --app string                     Optional. Name of the application.
                                   Cannot be specified with 'default', 'subnets' or 'security-groups'
  --command string                 Optional. The command that is passed to "docker run" to override the default command.
  --count int                      Optional. The number of tasks to set up. (default 1)
  --cpu int                        Optional. The number of CPU units to reserve for each task. (default 256)
  --default                        Optional. Run tasks in default cluster and default subnets.
                                   Cannot be specified with 'app', 'env' or 'subnets'.
  --dockerfile string              Path to the Dockerfile. (default "Dockerfile")
  --env string                     Optional. Name of the environment.
                                   Cannot be specified with 'default', 'subnets' or 'security-groups'
  --env-vars stringToString        Optional. Environment variables specified by key=value separated with commas. (default [])
  --execution-role string          Optional. The role that grants the container agent permission to make AWS API calls.
  --follow                         Optional. Specifies if the logs should be streamed.
-h, --help                         help for run
  --image string                   Optional. The image to run instead of building a Dockerfile.
  --memory int                     Optional. The amount of memory to reserve in MiB for each task. (default 512)
  --resource-tags stringToString   Optional. Labels with a key and value separated with commas.
                                   Allows you to categorize resources. (default [])
  --security-groups strings        Optional. The security group IDs for the task to use. Can be specified multiple times.
                                   Cannot be specified with 'app' or 'env'.
  --subnets strings                Optional. The subnet IDs for the task to use. Can be specified multiple times.
                                   Cannot be specified with 'app', 'env' or 'default'.
  --tag string                     Optional. The container image tag in addition to "latest".
-n, --task-group-name string       Optional. The group name of the task. Tasks with the same group name share the same set of resources.
  --task-role string               Optional. The role for the task to use.
```
## Example
Run a task using your local Dockerfile. 
You will be prompted to specify a task group name and an environment for the tasks to run in.
```
$ copilot task run --follow
```

Run a task named "db-migrate" in the "test" environment under the current workspace.
```
$ copilot task run -n db-migrate --env test --follow
```

Run 4 tasks with 2GB memory, an existing image, and a custom task role.
```
$ copilot task run --num 4 --memory 2048 --image=rds-migrate --task-role migrate-role --follow
```

Run a task with environment variables.
```
$ copilot task run --env-vars name=myName,user=myUser
```

Run a task using the current workspace with specific subnets and security groups.
```
$ copilot task run --subnets subnet-123,subnet-456 --security-groups sg-123,sg-456
```

Run a task with a command.
```
$ copilot task run --command "python migrate-script.py"
```
