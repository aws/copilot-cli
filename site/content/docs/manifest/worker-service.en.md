List of all available properties for a `'Worker Service'` manifest. To learn about Copilot services, see the [Services](../concepts/services.en.md) concept page.

???+ note "Sample manifest for a worker service"

    ```yaml
    # Your service name will be used in naming your resources like log groups, ECS services, etc.
    name: orders-worker
    type: Worker Service

    image:
      build: ./orders/Dockerfile

    subscribe:
      topics:
        - name: events
          service: api
        - name: events
          service: fe
      queue:
        retention: 96h
        timeout: 30s
        dead_letter:
          tries: 10

    cpu: 256
    memory: 512
    count: 1

    variables:
      LOG_LEVEL: info
    env_file: log.env
    secrets:
      GITHUB_TOKEN: GITHUB_TOKEN

    # You can override any of the values defined above by environment.
    environments:
      production:
        count:
          range:
            min: 1
            max: 50
            spot_from: 26
          queue_delay:
            acceptable_latency: 1m
            msg_processing_time: 250ms
    ```

<a id="name" href="#name" class="field">`name`</a> <span class="type">String</span>
The name of your service.

<div class="separator"></div>

<a id="type" href="#type" class="field">`type`</a> <span class="type">String</span>  
The architecture type for your service. [Worker Services](../concepts/services.en.md#worker-service) are not reachable from the internet or elsewhere in the VPC. They are designed to pull messages from their associated SQS queues, which are populated by their subscriptions to SNS topics created by other Copilot services' `publish` fields.

<div class="separator"></div>

<a id="subscribe" href="#subscribe" class="field">`subscribe`</a> <span class="type">Map</span>
The `subscribe` section allows worker services to create subscriptions to the SNS topics exposed by other Copilot services in the same application and environment. Each topic can define its own SQS queue, but by default all topics are subscribed to the worker service's default queue.

```yaml
subscribe:
  topics:
    - name: events
      service: api
      queue: # Define a topic-specific queue for the api-events topic.
        timeout: 20s
    - name: events
      service: fe
  queue: # By default, messages from all topics will go to a shared queue.
    timeout: 45s
    retention: 96h
    delay: 30s
```

<span class="parent-field">subscribe.</span><a id="subscribe-queue" href="#subscribe-queue" class="field">`queue`</a> <span class="type">Map</span>
By default, a service level queue is always created. `queue` allows customization of certain attributes of that default queue.

<span class="parent-field">subscribe.queue.</span><a id="subscribe-queue-delay" href="#subscribe-queue-delay" class="field">`delay`</a> <span class="type">Duration</span>
The time in seconds for which the delivery of all messages in the queue is delayed. Default 0s. Range 0s-15m.

<span class="parent-field">subscribe.queue.</span><a id="subscribe-queue-retention" href="#subscribe-queue-retention" class="field">`retention`</a> <span class="type">Duration</span>
Retention specifies the time a message will remain in the queue before being deleted. Default 4d. Range 60s-336h.

<span class="parent-field">subscribe.queue.</span><a id="subscribe-queue-timeout" href="#subscribe-queue-timeout" class="field">`timeout`</a> <span class="type">Duration</span>
Timeout defines the length of time a message is unavailable after being delivered. Default 30s. Range 0s-12h.

<span class="parent-field">subscribe.queue.dead_letter.</span><a id="subscribe-queue-dead-letter-tries" href="#subscribe-queue-dead-letter-tries" class="field">`tries`</a> <span class="type">Integer</span>
If specified, creates a dead letter queue and a redrive policy which routes messages to the DLQ after `tries` attempts. That is, if a worker service fails to process a message successfully `tries` times, it will be routed to the DLQ for examination instead of redriven.

<span class="parent-field">subscribe.</span><a id="subscribe-topics" href="#subscribe-topics" class="field">`topics`</a> <span class="type">Array of `topic`s</span>
Contains information about which SNS topics the worker service should subscribe to.

<span class="parent-field">topic.</span><a id="topic-name" href="#topic-name" class="field">`name`</a> <span class="type">String</span>
Required. The name of the SNS topic to subscribe to.

<span class="parent-field">topic.</span><a id="topic-service" href="#topic-service" class="field">`service`</a> <span class="type">String</span>
Required. The service this SNS topic is exposed by. Together with the topic name, this uniquely identifies an SNS topic in the copilot environment.

<span class="parent-field">topic.</span><a id="topic-queue" href="#topic-queue" class="field">`queue`</a> <span class="type">Boolean or Map</span>
Optional. Specify SQS queue configuration for the topic. If specified as `true`, the queue will be created  with default configuration. Specify this field as a map for customization of certain attributes for this topic-specific queue.

{% include 'image-config.en.md' %}

{% include 'image-healthcheck.en.md' %}

{% include 'task-size.en.md' %}

{% include 'platform.en.md' %}

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
!!! info
    Fargate Spot is not supported for containers running on ARM architecture.

<div class="separator"></div>

Alternatively, you can specify a map for setting up autoscaling:
```yaml
count:
  range: 1-10
  cpu_percentage: 70
  memory_percentage: 80
  queue_delay:
    acceptable_latency: 10m
    msg_processing_time: 250ms
```

<span class="parent-field">count.</span><a id="count-range" href="#count-range" class="field">`range`</a> <span class="type">String or Map</span>  
You can specify a minimum and maximum bound for the number of tasks your service should maintain, based on the values you specify for the metrics.
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

<span class="parent-field">count.</span><a id="count-queue-delay" href="#count-queue-delay" class="field">`queue_delay`</a> <span class="type">Integer</span>   
Scale up or down to maintain an acceptable queue latency by tracking against the acceptable backlog per task.  
The acceptable backlog per task is calculated by dividing `acceptable_latency` by `msg_processing_time`. For example, if you can tolerate consuming a message within 10 minutes
of its arrival and it takes your task on average 250 milliseconds to process a message, then `acceptableBacklogPerTask = 10 * 60 / 0.25 = 2400`. Therefore, each task can hold up to
2,400 messages.   
A target tracking policy is set up on your behalf to ensure your service scales up and down to maintain <= 2400 messages per task. To learn more see [docs](https://docs.aws.amazon.com/autoscaling/ec2/userguide/as-using-sqs-queue.html).

<span class="parent-field">count.queue_delay.</span><a id="count-queue-delay-acceptable-latency" href="#count-queue-delay-acceptable-latency" class="field">`acceptable_latency`</a> <span class="type">Duration</span>   
The acceptable amount of time that a message can sit in the queue. For example, `"45s"`, `"5m"`, `10h`.

<span class="parent-field">count.queue_delay.</span><a id="count-queue-delay-msg-processing-time" href="#count-queue-delay-msg-processing-time" class="field">`msg_processing_time`</a> <span class="type">Duration</span>   
The average amount of time it takes to process an SQS message. For example, `"250ms"`, `"1s"`.

{% include 'exec.en.md' %}

{% include 'entrypoint.en.md' %}

{% include 'command.en.md' %}

{% include 'network.en.md' %}

{% include 'envvars.en.md' %}

{% include 'secrets.en.md' %}

{% include 'storage.en.md' %}

{% include 'publish.en.md' %}

{% include 'logging.en.md' %}

{% include 'taskdef-overrides.en.md' %}

{% include 'environments.en.md' %}
