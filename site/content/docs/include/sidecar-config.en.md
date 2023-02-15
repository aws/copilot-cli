
<a id="port" href="#port" class="field">`port`</a> <span class="type">Integer</span>  
Port of the container to expose (optional).

<a id="image" href="#image" class="field">`image`</a> <span class="type">String or Map</span>  
Image URL for the sidecar container or parameters related to Docker build configuration.

```yaml
sidecars:
  nginx:
    image: 123457839156.dkr.ecr.us-west-2.amazonaws.com/demo/front:nginx-latest
```

<span class="parent-field">image.</span><a id="image-build" href="#image-build" class="field">`build`</a> <span class="type">String or Map</span> 
Build sidecar container images natively from a Dockerfile with optional arguments.

If you specify the path of your Dockerfile as string, Copilot assume that the build context should be the dirname of the string you specify. Workload manifest looks like below
```yaml
sidecars:
  nginx:
    build: path/to/dockerfile
```
<span class="parent-field">image.build.</span><a id="image-build-dockerfile" href="#image-build-dockerfile" class="field">`dockerfile`</a> <span class="type">String</span>
The path to directory of dockerfile.

<span class="parent-field">image.build.</span><a id="image-build-context" href="#image-build-context" class="field">`context`</a> <span class="type">String</span> 
The path of directory that will be passed as build context to docker build command. 

<span class="parent-field">image.build.</span><a id="image-build-target" href="#image-build-target" class="field">`target`</a> <span class="type">String</span> 
Specify a build stage in a multi-stage Dockerfile.

<span class="parent-field">image.build.</span><a id="image-build-cache_from" href="#image-build-cache_from" class="field">`cache_from`</a> <span class="type">Array of Strings</span>
Specify one or more images from which to cache the intermediate image layers during the docker build process.

<span class="parent-field">image.build.</span><a id="image-build-args" href="#image-build-args" class="field">`args`</a> <span class="type">Map</span>
Set Environment Variables or build-time variables that can be set when building a Docker image.

<span class="parent-field">image.</span><a id="image-location" href="#image-location" class="field">`location`</a> <span class="type">String</span> 
You can specify an existing image name instead of building container from a Dockerfile. Mutually exclusive with [`image.build`](#image-build).

<a id="essential" href="#essential" class="field">`essential`</a> <span class="type">Bool</span>  
Whether the sidecar container is an essential container (optional, default true).

<a id="credentialsParameter" href="#credentialsParameter" class="field">`credentialsParameter`</a> <span class="type">String</span>  
ARN of the secret containing the private repository credentials (optional).

<a id="variables" href="#variables" class="field">`variables`</a> <span class="type">Map</span>  
Environment variables for the sidecar container (optional)

<a id="secrets" href="#secrets" class="field">`secrets`</a> <span class="type">Map</span>  
Secrets to expose to the sidecar container (optional)

<a id="mount-points" href="#mount-points" class="field">`mount_points`</a> <span class="type">Array of Maps</span>  
Mount paths for EFS volumes specified at the service level (optional).

<span class="parent-field">mount_points.</span><a id="mount-points-source-volume" href="#mount-points-source-volume" class="field">`source_volume`</a> <span class="type">String</span>  
Source volume to mount in this sidecar (required).

<span class="parent-field">mount_points.</span><a id="mount-points-path" href="#mount-points-path" class="field">`path`</a> <span class="type">String</span>  
The path inside the sidecar container at which to mount the volume (required).

<span class="parent-field">mount_points.</span><a id="mount-points-read-only" href="#mount-points-read-only" class="field">`read_only`</a> <span class="type">Boolean</span>  
Whether to allow the sidecar read-only access to the volume (default true).

<a id="labels" href="#labels" class="field">`labels`</a> <span class="type">Map</span>  
Docker labels to apply to this container (optional).

<a id="depends_on" href="#depends_on" class="field">`depends_on`</a> <span class="type">Map</span>  
Container dependencies to apply to this container (optional).

<a id="entrypoint" href="#entrypoint" class="field">`entrypoint`</a> <span class="type">String or Array of Strings</span>  
Override the default entrypoint in the sidecar.
```yaml
# String version.
entrypoint: "/bin/entrypoint --p1 --p2"
# Alteratively, as an array of strings.
entrypoint: ["/bin/entrypoint", "--p1", "--p2"]
```

<a id="command" href="#command" class="field">`command`</a> <span class="type">String or Array of Strings</span>  
Override the default command in the sidecar.

```yaml
# String version.
command: ps au
# Alteratively, as an array of strings.
command: ["ps", "au"]
```

<a id="healthcheck" href="#healthcheck" class="field">`healthcheck`</a> <span class="type">Map</span>  
Optional configuration for sidecar container health checks.

<span class="parent-field">healthcheck.</span><a id="healthcheck-cmd" href="#healthcheck-cmd" class="field">`command`</a> <span class="type">Array of Strings</span>  
The command to run to determine if the sidecar container is healthy.
The string array can start with `CMD` to execute the command arguments directly, or `CMD-SHELL` to run the command with the container's default shell.

<span class="parent-field">healthcheck.</span><a id="healthcheck-interval" href="#healthcheck-interval" class="field">`interval`</a> <span class="type">Duration</span>  
Time period between health checks, in seconds. Default is 10s.

<span class="parent-field">healthcheck.</span><a id="healthcheck-retries" href="#healthcheck-retries" class="field">`retries`</a> <span class="type">Integer</span>  
Number of times to retry before container is deemed unhealthy. Default is 2.

<span class="parent-field">healthcheck.</span><a id="healthcheck-timeout" href="#healthcheck-timeout" class="field">`timeout`</a> <span class="type">Duration</span>  
How long to wait before considering the health check failed, in seconds. Default is 5s.

<span class="parent-field">healthcheck.</span><a id="healthcheck-start-period" href="#healthcheck-start-period" class="field">`start_period`</a> <span class="type">Duration</span>
Length of grace period for containers to bootstrap before failed health checks count towards the maximum number of retries. Default is 0s.
