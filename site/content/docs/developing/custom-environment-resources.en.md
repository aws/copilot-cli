# Custom Environment Resources

## Importing existing resources
When creating a new [environment](../concepts/environments.en.md) with Copilot, you are given the option to import existing VPC resources. (Use [flags with `env init`](../commands/env-init.en.md#what-are-the-flags) or the guided experience, shown below.)
```bash
$ copilot env init
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

You may use the import feature to bring a VPC with two public and two private subnets, only two public subnets and no private subnets (such as a default VPC), or only two private subnets and no public subnets for your workloads that are not internet-facing. (For more details on the resources you'll need for isolated networks, go [here](https://github.com/aws/copilot-cli/discussions/2378).)

## Modifying Copilot's default resources 
When you select the default configuration, Copilot follows [AWS best practices](https://aws.amazon.com/blogs/containers/amazon-ecs-availability-best-practices/) and creates a VPC with two public and two private subnets, with one of each type in one of two Availability Zones. 
If you require additional availability zones or need to modify the CIDR ranges, you can opt in to modify these settings:
```bash 
$ copilot env init --container-insights
What is your environment's name? env-name
Which credentials would you like to use to create name? [profile default]

  Would you like to use the default configuration for a new environment?
    - A new VPC with 2 AZs, 2 public subnets and 2 private subnets
    - A new ECS Cluster
    - New IAM Roles to manage services and jobs in your environment
  [Use arrows to move, type to filter]
    Yes, use default.
  > Yes, but I'd like configure the default resources (CIDR ranges).
    No, I'd like to import existing resources (VPC, subnets).
    
  What VPC CIDR would you like to use? [? for help] (10.0.0.0/16)
  
  Which availability zones would you like to use?  [Use arrows to move, space to select, type to filter, ? for more help]
  [x]  us-west-2a
  [x]  us-west-2b
  > [x]  us-west-2c
  [ ]  us-west-2d
  
  What CIDR would you like to use for your public subnets? [? for help] (10.0.0.0/24,10.0.1.0/24) 10.0.0.0/24,10.0.1.0/24,10.0.2.0/24
  What CIDR would you like to use for your private subnets? [? for help] (10.0.2.0/24,10.0.3.0/24) 10.0.3.0/24,10.0.4.0/24,10.0.5.0/24
```

## Considerations
* If you are importing an existing VPC, we recommend following [Security best practices for your VPC](https://docs.aws.amazon.com/vpc/latest/userguide/vpc-security-best-practices.html) and the [Security & Filtering section from the Amazon VPC FAQs](https://aws.amazon.com/vpc/faqs/#Security_and_Filtering).
* If you are using a private hosted zone, [you must](https://docs.aws.amazon.com/Route53/latest/DeveloperGuide/hosted-zone-private-considerations.html#hosted-zone-private-considerations-vpc-settings) set `enableDnsHostname` and `enableDnsSupport` to true.
* To deploy internet-facing workloads in [private subnets](../manifest/lb-web-service.en.md#network-vpc-placement), your VPC will need a [NAT gateway](https://docs.aws.amazon.com/vpc/latest/userguide/vpc-nat-gateway.html). 
