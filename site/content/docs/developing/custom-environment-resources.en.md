# Custom Environment Resources

When creating a new [environment](../../concepts/environments.en.md) with Copilot, you are given the option to import existing VPC resources. (Use [flags with `env init`](../commands/env-init.en.md) or the guided experience, shown below.)
```bash
% copilot env init
What is your environment's name? env-name
Which credentials would you like to use to create name? [profile default]

  Would you like to use the default configuration for a new environment?
    - A new VPC with 2 AZs, 2 public subnets and 2 private subnets
    - A new ECS Cluster
    - New IAM Roles to manage services and jobs in your environment
  [Use arrows to move, type to filter]
    Yes, use default.
    Yes, but I'd like configure the default resources (CIDR ranges).
  > No, I'd like to import existing resources (VPC, subnets).
```

When you select the default configuration, Copilot follows [AWS best practices](https://aws.amazon.com/blogs/containers/amazon-ecs-availability-best-practices/) and creates a VPC with two public and two private subnets, with one of each type in one of two Availability Zones. While this is a good configuration for most cases, Copilot allows some flexibility when you import your own resources. For example, you may bring a VPC with only two private subnets and no public subnets for your workloads that are not internet-facing. (For more details on the resources you'll need for isolated networks, go [here](https://github.com/aws/copilot-cli/discussions/2378).)

## Considerations
* If you are using a private hosted zone, [you must](https://docs.aws.amazon.com/Route53/latest/DeveloperGuide/hosted-zone-private-considerations.html#hosted-zone-private-considerations-vpc-settings) set `enableDnsHostname` and `enableDnsSupport` to true.
* To deploy internet-facing workloads in [private subnets](../include/common-svc-fields.md), your VPC will need a [NAT gateway](https://docs.aws.amazon.com/vpc/latest/userguide/vpc-nat-gateway.html). 
