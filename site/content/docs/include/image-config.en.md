<span class="parent-field">image.</span><a id="image-build" href="#image-build" class="field">`build`</a> <span class="type">String or Map</span>  
Build a container from a Dockerfile with optional arguments. Mutually exclusive with [`image.location`](#image-location).

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

!!! warning
    If you are passing in a Windows image, you must add `platform: windows/x86_64` to your manifest.  
    If you are passing in an ARM architecture-based image, you must add `platform: linux/arm64` to your manifest.

<span class="parent-field">image.</span><a id="image-credential" href="#image-credential" class="field">`credentials`</a> <span class="type">String</span>  
An optional credentials ARN for a private repository. The `credentials` field follows the same definition as the [`credentialsParameter`](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/private-auth.html) in the Amazon ECS task definition.

<span class="parent-field">image.</span><a id="image-labels" href="#image-labels" class="field">`labels`</a> <span class="type">Map</span>  
An optional key/value map of [Docker labels](https://docs.docker.com/config/labels-custom-metadata/) to add to the container.

<span class="parent-field">image.</span><a id="image-depends-on" href="#image-depends-on" class="field">`depends_on`</a> <span class="type">Map</span>  
An optional key/value map of [Container Dependencies](https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_ContainerDependency.html) to add to the container. The key of the map is a container name and the value is the condition to depend on. Valid conditions are: `start`, `healthy`, `complete`, and `success`. You cannot specify a `complete` or `success` dependency on an essential container.

For example:
```yaml
image:
  build: ./Dockerfile
  depends_on:
    nginx: start
    startup: success
```
In the above example, the task's main container will only start after the `nginx` sidecar has started and the `startup` container has completed successfully.  
