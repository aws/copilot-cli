List of all available properties for a `'Scheduled Job'` manifest. To learn about Copilot jobs, see the [Jobs](../concepts/jobs.en.md) concept page.

???+ note "Sample scheduled job manifest"

    ```yaml
        name: report-generator
        type: Scheduled Job
    
        on:
          schedule: "@daily"
        cpu: 256
        memory: 512
        retries: 3
        timeout: 1h
    
        image:
          build: ./Dockerfile
    
        variables:
          LOG_LEVEL: info
        env_file: log.env
        secrets:
          GITHUB_TOKEN: GITHUB_TOKEN
    
        # You can override any of the values defined above by environment.
        environments:
          prod:
            cpu: 2048
            memory: 4096
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

| Rate         | Identical to          | In human-readable text and `UTC`, it runs ... |
| ------------ | --------------------- | --------------------------------------------- |
| `"@yearly"`  | `"cron(0 * * * ? *)"` | at midnight on January 1st                    |
| `"@monthly"` | `"cron(0 0 1 * ? *)"` | at midnight on the first day of the month     |
| `"@weekly"`  | `"cron(0 0 ? * 1 *)"` | at midnight on Sunday                         |
| `"@daily"`   | `"cron(0 0 * * ? *)"` | at midnight                                   |
| `"@hourly"`  | `"cron(0 * * * ? *)"` | at minute 0                                   |

* `"@every {duration}"` (For example, "1m", "5m")
* `"rate({duration})"` based on CloudWatch's [rate expressions](https://docs.aws.amazon.com/AmazonCloudWatch/latest/events/ScheduledEvents.html#RateExpressions)

Alternatively, you can specify a cron schedule if you'd like to trigger the job at a specific time:

* `"* * * * *"` based on the standard [cron format](https://en.wikipedia.org/wiki/Cron#Overview).
* `"cron({fields})"` based on CloudWatch's [cron expressions](https://docs.aws.amazon.com/AmazonCloudWatch/latest/events/ScheduledEvents.html#CronExpressions) with six fields.

Finally, you can disable the job from triggering by setting the `schedule` field to `none`:
```yaml
on:
  schedule: "none"
```

<div class="separator"></div>

{% include 'image.md' %}

{% include 'image-config.en.md' %}

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

<a id="platform" href="#platform" class="field">`platform`</a> <span class="type">String</span>  
Operating system and architecture (formatted as `[os]/[arch]`) to pass with `docker build --platform`. For example, `linux/arm64` or `windows/x86_64`. The default is `linux/x86_64`.

Override the generated string to build with a different valid `osfamily` or `architecture`. For example, Windows users might change the string
```yaml
platform: windows/x86_64
```
which defaults to `WINDOWS_SERVER_2019_CORE`, using a map:
```yaml
platform:
  osfamily: windows_server_2019_full
  architecture: x86_64
```
```yaml
platform:
  osfamily: windows_server_2019_core
  architecture: x86_64
```
```yaml
platform:
  osfamily: windows_server_2022_core
  architecture: x86_64
```
```yaml
platform:
  osfamily: windows_server_2022_full
  architecture: x86_64
```

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

<span class="parent-field">storage.volumes.</span><a id="volume" href="#volume" class="field">`<volume>`</a> <span class="type">Map</span>  
Specify the configuration of a volume.

<span class="parent-field">storage.volumes.`<volume>`.</span><a id="path" href="#path" class="field">`path`</a> <span class="type">String</span>  
Required. Specify the location in the container where you would like your volume to be mounted. Must be fewer than 242 characters and must consist only of the characters `a-zA-Z0-9.-_/`.

<span class="parent-field">storage.volumes.`<volume>`.</span><a id="read_only" href="#read-only" class="field">`read_only`</a> <span class="type">Boolean</span>  
Optional. Defaults to `true`. Defines whether the volume is read-only or not. If false, the container is granted `elasticfilesystem:ClientWrite` permissions to the filesystem and the volume is writable.

<span class="parent-field">storage.volumes.`<volume>`.</span><a id="efs" href="#efs" class="field">`efs`</a> <span class="type">Map</span>  
Specify more detailed EFS configuration.

<span class="parent-field">storage.volumes.`<volume>`.efs.</span><a id="id" href="#id" class="field">`id`</a> <span class="type">String</span>  
Required. The ID of the filesystem you would like to mount.

<span class="parent-field">storage.volumes.`<volume>`.efs.</span><a id="root_dir" href="#root-dir" class="field">`root_dir`</a> <span class="type">String</span>  
Optional. Defaults to `/`. Specify the location in the EFS filesystem you would like to use as the root of your volume. Must be fewer than 255 characters and must consist only of the characters `a-zA-Z0-9.-_/`. If using an access point, `root_dir` must be either empty or `/` and `auth.iam` must be `true`.

<span class="parent-field">storage.volumes.`<volume>`.efs.</span><a id="auth" href="#auth" class="field">`auth`</a> <span class="type">Map</span>  
Specify advanced authorization configuration for EFS.

<span class="parent-field">storage.volumes.`<volume>`.efs.auth.</span><a id="iam" href="#iam" class="field">`iam`</a> <span class="type">Boolean</span>  
Optional. Defaults to `true`. Whether or not to use IAM authorization to determine whether the volume is allowed to connect to EFS.

<span class="parent-field">storage.volumes.`<volume>`.efs.auth.</span><a id="access_point_id" href="#access-point-id" class="field">`access_point_id`</a> <span class="type">String</span>  
Optional. Defaults to `""`. The ID of the EFS access point to connect to. If using an access point, `root_dir` must be either empty or `/` and `auth.iam` must be `true`.

<div class="separator"></div>

<a id="logging" href="#logging" class="field">`logging`</a> <span class="type">Map</span>  
The logging section contains log configuration parameters for your container's [FireLens](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/using_firelens.html) log driver (see examples [here](../developing/sidecars.en.md#sidecar-patterns)).

<span class="parent-field">logging.</span><a id="logging-image" href="#logging-image" class="field">`image`</a> <span class="type">Map</span>  
Optional. The Fluent Bit image to use. Defaults to `public.ecr.aws/aws-observability/aws-for-fluent-bit:stable`.

<span class="parent-field">logging.</span><a id="logging-destination" href="#logging-destination" class="field">`destination`</a> <span class="type">Map</span>  
Optional. The configuration options to send to the FireLens log driver.

<span class="parent-field">logging.</span><a id="logging-enableMetadata" href="#logging-enableMetadata" class="field">`enableMetadata`</a> <span class="type">Map</span>  
Optional. Whether to include ECS metadata in logs. Defaults to `true`.

<span class="parent-field">logging.</span><a id="logging-secretOptions" href="#logging-secretOptions" class="field">`secretOptions`</a> <span class="type">Map</span>  
Optional. The secrets to pass to the log configuration.

<span class="parent-field">logging.</span><a id="logging-configFilePath" href="#logging-configFilePath" class="field">`configFilePath`</a> <span class="type">Map</span>  
Optional. The full config file path in your custom Fluent Bit image.

{% include 'publish.en.md' %}

<div class="separator"></div>

<a id="environments" href="#environments" class="field">`environments`</a> <span class="type">Map</span>  
The environment section lets you override any value in your manifest based on the environment you're in.
In the example manifest above, we're overriding the CPU parameter so that our production container is more performant.
