List of all available properties for a `'Scheduled Job'` manifest.

???+ note "Sample manifest for a report generator cronjob"

    ```yaml
    # Your job name will be used in naming your resources like log groups, ECS Tasks, etc.
    name: report-generator
    type: Scheduled Job
    
    on:
      schedule: @daily
    cpu: 256
    memory: 512
    retries: 3
    timeout: 1h
    
    image:
      # Path to your service's Dockerfile.
      build: ./Dockerfile
    
    variables:
      LOG_LEVEL: info
    secrets:
      GITHUB_TOKEN: GITHUB_TOKEN
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

* `"@yearly"`
* `"@monthly"`
* `"@weekly"`
* `"@daily"`
* `"@hourly"`
* `"@every {duration}"` (For example, "1m", "5m")
* `"rate({duration})"` based on CloudWatch's [rate expressions](https://docs.aws.amazon.com/AmazonCloudWatch/latest/events/ScheduledEvents.html#RateExpressions)

Alternatively, you can specify a cron schedule if you'd like to trigger the job at a specific time:

* `"* * * * *"` based on the standard [cron format](https://en.wikipedia.org/wiki/Cron#Overview).
* `"cron({fields})"` based on CloudWatch's [cron expressions](https://docs.aws.amazon.com/AmazonCloudWatch/latest/events/ScheduledEvents.html#CronExpressions) with six fields.

<div class="separator"></div>

<a id="image" href="#image" class="field">`image`</a> <span class="type">Map</span>  
The image section contains parameters relating to the Docker build configuration.  

<span class="parent-field">image.</span><a id="image-build" href="#image-build" class="field">`build`</a> <span class="type">String or Map</span>  
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

<div class="separator"></div>

<a id="cpu" href="#cpu" class="field">`cpu`</a> <span class="type">Integer</span>  
Number of CPU units for the task. See the [Amazon ECS docs](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/task-cpu-memory-error.html) for valid CPU values.

<div class="separator"></div>

<a id="memory" href="#memory" class="field">`memory`</a> <span class="type">Integer</span>  
Amount of memory in MiB used by the task. See the [Amazon ECS docs](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/task-cpu-memory-error.html) for valid memory values.

<div class="separator"></div>

<a id="retries" href="#retries" class="field">`retries`</a> <span class="type">Integer</span>  
The number of times to retry the job before failing.

<div class="separator"></div>

<a id="timeout" href="#timeout" class="field">`timeout`</a> <span class="type">Duration</span>  
How long the job should run before it aborts and fails. You can use the units: `h`, `m`, or `s`.

<div class="separator"></div>

<a id="variables" href="#variables" class="field">`variables`</a> <span class="type">Map</span>   
Key-value pairs that represent environment variables that will be passed to your job. Copilot will include a number of environment variables by default for you.

<div class="separator"></div>

<a id="secrets" href="#secrets" class="field">`secrets`</a> <span class="type">Map</span>   
Key-value pairs that represent secret values from [AWS Systems Manager Parameter Store](https://docs.aws.amazon.com/systems-manager/latest/userguide/systems-manager-parameter-store.html) that will be securely passed to your job as environment variables. 

<div class="separator"></div>

<a id="environments" href="#environments" class="field">`environments`</a> <span class="type">Map</span>  
The environment section lets you override any value in your manifest based on the environment you're in. 
In the example manifest above, we're overriding the CPU parameter so that our production container is more performant.
