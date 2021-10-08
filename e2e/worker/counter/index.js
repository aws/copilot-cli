// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0
const {
  SQSClient,
  ReceiveMessageCommand,
  DeleteMessageCommand,
} = require("@aws-sdk/client-sqs");
const axios = require("axios");
const client = new SQSClient({ region: process.env.AWS_DEFAULT_REGION });

console.log(`COPILOT_QUEUE_URI: ${process.env.COPILOT_QUEUE_URI}`);

const eventsQueue = process.env.COPILOT_QUEUE_URI;

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

let processedMsgCount = 0;

(async () => {
  console.log(`The queue url created is: ${eventsQueue}`);
  while (true) {
    try {
      await sleep(300);
      const out = await client.send(
        new ReceiveMessageCommand({
          QueueUrl: eventsQueue,
          WaitTimeSeconds: 10,
        })
      );
      console.log(`results: ${JSON.stringify(out)}`);

      if (out.Messages === undefined || out.Messages.length === 0) {
        continue;
      }

      const resp = await axios.post(
        `http://frontend.${process.env.COPILOT_SERVICE_DISCOVERY_ENDPOINT}:8080/update-count`,
        {
          count: ++processedMsgCount,
        }
      );
      console.log(`response from frontend service: ${JSON.stringify(resp)}`);

      await client.send(
        new DeleteMessageCommand({
          QueueUrl: eventsQueue,
          ReceiptHandle: out.Messages[0].ReceiptHandle,
        })
      );
    } catch (err) {
      console.error(err);
    }
  }
})();
