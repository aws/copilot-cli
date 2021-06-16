List of all available properties for a `'Scheduled Job'` manifest. To learn about Copilot jobs, see the [Jobs](../concepts/jobs.en.md) concept page.

???+ note "Sample manifest for a report generator cronjob"

    ```yaml
    # Your job name will be used in naming your resources like log groups, ECS Tasks, etc.
    name: report-generator
    type: Scheduled Job

    on:
      schedule: @daily
    cpu: 256
    memory: 512
    retries: 3
    timeout: 1h

    image:
      # Path to your service's Dockerfile.
      build: ./Dockerfile

    variables:
      LOG_LEVEL: info
    secrets:
      GITHUB_TOKEN: GITHUB_TOKEN
    ```

<a id="name" href="#name" class="field">`name`</a> <span class="type">String</span>  
The name of your job.

<div class="separator"></div>

<a id="type" href="#type" class="field">`type`</a> <span class="type">String</span>  
The architecture type for your job.
Currently, Copilot only supports the "Scheduled Job" type for tasks that are triggered either on a fixed schedule or periodically.

<div class="separator"></div>

<a id="on" href="#on" class="field">`on`</a> <span class="type">Map</span>  
The configuration for the event that triggers your job.

<span class="parent-field">on.</span><a id="on-schedule" href="#on-schedule" class="field">`schedule`</a> <span class="type">String</span>  
You can specify a rate to periodically trigger your job. Supported rates:

* `"@yearly"`
* `"@monthly"`
* `"@weekly"`
* `"@daily"`
* `"@hourly"`
* `"@every {duration}"` (For example, "1m", "5m")
* `"rate({duration})"` based on CloudWatch's [rate expressions](https://docs.aws.amazon.com/AmazonCloudWatch/latest/events/ScheduledEvents.html#RateExpressions)

Alternatively, you can specify a cron schedule if you'd like to trigger the job at a specific time:

* `"* * * * *"` based on the standard [cron format](https://en.wikipedia.org/wiki/Cron#Overview).
* `"cron({fields})"` based on CloudWatch's [cron expressions](https://docs.aws.amazon.com/AmazonCloudWatch/latest/events/ScheduledEvents.html#CronExpressions) with six fields.

<div class="separator"></div>

<a id="image" href="#image" class="field">`image`</a> <span class="type">Map</span>  
The image section contains parameters relating to the Docker build configuration.

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

<span class="parent-field">image.</span><a id="image-labels" href="#image-labels" class="field">`labels`</a><span class="type">Map</span>  
An optional key/value map of [Docker labels](https://docs.docker.com/config/labels-custom-metadata/) to add to the container.

<span class="parent-field">image.</span><a id="image-depends-on" href="#image-depends-on" class="field">`depends_on`</a><span class="type">Map</span>  
An optional key/value map of [Container Dependencies](https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_ContainerDependency.html) to add to the container.

<div class="separator"></div>  

<a id="entrypoint" href="#entrypoint" class="field">`entrypoint`</a> <span class="type">String or Array of Strings</span>  
Override the default entrypoint in the image.
```yaml
# String version.
entrypoint: "/bin/entrypoint --p1 --p2"
# Alteratively, as an array of strings.
entrypoint: ["/bin/entrypoint", "--p1", "--p2"]
```

<div class="separator"></div>

<a id="command" href="#command" class="field">`command`</a> <span class="type">String or Array of Strings</span>  
Override the default command in the image.

```yaml
# String version.
command: ps au
# Alteratively, as an array of strings.
command: ["ps", "au"]
```

<div class="separator"></div>

<a id="cpu" href="#cpu" class="field">`cpu`</a> <span class="type">Integer</span>  
Number of CPU units for the task. See the [Amazon ECS docs](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/task-cpu-memory-error.html) for valid CPU values.

<div class="separator"></div>

<a id="memory" href="#memory" class="field">`memory`</a> <span class="type">Integer</span>  
Amount of memory in MiB used by the task. See the [Amazon ECS docs](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/task-cpu-memory-error.html) for valid memory values.

<div class="separator"></div>

<a id="retries" href="#retries" class="field">`retries`</a> <span class="type">Integer</span>  
The number of times to retry the job before failing.

<div class="separator"></div>

<a id="timeout" href="#timeout" class="field">`timeout`</a> <span class="type">Duration</span>  
How long the job should run before it aborts and fails. You can use the units: `h`, `m`, or `s`.

<div class="separator"></div>

<a id="network" href="#network" class="field">`network`</a> <span class="type">Map</span>  
The `network` section contains parameters for connecting to AWS resources in a VPC.

<span class="parent-field">network.</span><a id="network-vpc" href="#network-vpc" class="field">`vpc`</a> <span class="type">Map</span>  
Subnets and security groups attached to your tasks.

<span class="parent-field">network.vpc.</span><a id="network-vpc-placement" href="#network-vpc-placement" class="field">`placement`</a> <span class="type">String</span>    
Must be one of `'public'` or `'private'`. Defaults to launching your tasks in public subnets.

!!! info
    If you launch tasks in `'private'` subnets and use a Copilot-generated VPC, Copilot will automatically add NAT Gateways to your environment for internet connectivity. (See [pricing](https://aws.amazon.com/vpc/pricing/).) Alternatively, when running `copilot env init`, you can import an existing VPC with NAT Gateways, or one with VPC endpoints for isolated workloads. See our [custom environment resources](../developing/custom-environment-resources.en.md) page for more.

<span class="parent-field">network.vpc.</span><a id="network-vpc-security-groups" href="#network-vpc-security-groups" class="field">`security_groups`</a> <span class="type">Array of Strings</span>  
Additional security group IDs associated with your tasks. Copilot always includes a security group so containers within your environment
can communicate with each other.

<div class="separator"></div>

<a id="variables" href="#variables" class="field">`variables`</a> <span class="type">Map</span>  
Key-value pairs that represent environment variables that will be passed to your job. Copilot will include a number of environment variables by default for you.

<div class="separator"></div>

<a id="secrets" href="#secrets" class="field">`secrets`</a> <span class="type">Map</span>  
Key-value pairs that represent secret values from [AWS Systems Manager Parameter Store](https://docs.aws.amazon.com/systems-manager/latest/userguide/systems-manager-parameter-store.html) that will be securely passed to your job as environment variables.

<div class="separator"></div>

<a id="storage" href="#storage" class="field">`storage`</a> <span class="type">Map</span>  
The Storage section lets you specify external EFS volumes for your containers and sidecars to mount. This allows you to access persistent storage across regions for data processing or CMS workloads. For more detail, see the [storage](../developing/storage.en.md) page.

<span class="parent-field">storage.</span><a id="volumes" href="#volumes" class="field">`volumes`</a> <span class="type">Map</span>  
Specify the name and configuration of any EFS volumes you would like to attach. The `volumes` field is specified as a map of the form:
```yaml
volumes:
  <volume name>:
    path: "/etc/mountpath"
    efs:
      ...
```

<span class="parent-field">storage.volumes.</span><a id="volume" href="#volume" class="field">`volume`</a> <span class="type">Map</span>  
Specify the configuration of a volume.

<span class="parent-field">volume.</span><a id="path" href="#path" class="field">`path`</a> <span class="type">String</span>  
Required. Specify the location in the container where you would like your volume to be mounted. Must be fewer than 242 characters and must consist only of the characters `a-zA-Z0-9.-_/`.

<span class="parent-field">volume.</span><a id="read_only" href="#read-only" class="field">`read_only`</a> <span class="type">Boolean</span>  
Optional. Defaults to `true`. Defines whether the volume is read-only or not. If false, the container is granted `elasticfilesystem:ClientWrite` permissions to the filesystem and the volume is writable.

<span class="parent-field">volume.</span><a id="efs" href="#efs" class="field">`efs`</a> <span class="type">Map</span>  
Specify more detailed EFS configuration.

<span class="parent-field">volume.efs.</span><a id="id" href="#id" class="field">`id`</a> <span class="type">String</span>  
Required. The ID of the filesystem you would like to mount.

<span class="parent-field">volume.efs.</span><a id="root_dir" href="#root-dir" class="field">`root_dir`</a> <span class="type">String</span>  
Optional. Defaults to `/`. Specify the location in the EFS filesystem you would like to use as the root of your volume. Must be fewer than 255 characters and must consist only of the characters `a-zA-Z0-9.-_/`. If using an access point, `root_dir` must be either empty or `/` and `auth.iam` must be `true`.

<span class="parent-field">volume.efs.</span><a id="auth" href="#auth" class="field">`auth`</a> <span class="type">Map</span>  
Specify advanced authorization configuration for EFS.

<span class="parent-field">volume.efs.auth.</span><a id="iam" href="#iam" class="field">`iam`</a> <span class="type">Boolean</span>  
Optional. Defaults to `true`. Whether or not to use IAM authorization to determine whether the volume is allowed to connect to EFS.

<span class="parent-field">volume.efs.auth.</span><a id="access_point_id" href="#access-point-id" class="field">`access_point_id`</a> <span class="type">String</span>  
Optional. Defaults to `""`. The ID of the EFS access point to connect to. If using an access point, `root_dir` must be either empty or `/` and `auth.iam` must be `true`.

<a id="environments" href="#environments" class="field">`environments`</a> <span class="type">Map</span>  
The environment section lets you override any value in your manifest based on the environment you're in.
In the example manifest above, we're overriding the CPU parameter so that our production container is more performant.
