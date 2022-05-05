<div class="separator"></div>

<a id="deployment" href="#deployment" class="field">`deployment`</a> <span class="type">Map</span>  
The deployment section contains parameters to control how many tasks run during the deployment and the ordering of stopping and starting tasks.

<span class="parent-field">deployment.</span><a id="deployment-rolling" href="#deployment-rolling" class="field">`rolling`</a> <span class="type">String</span>  
Rolling deployment strategy. Valid values are

- `"default"`: Creates new tasks as many as the desired count with the updated task definition, before stopping the old tasks. Under the hood, this translates to setting the [`minimumHealthyPercent`](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/service_definition_parameters.html#minimumHealthyPercent) to 100 and [`maximumPercent`](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/service_definition_parameters.html#maximumPercent) to 200.
- `"recreate"`: Stop all running tasks and then spin up new tasks. Under the hood, this translates to setting the [`minimumHealthyPercent`](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/service_definition_parameters.html#minimumHealthyPercent) to 0 and [`maximumPercent`](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/service_definition_parameters.html#maximumPercent) to 100.
