# Internal Application Load Balancers

By default, the ALBs created for environments with Load Balanced Web Services are [internet-facing](https://docs.aws.amazon.com/elasticloadbalancing/latest/classic/elb-internet-facing-load-balancers.html). To create an [internal load balancer](https://docs.aws.amazon.com/elasticloadbalancing/latest/classic/elb-internal-load-balancers.html) whose nodes have only private IP addresses, you'll need to configure a few things when you initiate your environment and workload.

## Environment

The internal load balancer is an environment-level resource, to be shared among other permitted services. When you run `copilot env init`, you can import some specific resources to support the ALB. For services with `https` capability, use the [`--import-cert-arns`](../commands/env-init.en.md#what-are-the-flags) flag to import the ARNs of your existing private certificates. If you'd like your ALB to receive ingress traffic within the environment VPC, use the [`--internal-alb-allow-vpc-ingress`](../commands/env-init.en.md#what-are-the-flags) flag; otherwise, by default, access to the internal ALB will be limited to only Copilot-created services within the environment.

!!!info
    Today, Copilot will associate imported certs with an internal ALB *only if* the environment's VPC has no public subnets; otherwise, they will be associated with the default internet-facing load balancer. When initiating your environment, you may use the `--import-vpc-id` and `--import-private-subnets` flags to pass in your VPC and subnet IDs along with `--import-cert-arns`. For a more managed experience, you may use just the `--import-cert-arns` flag with `copilot env init`, then follow the prompts to import your existing VPC resources, opting out of importing public subnets. (Copilot's env config will soon have increased flexibility...stay tuned!)

## Service

The only service type that you can place behind an internal load balancer is a [Backend Service](https://aws.github.io/copilot-cli/docs/concepts/services/#backend-service). To tell Copilot to generate an internal ALB in the environment in which you deploy this service, add the `http` field to your Backend Service's workload manifest:

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
You can specify exactly which private subnets your internal ALB is placed in.

When you run `copilot env init`, use the [`--internal-alb-subnets`](../commands/env-init.en.md#what-are-the-flags) flag to pass in the IDs of the subnets in which you'd like the ALB to be placed.

### Aliases, Health Checks, and More
The `http` field for Backend Services has all the subfields and capabilities that Load Balanced Web Services's `http` field has.

``` yaml
http:
  path: '/'
  healthcheck:
    path: '/_healthcheck'
    port: 8080
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

For `alias`, you may 1. bring your own existing private hosted zone(s), or 2. add your own alias record(s) after deployment, independently of Copilot. You may add a single alias:
```yaml
http:
  alias: example.com
  hosted_zone: HostedZoneID1
```
or multiple aliases that share a hosted zone:
```yaml
http:
  alias: ["example.com", "www.example.com"]
  hosted_zone: HostedZoneID1
```
or multiple aliases, some of which use the top-level hosted zone:
```yaml
http:
  hosted_zone: HostedZoneID1
  aliases:
    - name: example.com
    - name: www.example.com
    - name: something-different.com
      hosted_zone: HostedZoneID2
```

