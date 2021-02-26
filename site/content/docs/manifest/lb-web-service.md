List of all available properties for a `'Load Balanced Web Service'` manifest.

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
        healthy_threshold: 3
        unhealthy_threshold: 2
        interval: 15s
        timeout: 10s
      stickiness: false
      allowed_source_ips: ["10.24.34.0/23"]
    
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
    
    variables:
      LOG_LEVEL: info
    secrets:
      GITHUB_TOKEN: GITHUB_TOKEN

    # Configuration for task storage using EFS volumes.
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
    
    
    # You can override any of the values defined above by environment.
    environments:
      production:
        count: 2
    ```

<a id="name" href="#name" class="field">`name`</a> <span class="type">String</span>  
The name of your service.   

<div class="separator"></div>

<a id="type" href="#type" class="field">`type`</a> <span class="type">String</span>  
The architecture type for your service. A [Load Balanced Web Service](../concepts/services.md#load-balanced-web-service) is an internet-facing service that's behind a load balancer, orchestrated by Amazon ECS on AWS Fargate.  

<div class="separator"></div>

<a id="http" href="#http" class="field">`http`</a> <span class="type">Map</span>   
The http section contains parameters related to integrating your service with an Application Load Balancer.

<span class="parent-field">http.</span><a id="http-path" href="#http-path" class="field">`path`</a> <span class="type">String</span>  
Requests to this path will be forwarded to your service. Each Load Balanced Web Service should listen on a unique path.

<span class="parent-field">http.</span><a id="http-healthcheck" href="#http-healthcheck" class="field">`healthcheck`</a> <span class="type">String or Map</span>  
If you specify a string, Copilot interprets it as the path exposed in your container to handle target group health check requests. The default is "/".
```yaml
http:
  healthcheck: '/'
```
You can also specify healthcheck as a map:
```yaml
http:
  healthcheck:
    path: '/'
    healthy_threshold: 3
    unhealthy_threshold: 2
    interval: 15s
    timeout: 10s
```

<span class="parent-field">http.healthcheck.</span><a id="http-healthcheck-healthy-threshold" href="#http-healthcheck-healthy-threshold" class="field">`healthy_threshold`</a> <span class="type">Integer</span>  
The number of consecutive health check successes required before considering an unhealthy target healthy. The Copilot default is 2. Range: 2-10.

<span class="parent-field">http.healthcheck.</span><a id="http-healthcheck-unhealthy-threshold" href="#http-healthcheck-unhealthy-threshold" class="field">`unhealthy_threshold`</a> <span class="type">Integer</span>  
The number of consecutive health check failures required before considering a target unhealthy. The Copilot default is 2. Range: 2-10.

<span class="parent-field">http.healthcheck.</span><a id="http-healthcheck-interval" href="#http-healthcheck-interval" class="field">`interval`</a> <span class="type">Duration</span>  
The approximate amount of time, in seconds, between health checks of an individual target. The Copilot default is 10s. Range: 5s–300s.

<span class="parent-field">http.healthcheck.</span><a id="http-healthcheck-timeout" href="#http-healthcheck-timeout" class="field">`timeout`</a> <span class="type">Duration</span>  
The amount of time, in seconds, during which no response from a target means a failed health check. The Copilot default is 5s. Range 5s-300s.

<span class="parent-field">http.</span><a id="http-target-container" href="#http-target-container" class="field">`target_container`</a> <span class="type">String</span>  
A sidecar container that takes the place of a service container.

<span class="parent-field">http.</span><a id="http-stickiness" href="#http-stickiness" class="field">`stickiness`</a> <span class="type">Boolean</span>  
Indicates whether sticky sessions are enabled.

<span class="parent-field">http.</span><a id="http-allowed-source-ips" href="#http-allowed-source-ips" class="field">`allowed_source_ips`</a> <span class="type">Array of Strings</span>  
CIDR IP addresses permitted to access your service.
```yaml
http:
  allowed_source_ips: ["192.0.2.0/24", "198.51.100.10/32"]
```

<div class="separator"></div>

<a id="image" href="#image" class="field">`image`</a> <span class="type">Map</span>  
The image section contains parameters relating to the Docker build configuration and exposed port.  

<span class="parent-field">image.</span><a id="image-build" href="#image-build" class="field">`build`</a> <span class="type">String or Map</span>  
If you specify a string, Copilot interprets it as the path to your Dockerfile. It will assume that the dirname of the string you specify should be the build context. The manifest:
```yaml
image:
  build: path/to/dockerfile
```
will result in the following call to docker build: `$ docker build --file path/to/dockerfile path/to`

You can also specify build as a map:
```yaml
image:
  build:
    dockerfile: path/to/dockerfile
    context: context/dir
    target: build-stage
    cache_from:
      - image:tag
    args:
      key: value
```
In this case, Copilot will use the context directory you specified and convert the key-value pairs under args to --build-arg overrides. The equivalent docker build call will be:  
`$ docker build --file path/to/dockerfile --target build-stage --cache-from image:tag --build-arg key=value context/dir`.

You can omit fields and Copilot will do its best to understand what you mean. For example, if you specify `context` but not `dockerfile`, Copilot will run Docker in the context directory and assume that your Dockerfile is named "Dockerfile." If you specify `dockerfile` but no `context`, Copilot assumes you want to run Docker in the directory that contains `dockerfile`.

All paths are relative to your workspace root.

<span class="parent-field">image.</span><a id="image-location" href="#image-location" class="field">`location`</a> <span class="type">String</span>  
Instead of building a container from a Dockerfile, you can specify an existing image name. Mutually exclusive with [`image.build`](#image-build).    
The `location` field follows the same definition as the [`image` parameter](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/task_definition_parameters.html#container_definition_image) in the Amazon ECS task definition.

<span class="parent-field">image.</span><a id="image-port" href="#image-port" class="field">`port`</a> <span class="type">Integer</span>  
The port exposed in your Dockerfile. Copilot should parse this value for you from your `EXPOSE` instruction.

<div class="separator"></div>

<a id="cpu" href="#cpu" class="field">`cpu`</a> <span class="type">Integer</span>  
Number of CPU units for the task. See the [Amazon ECS docs](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/task-cpu-memory-error.html) for valid CPU values.

<div class="separator"></div>

<a id="memory" href="#memory" class="field">`memory`</a> <span class="type">Integer</span>  
Amount of memory in MiB used by the task. See the [Amazon ECS docs](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/task-cpu-memory-error.html) for valid memory values.

<div class="separator"></div>

<a id="count" href="#count" class="field">`count`</a> <span class="type">Integer or Map</span>  
If you specify a number:
```yaml
count: 5
```
The service will set the desired count to 5 and maintain 5 tasks in your service.

Alternatively, you can specify a map for setting up autoscaling:
```yaml
count:
  range: 1-10
  cpu_percentage: 70
  memory_percentage: 80
  requests: 10000
  response_time: 2s
```


<span class="parent-field">count.</span><a id="count-range" href="#count-range" class="field">`range`</a> <span class="type">String</span>  
Specify a minimum and maximum bound for the number of tasks your service should maintain.  

<span class="parent-field">count.</span><a id="count-cpu-percentage" href="#count-cpu-percentage" class="field">`cpu_percentage`</a> <span class="type">Integer</span>  
Scale up or down based on the average CPU your service should maintain.  

<span class="parent-field">count.</span><a id="count-memory-percentage" href="#count-memory-percentage" class="field">`memory_percentage`</a> <span class="type">Integer</span>  
Scale up or down based on the average memory your service should maintain.  

<span class="parent-field">count.</span><a id="requests" href="#count-requests" class="field">`requests`</a> <span class="type">Integer</span>  
Scale up or down based on the request count handled per tasks.

<span class="parent-field">count.</span><a id="response-time" href="#count-response-time" class="field">`response_time`</a> <span class="type">Duration</span>  
Scale up or down based on the service average response time.

<div class="separator"></div>

<a id="variables" href="#variables" class="field">`variables`</a> <span class="type">Map</span>   
Key-value pairs that represent environment variables that will be passed to your service. Copilot will include a number of environment variables by default for you.

<div class="separator"></div>

<a id="secrets" href="#secrets" class="field">`secrets`</a> <span class="type">Map</span>   
Key-value pairs that represent secret values from [AWS Systems Manager Parameter Store](https://docs.aws.amazon.com/systems-manager/latest/userguide/systems-manager-parameter-store.html) that will be securely passed to your service as environment variables.

<div class="separator"></div>

<a id="storage" href="#storage" class="field">`storage`</a> <span class="type">Map</span>  
The Storage section lets you specify external EFS volumes for your containers and sidecars to mount. This allows you to access persistent storage across regions for data processing or CMS workloads. For more detail, see the [storage](../developing/storage.md) page.

<div class="separator"></div>

<a id="environments" href="#environments" class="field">`environments`</a> <span class="type">Map</span>  
The environment section lets you override any value in your manifest based on the environment you're in. In the example manifest above, we're overriding the count parameter so that we can run 2 copies of our service in our prod environment.
