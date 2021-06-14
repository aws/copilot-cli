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
You can specify a minimum and maximum bound for the number of tasks your service should maintain.
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

<div class="separator"></div>

<a id="exec" href="#exec" class="field">`exec`</a> <span class="type">Boolean</span>  
Enable running commands in your container. The default is `false`. Required for `$ copilot svc exec`.

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
Key-value pairs that represent environment variables that will be passed to your service. Copilot will include a number of environment variables by default for you.

<div class="separator"></div>

<a id="secrets" href="#secrets" class="field">`secrets`</a> <span class="type">Map</span>  
Key-value pairs that represent secret values from [AWS Systems Manager Parameter Store](https://docs.aws.amazon.com/systems-manager/latest/userguide/systems-manager-parameter-store.html) that will be securely passed to your service as environment variables.

<a id="depends-on" href="#depends-on" class="field">`depends_on`</a> <span class="type">Map</span> 
An optional key/value map of [Container Dependencies](https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_ContainerDependency.html) to add to the container.

<div class="separator"></div>  

<a id="storage" href="#storage" class="field">`storage`</a> <span class="type">Map</span>  
The Storage section lets you specify external EFS volumes for your containers and sidecars to mount. This allows you to access persistent storage across availability zones in a region for data processing or CMS workloads. For more detail, see the [storage](../developing/storage.en.md) page. You can also specify extensible ephemeral storage at the task level.

<span class="parent-field">storage.</span><a id="ephemeral" href="#ephemeral" class="field">`ephemeral`</a> <span class="type">Int</span>
Specify how much ephemeral task storage to provision in GiB. The default value and minimum is 20 GiB. The maximum size is 200 GiB. Sizes above 20 GiB incur additional charges.

To create a shared filesystem context between an essential container and a sidecar, you can use an empty volume:
```yaml
storage:
  ephemeral: 100
  volumes:
    scratch:
      path: /var/data
      read_only: false

sidecars:
  mySidecar:
    image: public.ecr.aws/my-image:latest
    mount_points:
      - source_volume: scratch
        path: /var/data
        read_only: false
```
This example will provision 100 GiB of storage to be shared between the sidecar and the task container. This can be useful for large datasets, or for using a sidecar to transfer data from EFS into task storage for workloads with high disk I/O requirements.

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

<span class="parent-field">volume.</span><a id="efs" href="#efs" class="field">`efs`</a> <span class="type">Boolean or Map</span>  
Specify more detailed EFS configuration. If specified as a boolean, or using only the `uid` and `gid` subfields, creates a managed EFS filesystem and dedicated Access Point for this workload.
```yaml
// Simple managed EFS
efs: true

// Managed EFS with custom POSIX info
efs:
  uid: 10000
  gid: 110000
```

<span class="parent-field">volume.efs.</span><a id="id" href="#id" class="field">`id`</a> <span class="type">String</span>  
Required. The ID of the filesystem you would like to mount.

<span class="parent-field">volume.efs.</span><a id="root_dir" href="#root-dir" class="field">`root_dir`</a> <span class="type">String</span>
Optional. Defaults to `/`. Specify the location in the EFS filesystem you would like to use as the root of your volume. Must be fewer than 255 characters and must consist only of the characters `a-zA-Z0-9.-_/`. If using an access point, `root_dir` must be either empty or `/` and `auth.iam` must be `true`.

<span class="parent-field">volume.efs.</span><a id="uid" href="#uid" class="field">`uid`</a> <span class="type">Uint32</span>
Optional. Must be specified with `gid`. Mutually exclusive with `root_dir`, `auth`, and `id`. The POSIX UID to use for the dedicated access point created for the managed EFS filesystem. 

<span class="parent-field">volume.efs.</span><a id="gid" href="#gid" class="field">`gid`</a> <span class="type">Uint32</span>
Optional. Must be specified with `uid`. Mutually exclusive with `root_dir`, `auth`, and `id`. The POSIX GID to use for the dedicated access point created for the managed EFS filesystem. 

<span class="parent-field">volume.efs.</span><a id="auth" href="#auth" class="field">`auth`</a> <span class="type">Map</span>  
Specify advanced authorization configuration for EFS.

<span class="parent-field">volume.efs.auth.</span><a id="iam" href="#iam" class="field">`iam`</a> <span class="type">Boolean</span>  
Optional. Defaults to `true`. Whether or not to use IAM authorization to determine whether the volume is allowed to connect to EFS.

<span class="parent-field">volume.efs.auth.</span><a id="access_point_id" href="#access-point-id" class="field">`access_point_id`</a> <span class="type">String</span>  
Optional. Defaults to `""`. The ID of the EFS access point to connect to. If using an access point, `root_dir` must be either empty or `/` and `auth.iam` must be `true`.

<div class="separator"></div>

<a id="environments" href="#environments" class="field">`environments`</a> <span class="type">Map</span>  
The environment section lets you override any value in your manifest based on the environment you're in. In the example manifest above, we're overriding the count parameter so that we can run 2 copies of our service in our prod environment.
