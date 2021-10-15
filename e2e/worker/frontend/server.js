// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

"use strict";

const express = require("express");
const { SNSClient, PublishCommand } = require("@aws-sdk/client-sns");

const PORT = 8080;
const HOST = "0.0.0.0";
const client = new SNSClient({ region: process.env.AWS_DEFAULT_REGION });
const app = express();
app.use(express.json());
app.use(
  express.urlencoded({
    extended: true,
  })
);

// Start the service waiting for an ack message, and count to be 0.
let status = "waiting on acknowledgement";
let count = 0;

// Each health check request from the ALB will result in publishing an event.
app.get("/", async (req, res) => {
  const { events } = JSON.parse(process.env.COPILOT_SNS_TOPIC_ARNS);
  const out = await client.send(
    new PublishCommand({
      Message: "healthcheck",
      TopicArn: events,
    })
  );
  console.log(JSON.stringify(out));
  res.send("hello");
});

app.get("/status", (req, res) => {
  res.send(status);
});

app.get("/count", (req, res) => {
  res.send(count.toString());
});

app.post("/update-count", (req, res) => {
  count = req.body.count;
  res.send("ok");
});

app.post("/ack", async (req, res) => {
  status = "consumed";
  res.send("ok");
});

app.listen(PORT, HOST);
console.log(`Running on http://${HOST}:${PORT}`);
