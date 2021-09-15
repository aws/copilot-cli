<div class="separator"></div>

<a id="publish" href="#publish" class="field">`publish`</a> <span class="type">Map</span>  
The `publish` section allows services to publish messages to one or more SNS topics.

```yaml
publish:
  topics:
    - name: order-events
```

In the example above, this manifest declares an SNS topic named `order-events` that other worker services which are deployed to the Copilot environment can subscribe to.

<span class="parent-field">publish.</span><a id="publish-topics" href="#publish-topics" class="field">`topics`</a> <span class="type">Array of topics</span>  
List of [`topic`](#publish-topics-topic) objects.

<span class="parent-field">publish.topics.</span><a id="publish-topics-topic" href="#publish-topics-topic" class="field">`topic`</a> <span class="type">Map</span>  
Holds configuration for a single SNS topic.

<span class="parent-field">topic.</span><a id="topic-name" href="#topic-name" class="field">`name`</a> <span class="type">String</span>  
Required. The name of the SNS topic. Must contain only upper and lowercase letters, numbers, hyphens, and underscores.
