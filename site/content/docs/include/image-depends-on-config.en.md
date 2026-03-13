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
