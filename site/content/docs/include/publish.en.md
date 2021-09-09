<div class="separator"></div>

<a id="publish" href="#publish" class="field">`publish`</a> <span class="type">Map</span>  
The `publish` section allows services to publish messages to one or more SNS topics. By default, no worker services are allowed to subscribe to the created topics. Worker services in the environment can be allowlisted using the `allowed_workers` field on each topic.

```yaml
publish:
  topics:
    - name: order-events
      allowed_workers: [database-worker, receipts-worker]
```

In the example above, this manifest declares an SNS topic named `order-events` and authorizes the worker services named `database-worker` or `receipts-worker` which are deployed to the Copilot environment to subscribe to this topic.

<span class="parent-field">publish.</span><a id="publish-topics" href="#publish-topics" class="field">`topics`</a> <span class="type">Array of topics</span>  
List of [`topic`](#publish-topics-topic) objects.

<span class="parent-field">publish.topics.</span><a id="publish-topics-topic" href="#publish-topics-topic" class="field">`topic`</a> <span class="type">Map</span>  
Holds naming information and permissions for a single SNS topic.

<span class="parent-field">topic.</span><a id="topic-name" href="#topic-name" class="field">`name`</a> <span class="type">String</span>  
Required. The name of the SNS topic. Must contain only upper and lowercase letters, numbers, hyphens, and underscores.

<span class="parent-field">topic.</span><a id="topic-allowed-workers" href="#topic-allowed-workers" class="field">`allowed_workers`</a> <span class="type">Array of strings</span>  
An array containing the names of worker services which are authorized to subscribe to this SNS topic. If this field is not specified, no workers will be able to create subscriptions to this SNS topic.