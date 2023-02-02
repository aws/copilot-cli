# AWS Copilot v1.19: Internal Load Balancers, Subnet Placement Specification, and more

Posted On: Jun 13, 2022

The AWS Copilot core team is excited to announce the v1.19 release!
Special thanks to [@gautam-nutalapati](https://github.com/gautam-nutalapati) and [@jonstacks](https://github.com/jonstacks), who contributed to this release.
Our public [сommunity сhat](https://gitter.im/aws/copilot-cli) is growing and has nearly 300 people online,
who help each other daily. Thanks to every one of you who shows love and support for AWS Copilot.

Copilot v1.19 brings brand-new features and several improvements:

* **Load Balancing for Backend Services:** You can now add an Application Load Balancer that is **internal** (as opposed to 'internet-facing', like those created for Load Balanced Web Services). [See detailed section](./#internal-load-balancers).
* **Subnet Placement Specification**:
You now have even finer-grained control over where your ECS tasks are launched. Beyond `public` and `private` subnet placement, you can now tell Copilot specific subnets. Simply add the IDs of the desired subnets to your workload manifest.
```yaml
# in copilot/{service name}/manifest.yml
network:
  vpc:
    placement:
      subnets: ["SubnetID1", "SubnetID2"]
```
* **Hosted Zones–A Record Management**:
You can now list, along with aliases, the IDs of hosted zones in your service manifest. Copilot will handle the insertion of A records upon deployment to an environment with imported certificates. ([#3608](https://github.com/aws/copilot-cli/pull/3608), [#3643](https://github.com/aws/copilot-cli/pull/3643))
```yaml
# single alias and hosted zone
http:
  alias: example.com
  hosted_zone: HostedZoneID1

# multiple aliases that share a hosted zone
http:
  alias: ["example.com", "www.example.com"]
  hosted_zone: HostedZoneID1

# multiple aliases, some of which use the top-level hosted zone
http:
  hosted_zone: HostedZoneID1
  alias:
    - name: example.com
    - name: www.example.com
    - name: something-different.com
      hosted_zone: HostedZoneID2
```
* **Access to Created Private Route Tables**:
Copilot now exports private route table IDs from CloudFormation environment stacks. Use them to create VPC gateway endpoints with [addons](../docs/developing/addons/workload.en.md). ([#3611](https://github.com/aws/copilot-cli/pull/3611))
* **`port` for Target Group Health Checks**:
With the new `port` field, you can configure a non-default port for health checks, one different than that for requests from the load balancer. ([#3548](https://github.com/aws/copilot-cli/pull/3548))
```yaml
http:
  path: '/'
  healthcheck:
    port: 8080
```

* **Bug fixes:**
    * Preserve tags applied by `app init --resource-tags` when services are deleted from an application ([#3582](https://github.com/aws/copilot-cli/pull/3582))
    * Fix regression when enabling autoscaling fields for Load Balanced Web Services with Network Load Balancers ([#3578](https://github.com/aws/copilot-cli/pull/3578))
    * Enable `copilot svc exec` for Fargate Windows tasks ([#3566](https://github.com/aws/copilot-cli/pull/3566))

There are no breaking changes in this release.

## What’s AWS Copilot?

The AWS Copilot CLI is a tool for developers to build, release, and operate production ready containerized applications on AWS.
From getting started, pushing to staging, and releasing to production, Copilot can help manage the entire lifecycle of your application development.
At the foundation of Copilot is AWS CloudFormation, which enables you to provision infrastructure as code in a single operation.
Copilot provides pre-defined CloudFormation templates and user-friendly workflows for different types of micro services creation and operation,
enabling you to focus on developing your application, instead of writing deployment scripts.

See the section [Overview](../docs/concepts/overview.en.md) for a more detailed introduction to AWS Copilot.

## Internal Load Balancers
_Contributed by [Janice Huang](https://github.com/huanjani) and [Danny Randall](https://github.com/dannyrandall)_  
By configuring a few things when you initiate your Copilot environment and workload, you can now create an [internal load balancer](https://docs.aws.amazon.com/elasticloadbalancing/latest/classic/elb-internal-load-balancers.html), whose nodes have only private IP addresses.

The internal load balancer is an environment-level resource, to be shared among other permitted services. When you run `copilot env init`, you can import some specific resources to support the ALB. For services with `https` capability, use the [`--import-cert-arns`](../docs/commands/env-init.en.md#what-are-the-flags) flag to import the ARNs of your existing private certificates.  For now, Copilot will associate imported certs with an internal ALB *only if* the environment's VPC has no public subnets, so import only private subnets. If you'd like your ALB to receive ingress traffic within the environment VPC, use the [`--internal-alb-allow-vpc-ingress`](../docs/commands/env-init.en.md#what-are-the-flags) flag; otherwise, by default, access to the internal ALB will be limited to only Copilot-created services within the environment.

The only service type that you can place behind an internal load balancer is a [Backend Service](../docs/concepts/services.en.md#backend-service). To tell Copilot to generate an internal ALB in the environment in which you deploy this service, add the `http` field to your Backend Service's workload manifest:

```yaml
# in copilot/{service name}/manifest.yml
http:
  path: '/'
  network:
    vpc:
      placement: private
  # for https
  alias: example.aws
  hosted_zone: Z0873220N255IR3MTNR4
```
For more, read our documentation on [Internal ALBs](../docs/developing/internal-albs.en.md)!

## What’s next?

Download the new Copilot CLI version by following the link below and leave your feedback on [GitHub](https://github.com/aws/copilot-cli/) or our [Community Chat](https://gitter.im/aws/copilot-cli):

* Download [the latest CLI version](../docs/getting-started/install.en.md)
* Try our [Getting Started Guide](../docs/getting-started/first-app-tutorial.en.md)
* Read full release notes on [GitHub](https://github.com/aws/copilot-cli/releases/tag/v1.19.0)
