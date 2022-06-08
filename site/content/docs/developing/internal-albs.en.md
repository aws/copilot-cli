# Internal Application Load Balancers

By default, the ALBs created for environments with Load Balanced Web Services are [internet-facing](https://docs.aws.amazon.com/elasticloadbalancing/latest/classic/elb-internet-facing-load-balancers.html). To create an [internal](https://docs.aws.amazon.com/elasticloadbalancing/latest/classic/elb-internal-load-balancers.html) load balancer whose nodes have only private IP addresses, you'll need to configure a few things when you initiate your environment and workload.

## Environment

The internal load balancer is an environment-level resource, to be shared among other permitted services. When you run `copilot env init`, you can import some specific resources to support the ALB. For services with `https` capability, use the [`--import-cert-arns`](../commands/env-init.en.md#what-are-the-flags) flag to import the ARNs of your existing private certificates.

!!!info
    Today, Copilot will associate imported certs with an internal ALB *only if* the environment's VPC has no public subnets; otherwise, they will be associated with the default internet-facing load balancer. When initiating your environment, you may use the `--import-vpc-id` and `--import-private-subnets` flags to pass in your VPC and subnet IDs along with `--import-cert-arns`. For a more managed experience, you may use just the `--import-cert-arns` flag with `copilot env init`, then follow the prompts to import your existing VPC resources, opting out of importing public subnets. (Copilot's env config will soon have increased flexibility...stay tuned!)

## Service

The only service type that you can place behind an internal load balancer is a [Backend Service](https://aws.github.io/copilot-cli/docs/concepts/services/#backend-service). These workloads, by definition, are not internet-facing. To tell Copilot to generate an ALB in the environment in which you deploy this service, add the `http` field to your Backend Service's workload manifest:

```yaml
# in copilot/{service name}/manifest.yml
http:
  path: '/'
network:
  vpc:
    placement: private
```

!!!attention
    Currently, you must use a new Backend Service that has not yet been deployed. Very soon, you will be able to add internal load balancers to existing Backend Services!

## Advanced Configuration

### Subnet Placement
Maybe you would like your internal ALB and service(s) placed in different subnets than each other, or just want to specify subnet placement for one or the other...or both.

With `copilot env init`, use the [`internal-alb-subnets`](../commands/env-init.en.md#what-are-the-flags) flag to pass in the IDs of the subnets in which you'd like the ALB to be placed.

Call out specific subnets in your Backend Service manifest:

```yaml
# in copilot/{service name}/manifest.yml
http:
  path: '/'
network:
  vpc:
    placement:
      subnets: ["PrivateSubnetID1", "PrivateSubnetID2"]
```

### Aliases, Health Checks, and More
The `http` field for Backend Services has all the subfields and capabilities as that for Load Balanced Web Services.

``` yaml
http:
  path: '/'
  healthcheck:
    path: '/_healthcheck'
    success_codes: '200,301'
    healthy_threshold: 3
    unhealthy_threshold: 2
    interval: 15s
    timeout: 10s
    grace_period: 45s
  deregistration_delay: 5s
  stickiness: false
  allowed_source_ips: ["10.24.34.0/23"]
  alias: example.com
```

For `alias`, you may 1. bring your own existing private hosted zone(s), or 2. add your own alias records after deployment, independently of Copilot. You may add a single alias:
```yaml
http:
  alias: example.com
  hosted_zone: HostedZoneID1
```
or multiple aliases:
```yaml
http:
  alias: ["example.com", "v1.example.com"]
```
or
```yaml
http:
  aliases:
    - name: example.com
      hosted_zone: HostedZoneID1
    - name: another.example.com
    - name: still.another.example.com
      hosted_zone: HostedZoneID2
```

