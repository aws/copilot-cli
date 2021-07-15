
<a id="port" href="#port" class="field">`port`</a> <span class="type">Integer</span>  
Port of the container to expose (optional).

<a id="image" href="#image" class="field">`image`</a> <span class="type">String</span>  
Image URL for the sidecar container (required).

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
