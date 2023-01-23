List of all available properties for a `'Request-Driven Web Service'` manifest.

???+ note "Sample AWS App Runner manifests"

    === "Public"

        ```yaml
        # Deploys a web service accessible at https://web.example.com.
        name: frontend
        type: Request-Driven Web Service

        http:
          healthcheck: '/_healthcheck'
          alias: web.example.com

        image:
          build: ./frontend/Dockerfile
          port: 80
        cpu: 1024
        memory: 2048

        variables:
          LOG_LEVEL: info
        tags:
          owner: frontend
        observability:
          tracing: awsxray
        secrets:
          GITHUB_TOKEN: GITHUB_TOKEN
          DB_SECRET:
            secretsmanager: '${COPILOT_APPLICATION_NAME}/${COPILOT_ENVIRONMENT_NAME}/mysql'

        environments:
          test:
            variables:
              LOG_LEVEL: debug
        ```

    === "Connected to the environment VPC"

        ```yaml
        # All egress traffic is routed though the environment VPC.
        name: frontend
        type: Request-Driven Web Service

        image:
          build: ./frontend/Dockerfile
          port: 8080
        cpu: 1024
        memory: 2048

        network:
          vpc:
            placement: private
        ```

    === "Event-driven"

        ```yaml
        # See https://aws.github.io/copilot-cli/docs/developing/publish-subscribe/
        name: refunds
        type: Request-Driven Web Service

        image:
          build: ./refunds/Dockerfile
          port: 8080

        http:
          alias: refunds.example.com
        cpu: 1024
        memory: 2048

        publish:
          topics:
            - name: 'refunds'
            - name: 'orders'
              fifo: true
        ```

<a id="name" href="#name" class="field">`name`</a> <span class="type">String</span>  
The name of your service.

<div class="separator"></div>

<a id="type" href="#type" class="field">`type`</a> <span class="type">String</span>  
The architecture type for your service. A [Request-Driven Web Service](../concepts/services.en.md#request-driven-web-service) is an internet-facing service that is deployed on AWS App Runner.

<div class="separator"></div>

<a id="http" href="#http" class="field">`http`</a> <span class="type">Map</span>  
The http section contains parameters related to the managed load balancer.

<span class="parent-field">http.</span><a id="http-private" href="#http-private" class="field">`private`</a> <span class="type">Bool or Map</span>
Restrict incoming traffic to only your environment. Defaults to false.

<span class="parent-field">http.private</span><a id="http-private-endpoint" href="#http-private-endpoint" class="field">`endpoint`</a> <span class="type">String</span>
The ID of an existing VPC Endpoint to App Runner.
```yaml
http:
  private:
    endpoint: vpce-12345
```

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
Assign a friendly domain name to your request-driven web services. To learn more see [`developing/domain`](../developing/domain.en.md##request-driven-web-service).

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
    Only public images stored in [Amazon ECR Public](https://docs.aws.amazon.com/AmazonECR/latest/public/public-repositories.html) are available with AWS App Runner.

<span class="parent-field">image.</span><a id="image-port" href="#image-port" class="field">`port`</a> <span class="type">Integer</span>  
The port exposed in your Dockerfile. Copilot should parse this value for you from your `EXPOSE` instruction.

<div class="separator"></div>  

<a id="cpu" href="#cpu" class="field">`cpu`</a> <span class="type">Integer</span>  
Number of CPU units reserved for each instance of your service. See the [AWS App Runner docs](https://docs.aws.amazon.com/apprunner/latest/api/API_InstanceConfiguration.html#apprunner-Type-InstanceConfiguration-Cpu) for valid CPU values.

<div class="separator"></div>

<a id="memory" href="#memory" class="field">`memory`</a> <span class="type">Integer</span>  
Amount of memory in MiB reserved for each instance of your service. See the [AWS App Runner docs](https://docs.aws.amazon.com/apprunner/latest/api/API_InstanceConfiguration.html#apprunner-Type-InstanceConfiguration-Memory) for valid memory values.

<div class="separator"></div>

<a id="network" href="#network" class="field">`network`</a> <span class="type">Map</span>      
The `network` section contains parameters for connecting the service to AWS resources in the environment's VPC.  
By connecting the service to a VPC, you can use [service discovery](../developing/svc-to-svc-communication.en.md#service-discovery) to communicate with other services
in your environment, or connect to a database in your VPC such as Amazon Aurora with [`storage init`](../commands/storage-init.en.md).

<span class="parent-field">network.</span><a id="network-vpc" href="#network-vpc" class="field">`vpc`</a> <span class="type">Map</span>    
Subnets in the VPC to route egress traffic from the service.

<span class="parent-field">network.vpc.</span><a id="network-vpc-placement" href="#network-vpc-placement" class="field">`placement`</a> <span class="type">String</span>  
The only valid option today is `'private'`. If you prefer the service not to be connected to a VPC, you can remove the `network` field.

When the placement is `'private'`, the App Runner service routes egress traffic through the private subnets of the VPC.  
If you use a Copilot-generated VPC, Copilot will automatically add NAT Gateways to your environment for internet connectivity. (See [pricing](https://aws.amazon.com/vpc/pricing/).)
Alternatively, when running `copilot env init`, you can import an existing VPC with NAT Gateways, or one with VPC endpoints
for isolated workloads. See our [custom environment resources](../developing/custom-environment-resources.en.md) page for more.

{% include 'observability.en.md' %}

<div class="separator"></div>

<a id="command" href="#command" class="field">`command`</a> <span class="type">String</span>  
Optional. Override the default command in the image.

<div class="separator"></div>

<a id="variables" href="#variables" class="field">`variables`</a> <span class="type">Map</span>  
Key-value pairs that represent environment variables that will be passed to your service. Copilot will include a number of environment variables by default for you.

{% include 'secrets.en.md' %}

{% include 'publish.en.md' %}

<div class="separator"></div>

<a id="variables" href="#variables" class="field">`tags`</a> <span class="type">Map</span>  
Key-value pairs representing AWS tags that are passed down to your AWS App Runner resources.

<div class="separator"></div>

<a id="count" href="#count" class="field">`count`</a> <span class="type">String</span>
Specify the name of an existing autoscaling configuration.
```yaml
count: high-availability/3
```

<div class="separator"></div>

<a id="environments" href="#environments" class="field">`environments`</a> <span class="type">Map</span>  
The environment section lets you override any value in your manifest based on the environment you're in. In the example manifest above, we're overriding the `LOG_LEVEL` environment variable in our 'test' environment.
