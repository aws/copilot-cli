# Publish/Subscribe Architectures

Copilot [Worker Services](../manifest/worker-service.en.md) take advantage of the `publish` field common to all service and job types to allow customers to easily create publish/subscribe logic for passing messages between services. 

A common pattern in AWS is the combination of SNS and SQS to deliver and process messages. [SNS](https://docs.aws.amazon.com/sns/latest/dg/welcome.html) is a robust message delivery system which can send messages to a variety of subscribed endpoints with guarantees about message delivery. 

[SQS](https://docs.aws.amazon.com/AWSSimpleQueueService/latest/SQSDeveloperGuide/welcome.html) is a message queue to allow asynchronous processing of messages. Queues can be populated by one or more SNS topics or AWS EventBridge event filters.

The combination of these two services effectively decouples the sending and receipt of messages, meaning publishers don't have to care what queues are actually subscribed to their topics, and worker service code doesn't have to care where the messages come from.

## Sending Messages from a Publisher

To allow an existing service to publish messages to SNS, simply set the `publish` field in its manifest.
We suggest using a name for the topic that describes its function.

```yaml
# manifest.yml for api service
name: api
type: Backend Service

publish:
  topics:
    - name: ordersTopic
```

This will create an [SNS topic](https://docs.aws.amazon.com/sns/latest/dg/welcome.html) and set a resource policy on the topic to allow SQS queues in your AWS account to create subscriptions.

Copilot also injects the ARNs of any SNS topics into your container under the environment variable `COPILOT_SNS_TOPIC_ARNS`.
The JSON string is of the format:
```json
{
  "firstTopicName": "arn:aws:sns:us-east-1:123456789012:firstTopic",
  "secondTopicName": "arn:aws:sns:us-east-1:123456789012:secondTopic",
}
```

### Javascript Example
Once the publishing service has been deployed, you can send messages to SNS via the AWS SDK for SNS. 

```javascript
const { SNSClient, PublishCommand } = require("@aws-sdk/client-sns");
const client = new SNSClient({ region: "us-west-2" });
const {ordersTopic} = JSON.parse(process.env.COPILOT_SNS_TOPIC_ARNS);
const out = await client.send(new PublishCommand({
   Message: "hello",
   TopicArn: ordersTopic,
 }));
```

## Subscribing to a topic with a Worker Service

To subscribe to an existing SNS topic with a worker service, you'll need to edit the worker service's manifest.
Using the [`subscribe`](../manifest/worker-service/#subscribe) field in the manifest, you can define subscriptions to 
existing SNS topics exposed by other services in your environment.  In this example, we'll use the `ordersTopic` topic 
which the `api` service from the last section exposed. We'll also customize the worker service's queue to enable a dead-letter queue. 
The `tries` field tells SQS how many times to try redelivering a failed message before sending it to the DLQ for further inspection.

```yaml
name: orders-worker
type: Worker Service

subscribe:
  topics:
    - name: ordersTopic
      service: api
  queue:
    dead_letter:
      tries: 5
```

Copilot will create a subscription between this worker service's queue and the `ordersTopic` topic from the `api` service. It will also inject the queue URI into the service container under the environment variable `COPILOT_QUEUE_URI`.

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

### Javascript Example

The central business logic of a worker service's container involves pulling messages from the queue. To do this with the AWS SDK, you can use the SQS Clients for your language of choice. In Javascript, the logic for pulling, processing, and deleting messages from the queue would look like the following code snipped.

```javascript
const { SQSClient, ReceiveMessageCommand, DeleteMessageCommand } = require("@aws-sdk/client-sqs");
const client = new SQSClient({ region: "us-west-2" });
const out = await client.send(new ReceiveMessageCommand({
            QueueUrl: process.env.COPILOT_QUEUE_URI,
            WaitTimeSeconds: 10,
}));

console.log(`results: ${JSON.stringify(out)}`);
 
if (out.Messages === undefined || out.Messages.length === 0) {
    return;
}

// Process the message here.

await client.send( new DeleteMessageCommand({
    QueueUrl: process.env.COPILOT_QUEUE_URI,
    ReceiptHandle: out.Messages[0].ReceiptHandle,
}));
```
