
<a id="port" href="#port" class="field">`port`</a> <span class="type">Integer</span>  
Port of the container to expose (optional).

<a id="image" href="#image" class="field">`image`</a> <span class="type">String or Map</span>  
Image URL for the sidecar container (required).

{% include 'image-config.en.md' %}

<a id="essential" href="#essential" class="field">`essential`</a> <span class="type">Bool</span>  
Whether the sidecar container is an essential container (optional, default true).

<a id="credentialsParameter" href="#credentialsParameter" class="field">`credentialsParameter`</a> <span class="type">String</span>  
ARN of the secret containing the private repository credentials (optional).

<a id="variables" href="#variables" class="field">`variables`</a> <span class="type">Map</span>  
Environment variables for the sidecar container (optional)

<a id="secrets" href="#secrets" class="field">`secrets`</a> <span class="type">Map</span>  
Secrets to expose to the sidecar container (optional)

<a id="envFile" href="#envFile" class="field">`env_file`</a> <span class="type">String</span>  
The path to a file from the root of your workspace containing the environment variables to pass to the sidecar container. For more information about the environment variable file, see [Considerations for specifying environment variable files](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/taskdef-envfiles.html#taskdef-envfiles-considerations).


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
