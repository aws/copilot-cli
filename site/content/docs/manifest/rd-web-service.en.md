List of all available properties for a `'Request-Driven Web Service'` manifest.

???+ note "Sample manifest for a frontend service"

    ```yaml
    # Your service name will be used in naming your resources like log groups, App Runner services, etc.
    name: frontend
    # The "architecture" of the service you're running.
    type: Request-Driven Web Service

    http:
      healthcheck:
        path: '/_healthcheck'
        healthy_threshold: 3
        unhealthy_threshold: 5
        interval: 10s
        timeout: 5s

    # Configuration for your containers and service.
    image:
      build: ./frontend/Dockerfile
      port: 80

    cpu: 1024
    memory: 2048

    variables:
      LOG_LEVEL: info
    
    tags:
      owner: frontend-team

    environments:
      test:
        LOG_LEVEL: debug
    ```

<a id="name" href="#name" class="field">`name`</a> <span class="type">String</span>  
The name of your service.

<div class="separator"></div>

<a id="type" href="#type" class="field">`type`</a> <span class="type">String</span>  
The architecture type for your service. A [Request-Driven Web Service](../concepts/services.en.md#request-driven-web-service) is an internet-facing service that is deployed on AWS App Runner.

<div class="separator"></div>

<a id="http" href="#http" class="field">`http`</a> <span class="type">Map</span>  
The http section contains parameters related to integrating your service with an Application Load Balancer.

<span class="parent-field">http.</span><a id="http-path" href="#http-path" class="field">`path`</a> <span class="type">String</span>  
Requests to this path will be forwarded to your service. Each Request-Driven Web Service should listen on a unique path.

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
The number of consecutive health check successes required before considering an unhealthy target healthy. The default is 3. Range: 1-20.

<span class="parent-field">http.healthcheck.</span><a id="http-healthcheck-unhealthy-threshold" href="#http-healthcheck-unhealthy-threshold" class="field">`unhealthy_threshold`</a> <span class="type">Integer</span>  
The number of consecutive health check failures required before considering a target unhealthy. The default is 3. Range: 1-20.

<span class="parent-field">http.healthcheck.</span><a id="http-healthcheck-interval" href="#http-healthcheck-interval" class="field">`interval`</a> <span class="type">Duration</span>  
The approximate amount of time, in seconds, between health checks of an individual target. The default is 5s. Range: 1s–20s.

<span class="parent-field">http.healthcheck.</span><a id="http-healthcheck-timeout" href="#http-healthcheck-timeout" class="field">`timeout`</a> <span class="type">Duration</span>  
The amount of time, in seconds, during which no response from a target means a failed health check. The default is 2s. Range 1s-20s.

{% include 'image-config.md' %}

<div class="separator"></div>  

<a id="cpu" href="#cpu" class="field">`cpu`</a> <span class="type">Integer</span>  
Number of CPU units reserved for each instance of your service. See the [AWS App Runner docs](https://docs.aws.amazon.com/apprunner/latest/api/API_InstanceConfiguration.html#apprunner-Type-InstanceConfiguration-Cpu) for valid CPU values.

<div class="separator"></div>

<a id="memory" href="#memory" class="field">`memory`</a> <span class="type">Integer</span>  
Amount of memory in MiB reserved for each instance of your service. See the [AWS App Runner docs](https://docs.aws.amazon.com/apprunner/latest/api/API_InstanceConfiguration.html#apprunner-Type-InstanceConfiguration-Memory) for valid memory values.

<div class="separator"></div>

<a id="variables" href="#variables" class="field">`variables`</a> <span class="type">Map</span>  
Key-value pairs that represent environment variables that will be passed to your service. Copilot will include a number of environment variables by default for you.

<div class="separator"></div>

<a id="environments" href="#environments" class="field">`environments`</a> <span class="type">Map</span>  
The environment section lets you override any value in your manifest based on the environment you're in. In the example manifest above, we're overriding the count parameter so that we can run 2 copies of our service in our prod environment.


