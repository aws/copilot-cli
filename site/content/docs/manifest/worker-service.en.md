List of all available properties for a `'Worker Service'` manifest. To learn about Copilot services, see the [Services](../concepts/services.en.md) concept page.

???+ note "Sample worker service manifests"

    === "Single queue"

        ```yaml
        # Collect messages from multiple topics published from other services to a single SQS queue.
        name: cost-analyzer
        type: Worker Service

        image:
          build: ./cost-analyzer/Dockerfile

        subscribe:
          topics:
            - name: products
              service: orders
              filter_policy:
                event:
                - anything-but: order_cancelled
            - name: inventory
              service: warehouse
          queue:
            retention: 96h
            timeout: 30s
            dead_letter:
              tries: 10

        cpu: 256
        memory: 512
        count: 3
        exec: true

        secrets:
          DB:
            secretsmanager: '${COPILOT_APPLICATION_NAME}/${COPILOT_ENVIRONMENT_NAME}/mysql'
        ```

    === "Spot autoscaling"

        ```yaml
        # Burst to Fargate Spot tasks if capacity is available.
        name: cost-analyzer
        type: Worker Service

        image:
          build: ./cost-analyzer/Dockerfile

        subscribe:
          topics:
            - name: products
              service: orders
            - name: inventory
              service: warehouse

        cpu: 256
        memory: 512
        count:
          range:
            min: 1
            max: 10
            spot_from: 2
          queue_delay: # Ensure messages are processed within 10mins assuming a single message takes 250ms to process.
            acceptable_latency: 10m
            msg_processing_time: 250ms
        exec: true
        ```

    === "Separate queues"

        ```yaml
        # Assign individual queues to each topic.
        name: cost-analyzer
        type: Worker Service

        image:
          build: ./cost-analyzer/Dockerfile

        subscribe:
          topics:
            - name: products
              service: orders
              queue:
                retention: 5d
                timeout: 1h
                dead_letter:
                  tries: 3
            - name: inventory
              service: warehouse
              queue:
                retention: 1d
                timeout: 5m
        count: 1
        ```

<a id="name" href="#name" class="field">`name`</a> <span class="type">String</span>  
The name of your service.

<div class="separator"></div>

<a id="type" href="#type" class="field">`type`</a> <span class="type">String</span>  
The architecture type for your service. [Worker Services](../concepts/services.en.md#worker-service) are not reachable from the internet or elsewhere in the VPC. They are designed to pull messages from their associated SQS queues, which are populated by their subscriptions to SNS topics created by other Copilot services' `publish` fields.

<div class="separator"></div>

<a id="subscribe" href="#subscribe" class="field">`subscribe`</a> <span class="type">Map</span>  
The `subscribe` section allows worker services to create subscriptions to the SNS topics exposed by other Copilot services in the same application and environment. Each topic can define its own SQS queue, but by default all topics are subscribed to the worker service's default queue.

The URI of the default queue will be injected into the container as an environment variable, `COPILOT_QUEUE_URI`. 

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

<span class="parent-field">subscribe.queue.</span><a id="subscribe-queue-fifo" href="#subscribe-queue-fifo" class="field">`fifo`</a> <span class="type">Boolean or Map</span>  
Enable FIFO (first in, first out) ordering on your SQS queue to handle scenarios where the order of operations and events is critical, or where duplicates can't be tolerated.

```yaml
subscribe:
  topics:
    - name: events
      service: api
    - name: events
      service: fe
  queue: # Messages from both FIFO SNS Topics go to the shared FIFO SQS Queue.
    fifo: true
```
When the queue is enabled with FIFO capabilities, Copilot requires that the source SNS topics [are also FIFO](../include/publish.en.md#publish-topics-topic-fifo).

Alternatively, you can also specify advanced SQS FIFO queue configurations:
```yaml
subscribe:
  topics:
    - name: events
      service: api
      queue: # Define a topic-specific Standard queue for the api-events topic.
        timeout: 20s
    - name: events
      service: fe
  queue: # By default, messages from all FIFO topics will go to a shared FIFO queue.
    fifo:
      content_based_deduplication: true
      high_throughput: true 
```

<span class="parent-field">subscribe.queue.fifo.</span><a id="subscribe-queue-fifo-content-based-deduplication" href="#subscribe-queue-fifo-content-based-deduplication" class="field">`content_based_deduplication`</a> <span class="type">Boolean</span>  
If the message body is guaranteed to be unique for each published message, you can enable content-based deduplication for the SNS FIFO topic.

<span class="parent-field">subscribe.queue.fifo.</span><a id="subscribe-queue-fifo-deduplication-scope" href="#subscribe-queue-fifo-deduplication-scope" class="field">`deduplication_scope`</a> <span class="type">String</span>  
For high throughput for FIFO queues, specifies whether message deduplication occurs at the message group or queue level. Valid values are "messageGroup" and "queue".

<span class="parent-field">subscribe.queue.fifo.</span><a id="subscribe-queue-fifo-throughput-limit" href="#subscribe-queue-fifo-throughput-limit" class="field">`throughput_limit`</a> <span class="type">String</span>  
For high throughput for FIFO queues, specifies whether the FIFO queue throughput quota applies to the entire queue or per message group. Valid values are "perQueue" and "perMessageGroupId".

<span class="parent-field">subscribe.queue.fifo.</span><a id="subscribe-queue-fifo-high-throughput" href="#subscribe-queue-fifo-high-throughput" class="field">`high_throughput`</a> <span class="type">Boolean</span>  
If enabled, provides higher transactions per second (TPS) for messages in FIFO queues. Mutually exclusive with `deduplication_scope` and `throughput_limit`.


<span class="parent-field">subscribe.queue.dead_letter.</span><a id="subscribe-queue-dead-letter-tries" href="#subscribe-queue-dead-letter-tries" class="field">`tries`</a> <span class="type">Integer</span>  
If specified, creates a dead letter queue and a redrive policy which routes messages to the DLQ after `tries` attempts. That is, if a worker service fails to process a message successfully `tries` times, it will be routed to the DLQ for examination instead of redriven.

<span class="parent-field">subscribe.</span><a id="subscribe-topics" href="#subscribe-topics" class="field">`topics`</a> <span class="type">Array of `topic`s</span>  
Contains information about which SNS topics the worker service should subscribe to.

<span class="parent-field">subscribe.topics.topic</span><a id="topic-name" href="#topic-name" class="field">`name`</a> <span class="type">String</span>  
Required. The name of the SNS topic to subscribe to.

<span class="parent-field">subscribe.topics.topic</span><a id="topic-service" href="#topic-service" class="field">`service`</a> <span class="type">String</span>  
Required. The service this SNS topic is exposed by. Together with the topic name, this uniquely identifies an SNS topic in the copilot environment.

<span class="parent-field">subscribe.topics.topic</span><a id="topic-filter-policy" href="#topic-filter-policy" class="field">`filter_policy`</a> <span class="type">Map</span>  
Optional. Specify a SNS subscription filter policy to evaluate incoming message attributes against the policy.  
The filter policy can be specified in JSON, for example:
```json
filter_policy: {"store":["example_corp"],"event":[{"anything-but":"order_cancelled"}],"customer_interests":["rugby","football","baseball"],"price_usd":[{"numeric":[">=",100]}]}
```
or alternatively as a map in YAML:
```yaml
filter_policy:
  store:
    - example_corp
  event:
    - anything-but: order_cancelled
  customer_interests:
    - rugby
    - football
    - baseball
  price_usd:
    - numeric:
      - ">="
      - 100
```
For additional information on how to write filter policies, see the [SNS documentation](https://docs.aws.amazon.com/sns/latest/dg/sns-subscription-filter-policies.html).

<span class="parent-field">subscribe.topics.topic.</span><a id="topic-queue" href="#topic-queue" class="field">`queue`</a> <span class="type">Boolean or Map</span>  
Optional. Specify SQS queue configuration for the topic. If specified as `true`, the queue will be created  with default configuration. Specify this field as a map for customization of certain attributes for this topic-specific queue.
If you specify one or more topic-specific queues, you can access those queue URIs via the `COPILOT_TOPIC_QUEUE_URIS` variable.
This variable is a JSON map from a unique identifier for the topic-specific queue to its URI.

For example, a worker service with a topic-specific queue for the `orders` topic from the `merchant` service and a FIFO
topic `transactions` from the `merchant` service will have the following JSON structure.

```json
// COPILOT_TOPIC_QUEUE_URIS
{
  "merchantOrdersEventsQueue": "https://sqs.eu-central-1.amazonaws.com/...",
  "merchantTransactionsfifoEventsQueue": "https://sqs.eu-central-1.amazonaws.com/..."
}
```

<span class="parent-field">subscribe.topics.topic.queue.</span><a id="subscribe-topics-topic-queue-fifo" href="#subscribe-topics-topic-queue-fifo" class="field">`fifo`</a> <span class="type">Boolean or Map</span>   
Optional. Specify SQS FIFO queue configuration for the topic. If specified as `true`, the FIFO queue will be created with the default FIFO configuration. 
Specify this field as a map for customization of certain attributes for this topic-specific queue.

{% include 'image.md' %}

{% include 'image-config.en.md' %}

{% include 'image-healthcheck.en.md' %}

{% include 'task-size.en.md' %}

{% include 'platform.en.md' %}

<div class="separator"></div>

<a id="count" href="#count" class="field">`count`</a> <span class="type">Integer or Map</span>
The number of tasks that your service should maintain.

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
  memory_percentage:
    value: 80
    cooldown:
      in: 80s
      out: 160s
  queue_delay:
    acceptable_latency: 10m
    msg_processing_time: 250ms
    cooldown:
      in: 30s
      out: 60s
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

<span class="parent-field">count.range.</span><a id="count-range-min" href="#count-range-min" class="field">`min`</a> <span class="type">Integer</span>
The minimum desired count for your service using autoscaling.

<span class="parent-field">count.range.</span><a id="count-range-max" href="#count-range-max" class="field">`max`</a> <span class="type">Integer</span>
The maximum desired count for your service using autoscaling.

<span class="parent-field">count.range.</span><a id="count-range-spot-from" href="#count-range-spot-from" class="field">`spot_from`</a> <span class="type">Integer</span>
The desired count at which you wish to start placing your service using Fargate Spot capacity providers.

<span class="parent-field">count.</span><a id="count-cooldown" href="#count-cooldown" class="field">`cooldown`</a> <span class="type">Map</span>
Cooldown scaling fields that are used as the default cooldown for all autoscaling fields specified.

<span class="parent-field">count.cooldown.</span><a id="count-cooldown-in" href="#count-cooldown-in" class="field">`in`</a> <span class="type">Duration</span>
The cooldown time for autoscaling fields to scale up the service.

<span class="parent-field">count.cooldown.</span><a id="count-cooldown-out" href="#count-cooldown-out" class="field">`out`</a> <span class="type">Duration</span>
The cooldown time for autoscaling fields to scale down the service.

The following options `cpu_percentage` and `memory_percentage` are autoscaling fields for `count` which can be defined either as the value of the field, or as a Map containing advanced information about the field's `value` and `cooldown`:
```yaml
value: 50
cooldown:
  in: 30s
  out: 60s
```
The cooldown specified here will override the default cooldown.

<span class="parent-field">count.</span><a id="count-cpu-percentage" href="#count-cpu-percentage" class="field">`cpu_percentage`</a> <span class="type">Integer or Map</span>
Scale up or down based on the average CPU your service should maintain.

<span class="parent-field">count.</span><a id="count-memory-percentage" href="#count-memory-percentage" class="field">`memory_percentage`</a> <span class="type">Integer or Map</span>
Scale up or down based on the average memory your service should maintain.

<span class="parent-field">count.</span><a id="count-queue-delay" href="#count-queue-delay" class="field">`queue_delay`</a> <span class="type">Map</span>
Scale up or down to maintain an acceptable queue latency by tracking against the acceptable backlog per task.  
The acceptable backlog per task is calculated by dividing `acceptable_latency` by `msg_processing_time`. For example, if you can tolerate consuming a message within 10 minutes
of its arrival and it takes your task on average 250 milliseconds to process a message, then `acceptableBacklogPerTask = 10 * 60 / 0.25 = 2400`. Therefore, each task can hold up to
2,400 messages.   
A target tracking policy is set up on your behalf to ensure your service scales up and down to maintain <= 2400 messages per task. To learn more see [docs](https://docs.aws.amazon.com/autoscaling/ec2/userguide/as-using-sqs-queue.html).

<span class="parent-field">count.queue_delay.</span><a id="count-queue-delay-acceptable-latency" href="#count-queue-delay-acceptable-latency" class="field">`acceptable_latency`</a> <span class="type">Duration</span>
The acceptable amount of time that a message can sit in the queue. For example, `"45s"`, `"5m"`, `10h`.

<span class="parent-field">count.queue_delay.</span><a id="count-queue-delay-msg-processing-time" href="#count-queue-delay-msg-processing-time" class="field">`msg_processing_time`</a> <span class="type">Duration</span>
The average amount of time it takes to process an SQS message. For example, `"250ms"`, `"1s"`.

<span class="parent-field">count.queue_delay.</span><a id="count-queue-delay-cooldown" href="#count-queue-delay-cooldown" class="field">`cooldown`</a> <span class="type">Map</span>
Scale up and down cooldown fields for queue delay autoscaling.

{% include 'exec.en.md' %}

{% include 'deployment.en.md' %}
```yaml 
deployment:
  rollback_alarms:
    cpu_utilization: 70    // Percentage value at or above which alarm is triggered.
    memory_utilization: 50 // Percentage value at or above which alarm is triggered.
    messages_delayed: 5    // Number of delayed messages in the queue at or above which alarm is triggered. 
```

{% include 'entrypoint.en.md' %}

{% include 'command.en.md' %}

{% include 'network.en.md' %}

{% include 'envvars.en.md' %}

{% include 'secrets.en.md' %}

{% include 'storage.en.md' %}

{% include 'publish.en.md' %}

{% include 'logging.en.md' %}

{% include 'observability.en.md' %}

{% include 'taskdef-overrides.en.md' %}

{% include 'environments.en.md' %}
