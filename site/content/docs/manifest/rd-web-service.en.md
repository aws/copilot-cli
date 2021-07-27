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
      alias: web.example.com

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
The http section contains parameters related to the managed load balancer.

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

<span class="parent-field">http.healthcheck.</span><a id="http-healthcheck-path" href="#http-healthcheck-path" class="field">`path`</a> <span class="type">String</span>  
The destination that the health check requests are sent to.

<span class="parent-field">http.healthcheck.</span><a id="http-healthcheck-healthy-threshold" href="#http-healthcheck-healthy-threshold" class="field">`healthy_threshold`</a> <span class="type">Integer</span>  
The number of consecutive health check successes required before considering an unhealthy target healthy. The default is 3. Range: 1-20.

<span class="parent-field">http.healthcheck.</span><a id="http-healthcheck-unhealthy-threshold" href="#http-healthcheck-unhealthy-threshold" class="field">`unhealthy_threshold`</a> <span class="type">Integer</span>  
The number of consecutive health check failures required before considering a target unhealthy. The default is 3. Range: 1-20.

<span class="parent-field">http.healthcheck.</span><a id="http-healthcheck-interval" href="#http-healthcheck-interval" class="field">`interval`</a> <span class="type">Duration</span>  
The approximate amount of time, in seconds, between health checks of an individual target. The default is 5s. Range: 1s–20s.

<span class="parent-field">http.healthcheck.</span><a id="http-healthcheck-timeout" href="#http-healthcheck-timeout" class="field">`timeout`</a> <span class="type">Duration</span>  
The amount of time, in seconds, during which no response from a target means a failed health check. The default is 2s. Range 1s-20s.

<span class="parent-field">http.</span><a id="http-alias" href="#http-alias" class="field">`alias`</a> <span class="type">String</span>
HTTPS custom domain for your service.

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

!!! note
    Only public images stored in [Amazon ECR Public](https://docs.aws.amazon.com/AmazonECR/latest/public/public-repositories.html) is available with AWS App Runner.

<span class="parent-field">image.</span><a id="image-port" href="#image-port" class="field">`port`</a> <span class="type">Integer</span>  
The port exposed in your Dockerfile. Copilot should parse this value for you from your `EXPOSE` instruction.

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

