<div class="separator"></div>

<a id="publish" href="#publish" class="field">`publish`</a> <span class="type">Map</span>  
The `publish` section allows services to publish messages to one or more SNS topics.

```yaml
publish:
  topics:
    - name: orderEvents
```

In the example above, this manifest declares an SNS topic named `orderEvents` that other worker services deployed to the Copilot environment can subscribe to. An environment variable named `COPILOT_SNS_TOPIC_ARNS` is injected into your workload as a JSON string.

In JavaScript, you could write:
```js
const {orderEvents} = JSON.parse(process.env.COPILOT_SNS_TOPIC_ARNS)
```
For more details, see the [pub/sub](../developing/publish-subscribe.en.md) page.

<span class="parent-field">publish.</span><a id="publish-topics" href="#publish-topics" class="field">`topics`</a> <span class="type">Array of topics</span>  
List of [`topic`](#publish-topics-topic) objects.

<span class="parent-field">publish.topics.</span><a id="publish-topics-topic" href="#publish-topics-topic" class="field">`topic`</a> <span class="type">Map</span>  
Holds configuration for a single SNS topic.

<span class="parent-field">topic.</span><a id="topic-name" href="#topic-name" class="field">`name`</a> <span class="type">String</span>  
Required. The name of the SNS topic. Must contain only upper and lowercase letters, numbers, hyphens, and underscores.
