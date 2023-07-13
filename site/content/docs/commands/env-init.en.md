# env init
```console
$ copilot env init [flags]
```

## What does it do?
`copilot env init` creates a new [environment](../concepts/environments.en.md) where your services will live.

After you answer the questions, the CLI creates the common infrastructure that's shared between your services such as a VPC, an Application Load Balancer, and an ECS Cluster. Additionally, you can [customize your Copilot environment](../developing/custom-environment-resources.en.md) by either configuring the default environment resources or importing existing resources for your environment.

You create environments using a [named profile](../credentials.en.md#environment-credentials) to specify which AWS account and region you'd like the environment to be in.

## What are the flags?
Like all commands in the AWS Copilot CLI, if you don't provide required flags, we'll prompt you for all the information we need to get you going. You can skip the prompts by providing information via flags:
```
Common Flags
      --allow-downgrade                Optional. Allow using an older version of Copilot to update Copilot components
                                       updated by a newer version of Copilot.
  -a, --app string                     Name of the application.
      --aws-access-key-id string       Optional. An AWS access key.
      --aws-secret-access-key string   Optional. An AWS secret access key.
      --aws-session-token string       Optional. An AWS session token for temporary credentials.
      --default-config                 Optional. Skip prompting and use default environment configuration.
  -n, --name string                    Name of the environment.
      --profile string                 Name of the profile.
      --region string                  Optional. An AWS region where the environment will be created.

Import Existing Resources Flags
      --import-cert-arns strings         Optional. Apply existing ACM certificates to the internet-facing load balancer.
      --import-private-subnets strings   Optional. Use existing private subnet IDs.
      --import-public-subnets strings    Optional. Use existing public subnet IDs.
      --import-vpc-id string             Optional. Use an existing VPC ID.

Configure Default Resources Flags
      --internal-alb-allow-vpc-ingress   Optional. Allow internal ALB ingress from ports 80 and 443.
      --internal-alb-subnets strings     Optional. Specify subnet IDs for an internal load balancer.
                                         By default, the load balancer will be placed in your private subnets.
                                         Cannot be specified with --default-config or any of the --override flags.
      --override-az-names strings        Optional. Availability Zone names.
                                         (default 2 random AZs)
      --override-private-cidrs strings   Optional. CIDR to use for private subnets.
                                         (default 10.0.2.0/24,10.0.3.0/24)
      --override-public-cidrs strings    Optional. CIDR to use for public subnets.
                                         (default 10.0.0.0/24,10.0.1.0/24)
      --override-vpc-cidr ipNet          Optional. Global CIDR to use for VPC.
                                         (default 10.0.0.0/16)

Telemetry Flags
      --container-insights   Optional. Enable CloudWatch Container Insights.
```

## Examples
Creates a test environment using your "default" AWS profile and default configuration.
```console
$ copilot env init --name test --profile default --default-config
```

Creates a prod-iad environment using your "prod-admin" AWS profile and enables CloudWatch Container Insights.
```console
$ copilot env init --name prod-iad --profile prod-admin --container-insights
```

Creates an environment with imported VPC resources.
```console
$ copilot env init --import-vpc-id vpc-099c32d2b98cdcf47 \
  --import-public-subnets subnet-013e8b691862966cf,subnet-014661ebb7ab8681a \
  --import-private-subnets subnet-055fafef48fb3c547,subnet-00c9e76f288363e7f \
  --import-cert-arns arn:aws:acm:us-east-1:123456789012:certificate/12345678-1234-1234-1234-123456789012
```

Creates an environment with overridden CIDRs and AZs.

```console
$ copilot env init --override-vpc-cidr 10.1.0.0/16 \
  --override-az-names us-west-2b,us-west-2c \
  --override-public-cidrs 10.1.0.0/24,10.1.1.0/24 \
  --override-private-cidrs 10.1.2.0/24,10.1.3.0/24
```

## What does it look like?
![Running copilot env init](https://raw.githubusercontent.com/kohidave/copilot-demos/master/env-init.svg?sanitize=true)
