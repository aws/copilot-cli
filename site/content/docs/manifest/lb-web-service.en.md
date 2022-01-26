List of all available properties for a `'Load Balanced Web Service'` manifest. To learn about Copilot services, see the [Services](../concepts/services.en.md) concept page.

???+ note "Sample manifest for a frontend service"

    ```yaml
    # Your service name will be used in naming your resources like log groups, ECS services, etc.
    name: frontend
    type: Load Balanced Web Service

    # Distribute traffic to your service.
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

    nlb:
      port: 443/tls

    # Configuration for your containers and service.
    image:
      build:
        dockerfile: ./frontend/Dockerfile
        context: ./frontend
      port: 80

    cpu: 256
    memory: 512
    count:
      range: 1-10
      cpu_percentage: 70
      memory_percentage: 80
      requests: 10000
      response_time: 2s
    exec: true

    variables:
      LOG_LEVEL: info
    env_file: log.env
    secrets:
      GITHUB_TOKEN: GITHUB_TOKEN

    # You can override any of the values defined above by environment.
    environments:
      test:
        count:
          range:
            min: 1
            max: 10
            spot_from: 2
      staging:
        count:
          spot: 2
      production:
        count: 2
    ```

<a id="name" href="#name" class="field">`name`</a> <span class="type">String</span>  
The name of your service.

<div class="separator"></div>

<a id="type" href="#type" class="field">`type`</a> <span class="type">String</span>  
The architecture type for your service. A [Load Balanced Web Service](../concepts/services.en.md#load-balanced-web-service) is an internet-facing service that's behind a load balancer, orchestrated by Amazon ECS on AWS Fargate.

{% include 'http-config.en.md' %}

{% include 'nlb.en.md' %}

{% include 'image-config-with-port.en.md' %}

{% include 'image-healthcheck.en.md' %}

{% include 'task-size.en.md' %}

{% include 'platform.en.md' %}

<div class="separator"></div>

<a id="count" href="#count" class="field">`count`</a> <span class="type">Integer or Map</span>  
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
  cpu_percentage: 70
  memory_percentage: 80
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

<span class="parent-field">range.</span><a id="count-range-min" href="#count-range-min" class="field">`min`</a> <span class="type">Integer</span>  
The minimum desired count for your service using autoscaling.

<span class="parent-field">range.</span><a id="count-range-max" href="#count-range-max" class="field">`max`</a> <span class="type">Integer</span>  
The maximum desired count for your service using autoscaling.

<span class="parent-field">range.</span><a id="count-range-spot-from" href="#count-range-spot-from" class="field">`spot_from`</a> <span class="type">Integer</span>  
The desired count at which you wish to start placing your service using Fargate Spot capacity providers.

<span class="parent-field">count.</span><a id="count-cpu-percentage" href="#count-cpu-percentage" class="field">`cpu_percentage`</a> <span class="type">Integer</span>  
Scale up or down based on the average CPU your service should maintain.

<span class="parent-field">count.</span><a id="count-memory-percentage" href="#count-memory-percentage" class="field">`memory_percentage`</a> <span class="type">Integer</span>  
Scale up or down based on the average memory your service should maintain.

<span class="parent-field">count.</span><a id="requests" href="#count-requests" class="field">`requests`</a> <span class="type">Integer</span>  
Scale up or down based on the request count handled per tasks.

<span class="parent-field">count.</span><a id="response-time" href="#count-response-time" class="field">`response_time`</a> <span class="type">Duration</span>  
Scale up or down based on the service average response time.

{% include 'exec.en.md' %}

{% include 'entrypoint.en.md' %}

{% include 'command.en.md' %}

{% include 'network.en.md' %}

{% include 'envvars.en.md' %}

{% include 'secrets.en.md' %}

{% include 'storage.en.md' %}

{% include 'publish.en.md' %}

{% include 'logging.en.md' %}

{% include 'taskdef-overrides.en.md' %}

{% include 'environments.en.md' %}
