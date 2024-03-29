# Internal Application Load Balancers

By default, the ALBs created for environments with Load Balanced Web Services are [internet-facing](https://docs.aws.amazon.com/elasticloadbalancing/latest/classic/elb-internet-facing-load-balancers.html). To create an [internal load balancer](https://docs.aws.amazon.com/elasticloadbalancing/latest/classic/elb-internal-load-balancers.html) whose nodes have only private IP addresses, you'll need to configure a few things in your environment and workload.

## Environment

The internal load balancer is an environment-level resource, to be shared among other permitted services.
To enable HTTPS for your load balancer, modify the [environment manifest](../manifest/environment.en.md#http-private) to 
import the ARNs of your existing certificates.

If there are no certificates applied to the load balancer, Copilot will associate the load balancer with the 
`http://{env name}.{app name}.internal` endpoint and the individual services are reachable at `http://{service name}.{env name}.{app name}.internal`.
```go
// To reach the "api" service behind the internal load balancer
endpoint := fmt.Sprintf("http://api.%s.%s.internal", os.Getenv("COPILOT_ENVIRONMENT_NAME"), os.Getenv("COPILOT_APPLICATION_NAME"))
resp, err := http.Get(endpoint)
```

## Service

The only service type that you can place behind an internal load balancer is a [Backend Service](../concepts/services.en.md#backend-service). To tell Copilot to generate an internal ALB in the environment in which you deploy this service, add the `http` field to your Backend Service's workload manifest:

```yaml
# in copilot/{service name}/manifest.yml
http:
  path: '/'
network:
  vpc:
    placement: private
```
If you have an existing internal ALB in the VPC to which your service will be deployed, you may import it per Backend Service by specifying it in your manifest before deployment:
```yaml
http:
  path: '/'
  alb: [name or ARN]
```

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
  alias:
    - name: example.com
    - name: www.example.com
    - name: something-different.com
      hosted_zone: HostedZoneID2
```

