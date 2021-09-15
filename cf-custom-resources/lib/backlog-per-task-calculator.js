// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0
"use strict";

const aws = require("aws-sdk");

// AWS Clients that are overriden in tests.
let ecs, sqs;

/**
 * This lambda function calculates the backlog of SQS messages per running ECS tasks,
 * and writes the metric to CloudWatch.
 */
exports.handler = async (event, context) => {
  setupClients();
  try {
    const runningCount = await getRunningTaskCount(process.env.CLUSTER_NAME, process.env.SERVICE_NAME);
    const backlogs = await Promise.all(
      convertQueueNames(process.env.QUEUE_NAMES).map(async (queueName) => {
        const queueUrl = await getQueueURL(queueName);
        return {
          queueName: queueName,
          backlogPerTask: await getBacklogPerTask(queueUrl, runningCount),
        };
      })
    );
    const timestamp = Date.now();
    for (const {queueName, backlogPerTask} of backlogs) {
      emitBacklogPerTaskMetric(process.env.NAMESPACE, timestamp, queueName, backlogPerTask);
    }
  } catch(err) {
    // If there is any issue we won't log a metric.
    // This is okay because autoscaling will maintain the current number of running tasks if a data point is missing.
    // See https://docs.aws.amazon.com/AmazonECS/latest/developerguide/service-autoscaling-targettracking.html#targettracking-considerations
    console.error(`Unexpected error ${err}`);
  }
};

/**
 * Returns the backlog per task. The backlog per task is calculated by dividing the number of messages in the queue with
 * the number of running tasks.
 * If there are no running task, we return the total number of messages in the queue so that we can start scaling up.
 * @param queueUrl The url of the queue.
 * @param runningTaskCount The number of running tasks part of the ECS service.
 * @return int The expected number of messages each running task will consume.
 */
const getBacklogPerTask = async (queueUrl, runningTaskCount) => {
  const adjustedRunningTasks = runningTaskCount === 0 ? 1 : runningTaskCount;
  const totalNumberOfMessages = await getQueueDepth(queueUrl);
  return Math.ceil(totalNumberOfMessages/adjustedRunningTasks);
}

/**
 * Writes the backlogPerTask metric for the given queue to stdout following the CloudWatch embedded metric format.
 * @see https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/CloudWatch_Embedded_Metric_Format_Generation.html
 * @param namespace The namespace for the metric.
 * @param timestamp The number of milliseconds after Jan 1, 1970 00:00:00 UTC used to emit the metric.
 * @param queueName The name of the queue.
 * @param backlogPerTask The number of messages in the queue divided by the number of running tasks.
 */
const emitBacklogPerTaskMetric = (namespace, timestamp, queueName, backlogPerTask) => {
  console.log(JSON.stringify({
    "_aws": {
      "Timestamp": timestamp,
      "CloudWatchMetrics": [{
        "Namespace": namespace,
        "Dimensions": [["QueueName"]],
        "Metrics": [{"Name":"BacklogPerTask", "Unit": "Count"}]
      }],
    },
    "QueueName": queueName,
    "BacklogPerTask": backlogPerTask,
  }));
}

/**
 * Returns the URL for the SQS queue.
 * @param queueName The name of the queue.
 * @returns string The URL of the queue.
 */
const getQueueURL = async (queueName) => {
  const out = await sqs.getQueueUrl({
    QueueName: queueName,
  }).promise();
  return out.QueueUrl;
}

/**
 * Returns the total number of messages in the SQS queue.
 * @param queueUrl The URL of the SQS queue.
 * @return int The ApproximateNumberOfMessages in the queue.
 */
const getQueueDepth = async (queueUrl) => {
  const out = await sqs.getQueueAttributes({
    QueueUrl: queueUrl,
    AttributeNames: ['ApproximateNumberOfMessages'],
  }).promise();
  return out.Attributes.ApproximateNumberOfMessages;
}

/**
 * Returns the number of running tasks part of the service.
 * @param clusterId The short name or full Amazon Resource Name (ARN) of the cluster.
 * @param serviceName The service name or full Amazon Resource Name (ARN) of the service.
 * @returns int The number of tasks running part of the service.
 */
const getRunningTaskCount = async (clusterId, serviceName) => {
  const out = await ecs.describeServices({
    cluster: clusterId,
    services: [serviceName],
  }).promise();
  if (out.services.length === 0) {
    throw new Error(`service ${serviceName} of cluster ${clusterId} does not exist`);
  }
  return out.services[0].runningCount;
}

/**
 * Create new clients.
 */
const setupClients = () => {
  ecs = new aws.ECS();
  sqs = new aws.SQS();
}

// convertQueueNames takes a comma separated string of SQS queue names and returns it as an array of strings.
const convertQueueNames = (stringToSplit) => {
  return stringToSplit.split(',')
}