List of all available properties for a `'Backend Service'` manifest.

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

{% include 'image-config.md' %}
{% include 'image-healthcheck.md' %}
{% include 'common-svc-fields.md' %}

<div class="separator"></div>

<a id="network" href="#network" class="field">`network`</a> <span class="type">Map</span>      
The `network` section contains parameters for connecting to AWS resources in a VPC.

<span class="parent-field">network.</span><a id="network-vpc" href="#network-vpc" class="field">`vpc`</a> <span class="type">Map</span>    
Subnets and security groups attached to your tasks.

<span class="parent-field">network.vpc.</span><a id="network-vpc-placement" href="#network-vpc-placement" class="field">`placement`</a> <span class="type">String</span>  
Must be one of `'public'`, `'private'`, or `'isolated'`. Defaults to launching your tasks in public subnets.

!!! info
    If you launch tasks in `'private'` subnets and use a Copilot-generated VPC, Copilot will add NAT gateways to your environment. Alternatively, you can import a VPC with NAT gateways when running `copilot env init` for internet connectivity.
    If you launch tasks in `'isolated'` subnets and use a Copilot-generated VPC, Copilot will add VPC endpoints to your environment. Alternatively, you can import a VPC when running `copilot env init` to customize your networking resources.

<span class="parent-field">network.vpc.</span><a id="network-vpc-security-groups" href="#network-vpc-security-groups" class="field">`security_groups`</a> <span class="type">Array of Strings</span>  
Additional security group IDs associated with your tasks. Copilot always includes a security group so containers within your environment
can communicate with each other.

<div class="separator"></div>

<a id="environments" href="#environments" class="field">`environments`</a> <span class="type">Map</span>  
The environment section lets you override any value in your manifest based on the environment you're in. In the example manifest above, we're overriding the count parameter so that we can run 2 copies of our service in our prod environment.