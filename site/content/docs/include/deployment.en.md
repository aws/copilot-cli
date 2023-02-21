<div class="separator"></div>

<a id="deployment" href="#deployment" class="field">`deployment`</a> <span class="type">Map</span>  
The deployment section contains parameters to control how many tasks run during the deployment and the ordering of stopping and starting tasks.

<span class="parent-field">deployment.</span><a id="deployment-rolling" href="#deployment-rolling" class="field">`rolling`</a> <span class="type">String</span>  
Rolling deployment strategy. Valid values are

- `"default"`: Creates new tasks as many as the desired count with the updated task definition, before stopping the old tasks. Under the hood, this translates to setting the [`minimumHealthyPercent`](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/service_definition_parameters.html#minimumHealthyPercent) to 100 and [`maximumPercent`](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/service_definition_parameters.html#maximumPercent) to 200.
- `"recreate"`: Stop all running tasks and then spin up new tasks. Under the hood, this translates to setting the [`minimumHealthyPercent`](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/service_definition_parameters.html#minimumHealthyPercent) to 0 and [`maximumPercent`](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/service_definition_parameters.html#maximumPercent) to 100.

<span class="parent-field">deployment.</span><a id="deployment-rollback-alarms" href="#deployment-rollback-alarms" class="field">`rollback_alarms`</a> <span class="type">Array of Strings or Map</span>
!!! info
    If an alarm is in "In alarm" state at the beginning of a deployment, Amazon ECS will NOT monitor alarms for the duration of that deployment. For more details, read the docs [here](https://docs.aws.amazon.com/AmazonECS/latest/userguide/deployment-alarm-failure.html).

As a list of strings, the names of existing CloudWatch alarms to associate with your service that may trigger a [deployment rollback](https://docs.aws.amazon.com/AmazonECS/latest/userguide/deployment-alarm-failure.html).
```yaml
deployment:
  rollback_alarms: ["MyAlarm-ELB-4xx", "MyAlarm-ELB-5xx"]
```
As a map, the alarm metric and threshold for Copilot-created alarms. 
Available metrics:
