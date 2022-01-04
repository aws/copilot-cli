List of all available properties for a `'Backend Service'` manifest. To learn about Copilot services, see the [Services](../concepts/services.en.md) concept page.

???+ note "Sample manifest for an api service"

    ```yaml
    # Your service name will be used in naming your resources like log groups, ECS services, etc.
    name: api
    type: Backend Service

    # Your service is reachable at "http://api.${COPILOT_SERVICE_DISCOVERY_ENDPOINT}:8080" but is not public.

    # Configuration for your containers and service.
    image:
      build: ./api/Dockerfile
      port: 8080
      healthcheck:
        command: ["CMD-SHELL", "curl -f http://localhost:8080 || exit 1"]
        interval: 10s
        retries: 2
        timeout: 5s
        start_period: 0s

    cpu: 256
    memory: 512
    count: 1
    exec: true

    storage:
      volumes:
        myEFSVolume:
          path: '/etc/mount1'
          read_only: true
          efs:
            id: fs-12345678
            root_dir: '/'
            auth:
              iam: true
              access_point_id: fsap-12345678

    network:
      vpc:
        placement: 'private'
        security_groups: ['sg-05d7cd12cceeb9a6e']

    variables:
      LOG_LEVEL: info
    env_file: log.env
    secrets:
      GITHUB_TOKEN: GITHUB_TOKEN

    # You can override any of the values defined above by environment.
    environments:
      test:
        count:
          spot: 2
      production:
        count: 2
    ```

<a id="name" href="#name" class="field">`name`</a> <span class="type">String</span>
The name of your service.

<div class="separator"></div>

<a id="type" href="#type" class="field">`type`</a> <span class="type">String</span>  
The architecture type for your service. [Backend Services](../concepts/services.en.md#backend-service) are not reachable from the internet, but can be reached with [service discovery](../developing/service-discovery.en.md) from your other services.

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
