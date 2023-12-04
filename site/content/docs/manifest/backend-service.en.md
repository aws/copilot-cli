List of all available properties for a `'Backend Service'` manifest. To learn about Copilot services, see the [Services](../concepts/services.en.md) concept page.

???+ note "Sample backend service manifests"

    === "Serving Internal Traffic"

        ```yaml
            name: api
            type: Backend Service

            image:
              build: ./api/Dockerfile
              port: 8080
              healthcheck:
                command: ["CMD-SHELL", "curl -f http://localhost:8080 || exit 1"]
                interval: 10s
                retries: 2
                timeout: 5s
                start_period: 0s

            network:
              connect: true

            cpu: 256
            memory: 512
            count: 2
            exec: true

            env_file: ./api/.env
            environments:
              test:
                deployment:
                  rolling: "recreate"
                count: 1
        ```

    === "Internal Application Load Balancer"

        ```yaml
        # Your service is reachable at:
        # http://api.${COPILOT_ENVIRONMENT_NAME}.${COPILOT_APPLICATION_NAME}.internal
        # behind an internal load balancer only within your VPC.
        name: api
        type: Backend Service

        image:
          build: ./api/Dockerfile
          port: 8080

        http:
          path: '/'
          healthcheck:
            path: '/_healthcheck'
            success_codes: '200,301'
            healthy_threshold: 3
            interval: 15s
            timeout: 10s
            grace_period: 30s
          deregistration_delay: 50s

        network:
          vpc:
            placement: 'private'

        count:
          range: 1-10
          cpu_percentage: 70
          requests: 10
          response_time: 2s

        secrets:
          GITHUB_WEBHOOK_SECRET: GH_WEBHOOK_SECRET
          DB_PASSWORD:
            secretsmanager: 'demo/test/mysql:password::'
        ```

    === "With a domain"

        ```yaml
        # Assuming your environment has private certificates imported, you can assign
        # an HTTPS endpoint to your service.
        # See https://aws.github.io/copilot-cli/docs/manifest/environment/#http-private-certificates
        name: api
        type: Backend Service

        image:
          build: ./api/Dockerfile
          port: 8080

        http:
          path: '/'
          alias: 'v1.api.example.com'
          hosted_zone: AN0THE9H05TED20NEID # Insert record for v1.api.example.com to the hosted zone.

        count: 1
        ```

    === "Event-driven"

        ```yaml
        # See https://aws.github.io/copilot-cli/docs/developing/publish-subscribe/
        name: warehouse
        type: Backend Service

        image:
          build: ./warehouse/Dockerfile
          port: 80

        publish:
          topics:
            - name: 'inventory'
            - name: 'orders'
              fifo: true

        variables:
          DDB_TABLE_NAME: 'inventory'

        count:
          range: 3-5
          cpu_percentage: 70
          memory_percentage: 80
        ```

    === "Shared file system"

        ```yaml
        # See http://localhost:8000/copilot-cli/docs/developing/storage/#file-systems
        name: sync
        type: Backend Serivce

        image:
          build: Dockerfile

        variables:
          S3_BUCKET_NAME: my-userdata-bucket

        storage:
          volumes:
            userdata:
              path: /etc/mount1
              efs:
                id: fs-1234567
        ```

    === "Expose Multiple Ports"

        ```yaml
        name: 'backend'
        type: 'Backend Service'
    
        image:
          build: './backend/Dockerfile'
          port: 8080
    
        http:
          path: '/'
          target_port: 8083           # Traffic on "/" is forwarded to the main container, on port 8083. 
          additional_rules:
            - path: 'customerdb'
              target_port: 8081       # Traffic on "/customerdb" is forwarded to the main container, on port 8081.
            - path: 'admin' 
              target_port: 8082       # Traffic on "/admin" is forwarded to the sidecar "envoy", on port 8082.
              target_container: envoy
    
        sidecars:
          envoy:
            port: 80
            image: aws_account_id.dkr.ecr.us-west-2.amazonaws.com/envoy-proxy-with-selfsigned-certs:v1
        ```    

<a id="name" href="#name" class="field">`name`</a> <span class="type">String</span>
The name of your service.

<div class="separator"></div>

<a id="type" href="#type" class="field">`type`</a> <span class="type">String</span>  
The architecture type for your service. [Backend Services](../concepts/services.en.md#backend-service) are not reachable from the internet, but can be reached with [service discovery](../developing/svc-to-svc-communication.en.md#service-discovery) from your other services.

<div class="separator"></div>

<a id="http" href="#http" class="field">`http`</a> <span class="type">Map</span>  
The http section contains parameters related to integrating your service with an internal Application Load Balancer.

<span class="parent-field">http.</span><a id="http-path" href="#http-path" class="field">`path`</a> <span class="type">String</span>  
Requests to this path will be forwarded to your service. Each Backend Service should listen on a unique path.

<span class="parent-field">http.</span><a id="http-alb" href="#http-alb" class="field">`alb`</a> <span class="type">String</span> <span class="version">Added in [v1.33.0](../../blogs/release-v133.en.md#imported-albs)</span>  
The ARN or name of an existing internal ALB to import. Listener rules will be added to your listener(s). Copilot will not manage DNS-related resources like certificates.

{% include 'http-healthcheck.en.md' %}

<span class="parent-field">http.</span><a id="http-deregistration-delay" href="#http-deregistration-delay" class="field">`deregistration_delay`</a> <span class="type">Duration</span>  
The amount of time to wait for targets to drain connections during deregistration. The default is 60s. Setting this to a larger value gives targets more time to gracefully drain connections, but increases the time required for new deployments. Range 0s-3600s.

<span class="parent-field">http.</span><a id="http-target-container" href="#http-target-container" class="field">`target_container`</a> <span class="type">String</span>  
A sidecar container that requests are routed to instead of the main service container.  
If the target container's port is set to `443`, then the protocol is set to `HTTPS` so that the load balancer establishes
TLS connections with the Fargate tasks using certificates that you install on the target container.

<span class="parent-field">http.</span><a id="http-stickiness" href="#http-stickiness" class="field">`stickiness`</a> <span class="type">Boolean</span>  
Indicates whether sticky sessions are enabled.

<span class="parent-field">http.</span><a id="http-allowed-source-ips" href="#http-allowed-source-ips" class="field">`allowed_source_ips`</a> <span class="type">Array of Strings</span>  
CIDR IP addresses permitted to access your service.
```yaml
http:
  allowed_source_ips: ["192.0.2.0/24", "198.51.100.10/32"]
```

<span class="parent-field">http.</span><a id="http-alias" href="#http-alias" class="field">`alias`</a> <span class="type">String or Array of Strings or Array of Maps</span>  
HTTPS domain alias of your service.
```yaml
# String version.
http:
  alias: example.com
# Alternatively, as an array of strings.
http:
  alias: ["example.com", "v1.example.com"]
# Alternatively, as an array of maps.
http:
  alias:
    - name: example.com
      hosted_zone: Z0873220N255IR3MTNR4
    - name: v1.example.com
      hosted_zone: AN0THE9H05TED20NEID
```
<span class="parent-field">http.</span><a id="http-hosted-zone" href="#http-hosted-zone" class="field">`hosted_zone`</a> <span class="type">String</span>  
ID of existing private hosted zone, into which Copilot will insert the alias record once the internal load balancer is created, mapping the alias name to the LB's DNS name. Must be used with `alias`.
```yaml
http:
  alias: example.com
  hosted_zone: Z0873220N255IR3MTNR4
# Also see http.alias array of maps example, above.
```
<span class="parent-field">http.</span><a id="http-version" href="#http-version" class="field">`version`</a> <span class="type">String</span>  
The HTTP(S) protocol version. Must be one of `'grpc'`, `'http1'`, or `'http2'`. If omitted, then `'http1'` is assumed.
If using gRPC, please note that a domain must be associated with your application.

<span class="parent-field">http.</span><a id="http-additional-rules" href="#http-additional-rules" class="field">`additional_rules`</a> <span class="type">Array of Maps</span>  
Configure multiple ALB listener rules.

{% include 'http-additionalrules.en.md' %}

{% include 'image-config-with-port.en.md' %}  
If the port is set to `443` and an internal load balancer is enabled with `http`, then the protocol is set to `HTTPS` so that the load balancer establishes
TLS connections with the Fargate tasks using certificates that you install on the container.

{% include 'image-healthcheck.en.md' %}

{% include 'task-size.en.md' %}

{% include 'platform.en.md' %}

<div class="separator"></div>

<a id="count" href="#count" class="field">`count`</a> <span class="type">Integer or Map</span>
The number of tasks that your service should maintain.

If you specify a number:
```yaml
count: 5
```
The service will set the desired count to 5 and maintain 5 tasks in your service.

<span class="parent-field">count.</span><a id="count-spot" href="#count-spot" class="field">`spot`</a> <span class="type">Integer</span>

If you want to use Fargate Spot capacity to run your services, you can specify a number under the `spot` subfield:
```yaml
count:
  spot: 5
```
!!! info
    Fargate Spot is not supported for containers running on ARM architecture.

<div class="separator"></div>

Alternatively, you can specify a map for setting up autoscaling:
```yaml
count:
  range: 1-10
  cooldown:
    in: 30s
  cpu_percentage: 70
  memory_percentage:
    value: 80
    cooldown:
      out: 45s
  requests: 10000
  response_time: 2s
```

<span class="parent-field">count.</span><a id="count-range" href="#count-range" class="field">`range`</a> <span class="type">String or Map</span>
You can specify a minimum and maximum bound for the number of tasks your service should maintain, based on the values you specify for the metrics.
```yaml
count:
  range: n-m
```
This will set up an Application Autoscaling Target with the `MinCapacity` of `n` and `MaxCapacity` of `m`.

Alternatively, if you wish to scale your service onto Fargate Spot instances, specify `min` and `max` under `range` and then specify `spot_from` with the desired count you wish to start placing your services onto Spot capacity. For example:

```yaml
count:
  range:
    min: 1
    max: 10
    spot_from: 3
```

This will set your range as 1-10 as above, but will place the first two copies of your service on dedicated Fargate capacity. If your service scales to 3 or higher, the third and any additional copies will be placed on Spot until the maximum is reached.

<span class="parent-field">count.range.</span><a id="count-range-min" href="#count-range-min" class="field">`min`</a> <span class="type">Integer</span>
The minimum desired count for your service using autoscaling.

<span class="parent-field">count.range.</span><a id="count-range-max" href="#count-range-max" class="field">`max`</a> <span class="type">Integer</span>
The maximum desired count for your service using autoscaling.

<span class="parent-field">count.range.</span><a id="count-range-spot-from" href="#count-range-spot-from" class="field">`spot_from`</a> <span class="type">Integer</span>
The desired count at which you wish to start placing your service using Fargate Spot capacity providers.

<span class="parent-field">count.</span><a id="count-cooldown" href="#count-cooldown" class="field">`cooldown`</a> <span class="type">Map</span>
Cooldown scaling fields that are used as the default cooldown for all autoscaling fields specified.

<span class="parent-field">count.cooldown.</span><a id="count-cooldown-in" href="#count-cooldown-in" class="field">`in`</a> <span class="type">Duration</span>
The cooldown time for autoscaling fields to scale up the service.

<span class="parent-field">count.cooldown.</span><a id="count-cooldown-out" href="#count-cooldown-out" class="field">`out`</a> <span class="type">Duration</span>
The cooldown time for autoscaling fields to scale down the service.

The following options `cpu_percentage`, `memory_percentage`, `requests` and `response_time` are autoscaling fields for `count` which can be defined either as the value of the field, or as a Map containing advanced information about the field's `value` and `cooldown`:
```yaml
value: 50
cooldown:
  in: 30s
  out: 60s
```
The cooldown specified here will override the default cooldown.

<span class="parent-field">count.</span><a id="count-cpu-percentage" href="#count-cpu-percentage" class="field">`cpu_percentage`</a> <span class="type">Integer or Map</span>
Scale up or down based on the average CPU your service should maintain.

<span class="parent-field">count.</span><a id="count-memory-percentage" href="#count-memory-percentage" class="field">`memory_percentage`</a> <span class="type">Integer or Map</span>
Scale up or down based on the average memory your service should maintain.

<span class="parent-field">count.</span><a id="requests" href="#count-requests" class="field">`requests`</a> <span class="type">Integer or Map</span>
Scale up or down based on the request count handled per task.

<span class="parent-field">count.</span><a id="response-time" href="#count-response-time" class="field">`response_time`</a> <span class="type">Duration or Map</span>
Scale up or down based on the service average response time.

{% include 'exec.en.md' %}

{% include 'deployment.en.md' %}
```yaml
deployment:
  rollback_alarms:
    cpu_utilization: 70    // Percentage value at or above which alarm is triggered.
    memory_utilization: 50 // Percentage value at or above which alarm is triggered.
```

{% include 'entrypoint.en.md' %}

{% include 'command.en.md' %}

{% include 'network.en.md' %}

{% include 'envvars.en.md' %}

{% include 'secrets.en.md' %}

{% include 'storage.en.md' %}

{% include 'publish.en.md' %}

{% include 'logging.en.md' %}

{% include 'observability.en.md' %}

{% include 'taskdef-overrides.en.md' %}

{% include 'environments.en.md' %}
