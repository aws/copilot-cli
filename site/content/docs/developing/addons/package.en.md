# Uploading Local Artifacts <span class="version" > added in [v1.21.0](../../../blogs/release-v121.en.md) </span>

Copilot supports uploading local files referenced from your addon templates to S3, and replacing the relevant resource properties with the uploaded S3 location.
On [`copilot svc deploy`](../../commands/svc-deploy.en.md) or [`copilot svc package --upload-assets`](../../commands/svc-package.en.md), certain fields on supported resources will be updated with a S3 location before the addons template is sent to CloudFormation.
Your templates on disk will not be modified.
To see the full list of resources that are supported, take a look at the [AWS CLI documentation](https://awscli.amazonaws.com/v2/documentation/api/latest/reference/cloudformation/package.html).

This feature can be used to deploy local Lambda functions stored in the same repo as another Copilot service.
For example, to deploy a javascript Lambda function alongside a copilot service, you can add this resource to your [addon template](./workload.en.md):

???+ note "Example Lambda Function"
    === "copilot/service-name/addons/lambda.yml"

        ```yaml hl_lines="4"
          ExampleFunction:
            Type: AWS::Lambda::Function
            Properties:
              Code: lambdas/example/
              Handler: "index.handler"
              Timeout: 900
              MemorySize: 512
              Role: !GetAtt "ExampleFunctionRole.Arn"
              Runtime: nodejs16.x
        ```
    
    === "lambdas/example/index.js"

        ```js
        exports.handler = function (event, context) {
	        console.log('example event:', event);
	        context.succeed('success!');
        };
        ```

On `copilot svc deploy`, the `lambdas/example` directory will be zipped and uploaded to S3, and the `Code` property will be updated to:
```yaml
Code:
  S3Bucket: copilotBucket
  S3Key: hashOfLambdasExampleZip
```
before the addon template is uploaded and deployed by Copilot.
If you specify a file, the file is directly uploaded to S3.
If you specify a folder, the folder will be zipped before being uploaded to S3.
For some resources that require a zip (e.g., `AWS::Serverless::Function`), a file will be zipped before upload as well.

File paths are considered relative to the parent of the `copilot/` directory in your repo.
For the above example, the folder structure would look like:
```bash
.
├── copilot
│   └── example-service
│       ├── addons
│       │   └── lambda.yml
│       └── manifest.yml
└── lambdas
    └── example
        └── index.js
```
Absolute paths are supported as well, though they may not work as well across multiple machines.

## Example: DynamoDB Stream Processing Lambda
This example will walk through creating an [Amazon Dynamo DB](https://aws.amazon.com/dynamodb/) table with a lambda function connected to process events from the [table's stream](https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/Streams.html).
This architecture could be useful if you have a service that needs to minimize latency on storing data, but can kick off a separate process that takes longer to process the data.

#### Prerequisites
- [A deployed copilot service](../../concepts/services.en.md)

#### Steps
1. Generate a DynamoDB table addon for your service by running `copilot storage init` (More info [here!](../storage.en.md))
2. Add the [`StreamSpecification`](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-dynamodb-table.html#cfn-dynamodb-table-streamspecification) property to the generated [`AWS::DynamoDB::Table`](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-dynamodb-table.html) resource:
  ```yaml title="copilot/service-name/addons/ddb.yml"
  StreamSpecification:
    StreamViewType: NEW_AND_OLD_IMAGES
  ```
3. Add a Lambda function, IAM Role, and Lambda event stream mapping resource, making sure to give access to the DynamoDB table stream in the IAM Role:
  ```yaml title="copilot/service-name/addons/ddb.yml" hl_lines="4 37 43"
    recordProcessor:
      Type: AWS::Lambda::Function
      Properties:
        Code: lambdas/record-processor/ # local path to the record processor lambda
        Handler: "index.handler"
        Timeout: 60
        MemorySize: 512
        Role: !GetAtt "recordProcessorRole.Arn"
        Runtime: nodejs16.x

    recordProcessorRole:
      Type: AWS::IAM::Role
      Properties:
        AssumeRolePolicyDocument:
          Version: 2012-10-17
          Statement:
            - Effect: Allow
              Principal:
                Service:
                  - lambda.amazonaws.com
              Action:
                - sts:AssumeRole
        Path: /
        ManagedPolicyArns:
          - !Sub arn:${AWS::Partition}:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole
        Policies:
          - PolicyDocument:
              Version: 2012-10-17
              Statement:
                - Effect: Allow
                  Action:
                    - dynamodb:DescribeStream
                    - dynamodb:GetRecords
                    - dynamodb:GetShardIterator
                    - dynamodb:ListStreams
                  # replace <table> with the generated table's resource name
                  Resource: !Sub ${<table>.Arn}/stream/*

    tableStreamMappingToRecordProcessor:
      Type: AWS::Lambda::EventSourceMapping
      Properties:
        FunctionName: !Ref recordProcessor
        EventSourceArn: !GetAtt <table>.StreamArn # replace <table> here too
        BatchSize: 1
        StartingPosition: LATEST
  ```
4. Write your lambda function:
  ```js title="lambdas/record-processor/index.js"
  "use strict";
  const AWS = require('aws-sdk');

  exports.handler = async function (event, context) {
    for (const record of event?.Records) {
      if (record?.eventName != "INSERT") {
        continue;
      }

      // process new records
      const item = AWS.DynamoDB.Converter.unmarshall(record?.dynamodb?.NewImage);
      console.log("processing item", item);
    }
  };
  ```
5. Run `copilot svc deploy` to deploy your lambda function!🎉
As your service adds records to the table, the lambda function will be triggered and can process new records.