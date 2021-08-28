List of all available properties for a `'Worker Service'` manifest. To learn about Copilot services, see the [Services](../concepts/services.en.md) concept page.

???+ note "Sample manifest for a worker service"

    ```yaml
    # Your service name will be used in naming your resources like log groups, ECS services, etc.
    name: events-worker
    type: Worker Service

    # Your service is reachable at "http://api.${COPILOT_SERVICE_DISCOVERY_ENDPOINT}:8080" but is not public.

    # Configuration for your containers and service.
    image:
      build: ./api/Dockerfile

    cpu: 256
    memory: 512
    count: 1

    subscribe:
      topics:
        - name: events
          service: api
          queue:
            timeout: 10s
            dead_letter:
              tries: 5
        - name: events
          service: fe
      queue:
        retention: 96h
        timeout: 30s
        dead_letter:
          tries: 10

    variables:
      LOG_LEVEL: info
    secrets:
      GITHUB_TOKEN: GITHUB_TOKEN

    # You can override any of the values defined above by environment.
    environments:
      test:
        count:
          spot: 2
      production:
        count: 2
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
  queue:
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




{% include 'image-config.en.md' %}

{% include 'image-healthcheck.en.md' %}

{% include 'common-svc-fields.en.md' %}
