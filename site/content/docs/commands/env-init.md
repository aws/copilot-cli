# env init
```bash
$ copilot env init [flags]
```

## What does it do?
`copilot env init` creates a new [environment](../concepts/environments.md) where your services will live.

After you answer the questions, the CLI creates the common infrastructure that's shared between your services such as a VPC, an Application Load Balancer, and an ECS Cluster. Additionally, you can [customize Copilot environment](../concepts/environments.md#customize-your-environment) by either configuring the default environment resources or importing existing resources for your environment.

You create environments using a [named profile](../credentials.md#environment-credentials) to specify which AWS Account and Region you'd like the environment to be in.

## What are the flags?
Like all commands in the AWS Copilot CLI, if you don't provide required flags, we'll prompt you for all the information we need to get you going. You can skip the prompts by providing information via flags:
```
Common Flags
      --aws-access-key-id string       Optional. An AWS access key.
      --aws-secret-access-key string   Optional. An AWS secret access key.
      --aws-session-token string       Optional. An AWS session token for temporary credentials.
      --default-config                 Optional. Skip prompting and use default environment configuration.
  -n, --name string                    Name of the environment.
      --prod                           If the environment contains production services.
      --profile string                 Name of the profile.
      --region string                  Optional. An AWS region where the environment will be created.

Import Existing Resources Flags
      --import-private-subnets strings   Optional. Use existing private subnet IDs.
      --import-public-subnets strings    Optional. Use existing public subnet IDs.
      --import-vpc-id string             Optional. Use an existing VPC ID.

Configure Default Resources Flags
      --override-private-cidrs strings   Optional. CIDR to use for private subnets (default 10.0.2.0/24,10.0.3.0/24).
      --override-public-cidrs strings    Optional. CIDR to use for public subnets (default 10.0.0.0/24,10.0.1.0/24).
      --override-vpc-cidr ipNet          Optional. Global CIDR to use for VPC (default 10.0.0.0/16).

Global Flags
  -a, --app string   Name of the application.
```

## Examples
Creates a test environment in your "default" AWS profile using default config.
```bash
$ copilot env init --name test --profile default --default-config
```

Creates a prod-iad environment using your "prod-admin" AWS profile using existing VPC.
```bash
$ copilot env init --name prod-iad --profile prod-admin --prod \
--import-vpc-id vpc-099c32d2b98cdcf47 \
--import-public-subnets subnet-013e8b691862966cf,subnet-014661ebb7ab8681a \
--import-private-subnets subnet-055fafef48fb3c547,subnet-00c9e76f288363e7f
```

## What does it look like?
![Running copilot env init](https://raw.githubusercontent.com/kohidave/copilot-demos/master/env-init.svg?sanitize=true)
