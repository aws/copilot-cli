<div class="separator"></div>

<a id="logging" href="#logging" class="field">`logging`</a> <span class="type">Map</span>  
The logging section contains log configuration. You can also configure parameters for your container's [FireLens](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/using_firelens.html) log driver in this section (see examples [here](../developing/sidecars.en.md#sidecar-patterns)).

<span class="parent-field">logging.</span><a id="retention" href="#logging-retention" class="field">`retention`</a> <span class="type">Integer</span>  
Optional. The number of days to retain the log events. See [this page](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-logs-loggroup.html#cfn-logs-loggroup-retentionindays) for all accepted values. If omitted, the default is 30.

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

<span class="parent-field">logging.</span><a id="logging-envFile" href="#logging-envFile" class="field">`env_file`</a> <span class="type">String</span>  
The path to a file from the root of your workspace containing the environment variables to pass to the logging sidecar container. For more information about the environment variable file, see [Considerations for specifying environment variable files](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/taskdef-envfiles.html#taskdef-envfiles-considerations).
