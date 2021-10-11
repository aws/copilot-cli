// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0
const {
  SQSClient,
  ReceiveMessageCommand,
  DeleteMessageCommand,
} = require("@aws-sdk/client-sqs");
const { SNSClient, PublishCommand } = require("@aws-sdk/client-sns");
const axios = require("axios");
const sqsClient = new SQSClient({ region: process.env.AWS_DEFAULT_REGION });
const snsClient = new SNSClient({ region: process.env.AWS_DEFAULT_REGION });

console.log(`COPILOT_QUEUE_URI: ${process.env.COPILOT_QUEUE_URI}`);

const eventsQueue = process.env.COPILOT_QUEUE_URI;

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

(async () => {
  console.log(`The queue url created is: ${eventsQueue}`);
  while (true) {
    try {
      await sleep(300);
      const sqsOut = await sqsClient.send(
        new ReceiveMessageCommand({
          QueueUrl: eventsQueue,
          WaitTimeSeconds: 10,
        })
      );
      console.log(`results: ${JSON.stringify(sqsOut)}`);

      if (sqsOut.Messages === undefined || sqsOut.Messages.length === 0) {
        continue;
      }

      const resp = await axios.post(
        `http://frontend.${process.env.COPILOT_SERVICE_DISCOVERY_ENDPOINT}:8080/ack`
      );
      console.log(
        `response from frontend service: ${JSON.stringify(resp.data)}`
      );

      const parsedTopicArns = JSON.parse(process.env.COPILOT_SNS_TOPIC_ARNS);
      const snsOut = await snsClient.send(
        new PublishCommand({
          Message: "processed one message",
          TopicArn: parsedTopicArns["processed-msg-count"],
        })
      );
      console.log(JSON.stringify(snsOut));

      await sqsClient.send(
        new DeleteMessageCommand({
          QueueUrl: eventsQueue,
          ReceiptHandle: sqsOut.Messages[0].ReceiptHandle,
        })
      );
    } catch (err) {
      console.error(err);
    }
  }
})();
