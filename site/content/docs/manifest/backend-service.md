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
      production:
        count: 2
    ```

<a id="name" href="#name" class="field">`name`</a> <span class="type">String</span>  
The name of your service.   

<div class="separator"></div>

<a id="type" href="#type" class="field">`type`</a> <span class="type">String</span>  
The architecture type for your service. [Backend Services](../concepts/services.md#backend-service) are not reachable from the internet, but can be reached with [service discovery](../developing/service-discovery.md) from your other services.

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
In this case, copilot will use the context directory you specified and convert the key-value pairs under args to --build-arg overrides. The equivalent docker build call will be:  
`$ docker build --file path/to/dockerfile --target build-stage --cache-from image:tag --build-arg key=value context/dir`.

You can omit fields and Copilot will do its best to understand what you mean. For example, if you specify `context` but not `dockerfile`, Copilot will run Docker in the context directory and assume that your Dockerfile is named "Dockerfile." If you specify `dockerfile` but no `context`, Copilot assumes you want to run Docker in the directory that contains `dockerfile`.

All paths are relative to your workspace root.

<span class="parent-field">image.</span><a id="image-location" href="#image-location" class="field">`location`</a> <span class="type">String</span>  
Instead of building a container from a Dockerfile, you can specify an existing image name. Mutually exclusive with [`image.build`](#image-build).    
The `location` field follows the same definition as the [`image` parameter](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/task_definition_parameters.html#container_definition_image) in the Amazon ECS task definition.

<span class="parent-field">image.</span><a id="image-port" href="#image-port" class="field">`port`</a> <span class="type">Integer</span>  
The port exposed in your Dockerfile. Copilot should parse this value for you from your `EXPOSE` instruction.  
If you don't need your Backend Service to accept requests from other services, you can omit this field.

<span class="parent-field">image.</span><a id="image-healthcheck" href="#image-healthcheck" class="field">`healthcheck`</a> <span class="type">Map</span>  
Optional configuration for container health checks.

<span class="parent-field">image.healthcheck.</span><a id="image-healthcheck-cmd" href="#image-healthcheck-cmd" class="field">`command`</a> <span class="type">Array of Strings</span>  
The command to run to determine if the container is healthy.  
The string array can start with `CMD` to execute the command arguments directly, or `CMD-SHELL` to run the command with the container's default shell.

<span class="parent-field">image.healthcheck.</span><a id="image-healthcheck-interval" href="#image-healthcheck-interval" class="field">`interval`</a> <span class="type">Duration</span>  
Time period between health checks, in seconds. Default is 10s.

<span class="parent-field">image.healthcheck.</span><a id="image-healthcheck-retries" href="#image-healthcheck-retries" class="field">`retries`</a> <span class="type">Integer</span>  
Number of times to retry before container is deemed unhealthy. Default is 2.

<span class="parent-field">image.healthcheck.</span><a id="image-healthcheck-timeout" href="#image-healthcheck-timeout" class="field">`timeout`</a> <span class="type">Duration</span>  
How long to wait before considering the health check failed, in seconds. Default is 5s.

<span class="parent-field">image.healthcheck.</span><a id="image-healthcheck-start-period" href="#image-healthcheck-start-period" class="field">`start_period`</a> <span class="type">Duration</span>  
Grace period within which to provide containers time to bootstrap before failed health checks count towards the maximum number of retries. Default is 0s.

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
```


<span class="parent-field">count.</span><a id="count-range" href="#count-range" class="field">`range`</a> <span class="type">String</span>  
Specify a minimum and maximum bound for the number of tasks your service should maintain.  

<span class="parent-field">count.</span><a id="count-cpu-percentage" href="#count-cpu-percentage" class="field">`cpu_percentage`</a> <span class="type">Integer</span>  
Scale up or down based on the average CPU your service should maintain.  

<span class="parent-field">count.</span><a id="count-memory-percentage" href="#count-memory-percentage" class="field">`memory_percentage`</a> <span class="type">Integer</span>  
Scale up or down based on the average memory your service should maintain.  

<div class="separator"></div>

<a id="network" href="#network" class="field">`network`</a> <span class="type">Map</span>    
The `network` section contains parameters for connecting to AWS resources in a VPC.

<span class="parent-field">network.</span><a id="network-vpc" href="#network-vpc" class="field">`vpc`</a> <span class="type">Map</span>  
Subnets and security groups attached to your tasks.

<span class="parent-field">network.vpc.</span><a id="network-vpc-placement" href="#network-vpc-placement" class="field">`placement`</a> <span class="type">String</span>  
Must be one of `'public'` or `'private'`. Defaults to launching your tasks in public subnets.

!!! info inline end
    Launching tasks in `'private'` subnets that need internet connectivity is only supported if you imported a VPC with
    NAT Gateways when running `copilot env init`. See [#1959](https://github.com/aws/copilot-cli/issues/1959) for tracking
    NAT Gateways support in Copilot-generated VPCs.

<span class="parent-field">network.vpc.</span><a id="network-vpc-security-groups" href="#network-vpc-security-groups" class="field">`security_groups`</a> <span class="type">Array of Strings</span>  
Additional security group IDs associated with your tasks. Copilot always includes a security group so containers within your environment
can communicate with each other.

<div class="separator"></div>

<a id="variables" href="#variables" class="field">`variables`</a> <span class="type">Map</span>   
Key-value pairs that represent environment variables that will be passed to your service. Copilot will include a number of environment variables by default for you.

<div class="separator"></div>

<a id="secrets" href="#secrets" class="field">`secrets`</a> <span class="type">Map</span>   
Key-value pairs that represent secret values from [AWS Systems Manager Parameter Store](https://docs.aws.amazon.com/systems-manager/latest/userguide/systems-manager-parameter-store.html) that will be securely passed to your service as environment variables.

<div class="separator"></div>

<a id="storage" href="#storage" class="field">`storage`</a> <span class="type">Map</span>  
The Storage section lets you specify external EFS volumes for your containers and sidecars to mount. This allows you to access persistent storage across regions for data processing or CMS workloads. For more detail, see the [storage](../developing/storage.md) page.

<span class="parent-field">storage.</span><a id="volumes" href="#volumes" class="field">`volumes`</a> <span class="type">Map</span>  
Specify the name and configuration of any EFS volumes you would like to attach. 

<span class="parent-field">volumes.</span><a id="volume" href="#volume" class="field">`volume`</a> <span class="type">Map</span>  
Specify the configuration of a volume. 

<span class="parent-field">volume.</span><a id="path" href="#path" class="field">`path`</a> <span class="type">String</span>  
Specify the location in the container where you would like your volume to be mounted. Must be fewer than 242 characters and must consist only of the characters `a-zA-Z0-9.-_/`. 

<span class="parent-field">volume.</span><a id="read_only" href="#read-only" class="field">`read_only`</a> <span class="type">Bool</span>  
Defines whether the volume is read-only or not. Defaults to true. If false, the container is granted `elasticfilesystem:ClientWrite` permissions to the filesystem and the volume is writable. 

<span class="parent-field">volume.</span><a id="efs" href="#efs" class="field">`efs`</a> <span class="type">Map</span>  
Specify more detailed EFS configuration.

<span class="parent-field">efs.</span><a id="id" href="#id" class="field">`id`</a> <span class="type">String</span>  
The ID of the filesystem you would like to mount. 

<span class="parent-field">efs.</span><a id="auth" href="#auth" class="field">`auth`</a> <span class="type">Map</span>  
Specify advanced authorization configuration for EFS. 

<span class="parent-field">auth.</span><a id="iam" href="#iam" class="field">`iam`</a> <span class="type">Bool</span>  
Default: true. Whether or not to use IAM authorization to determine whether the volume is allowed to connect to EFS. 

<span class="parent-field">auth.</span><a id="access_point_id" href="#access-point-id" class="field">`access_point_id`</a> <span class="type">String</span>  
The ID of the EFS access point to connect to. If using access points, IAM must be enabled and the `root_dir` field must be either empty or `/`. 

<div class="separator"></div>

<a id="environments" href="#environments" class="field">`environments`</a> <span class="type">Map</span>  
The environment section lets you override any value in your manifest based on the environment you're in. In the example manifest above, we're overriding the count parameter so that we can run 2 copies of our service in our prod environment.
