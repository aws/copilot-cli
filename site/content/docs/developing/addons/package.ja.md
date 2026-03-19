# ローカルアーティファクトのアップロード <span class="version" >を [v1.21.0](../../../blogs/release-v121.ja.md) にて追加</span>

Copilot は、Addon テンプレートから参照されるローカルファイルを S3 にアップロードし、関連するリソースプロパティをアップロードされた S3 のロケーションに置き換えることをサポートしています。
[`copilot svc deploy`](../../commands/svc-deploy.ja.md) または [`copilot svc package --upload-assets`](../../commands/svc-package.ja.md) では、Addon テンプレートが CloudFormation に送られる前にサポート対象のリソースの特定のフィールドが、S3 のロケーションに更新されます。
ディスク上のテンプレートが変更されることはありません。
サポートされているリソースの全リストを見るには、[AWS CLI documentation](https://awscli.amazonaws.com/v2/documentation/api/latest/reference/cloudformation/package.html) をご覧ください。

この機能は、他の Copilot Service と同じリポジトリに保存されているローカルの Lambda 関数をデプロイするために使用することができます。
例えば、JavaScript の Lambda 関数を Copilot Service と一緒にデプロイするには、このリソースを [Addon テンプレート](./workload.ja.md) に追加してください。

???+ note "Lambda 関数の例"
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
              Runtime: nodejs22.x
        ```
    
    === "lambdas/example/index.js"

        ```js
        exports.handler = function (event, context) {
	        console.log('example event:', event);
	        context.succeed('success!');
        };
        ```

`copilot svc deploy` で、`lambdas/example` ディレクトリが zip 圧縮されて S3 にアップロードされ、`Code` プロパティが以下に更新されます。
```yaml
Code:
  S3Bucket: copilotBucket
  S3Key: hashOfLambdasExampleZip
```
Addon テンプレートが Copilot によってアップロードされ、デプロイされる前に更新されます。
特定のファイルを指定した場合、そのファイルを直接 S3 にアップロードします。
或いは特定のフォルダを指定した場合、フォルダを zip で圧縮してから S3 にアップロードされます。
zip が必要な一部のリソース (`AWS::Serverless::Function` など) では、アップロード前にファイルも zip で圧縮されます。

ファイルのパスは、リポジトリ内の `copilot/` ディレクトリの親からの相対パスとみなされます。
上記の例の場合、フォルダー構造は次のようになります。
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
絶対パスもサポートされていますが、複数のマシンに跨って上手く機能しない場合があります。

## 例: DynamoDB ストリームを処理する Lambda
この例では、[Amazon DynamoDB](https://aws.amazon.com/jp/dynamodb/) テーブルを作成し、[テーブルのストリーム](https://docs.aws.amazon.com/ja_jp/amazondynamodb/latest/developerguide/Streams.html) からのイベントを処理する Lambda 関数を接続する方法を説明します。
このアーキテクチャは、データを保存する際のレイテンシを最小にする必要がある Service がある場合に便利ですが、データを処理するのに時間がかかる別のプロセスをキックオフすることができます。


#### 前提条件
- [デプロイされた Copilot Service](../../concepts/services.ja.md)

#### 手順

1. `copilot storage init`  を実行して、Service 用の DynamoDB テーブル Addon を生成します。(詳細は[こちら](../storage.ja.md))
2. 生成された [`AWS::DynamoDB::Table`](https://docs.aws.amazon.com/ja_jp/AWSCloudFormation/latest/UserGuide/aws-resource-dynamodb-table.html) リソースに [`StreamSpecification`](https://docs.aws.amazon.com/ja_jp/AWSCloudFormation/latest/UserGuide/aws-resource-dynamodb-table.html#cfn-dynamodb-table-streamspecification) プロパティを追加します。
  ```yaml title="copilot/service-name/addons/ddb.yml"
  StreamSpecification:
    StreamViewType: NEW_AND_OLD_IMAGES
  ```
3. Lambda 関数、IAM Role、Lambda イベントストリームマッピングリソースを追加し、IAM ロールにて DynamoDB テーブルストリームへのアクセス権を付与します。
  ```yaml title="copilot/service-name/addons/ddb.yml" hl_lines="4 37 43"
    recordProcessor:
      Type: AWS::Lambda::Function
      Properties:
        Code: lambdas/record-processor/ # レコード処理する Lambda のローカルパス
        Handler: "index.handler"
        Timeout: 60
        MemorySize: 512
        Role: !GetAtt "recordProcessorRole.Arn"
        Runtime: nodejs22.x

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
                  # <table> を生成されたテーブルのリソース名に置き換えてください。
                  Resource: !Sub ${<table>.Arn}/stream/*

    tableStreamMappingToRecordProcessor:
      Type: AWS::Lambda::EventSourceMapping
      Properties:
        FunctionName: !Ref recordProcessor
        EventSourceArn: !GetAtt <table>.StreamArn # ここも <table> を置き換えてください。
        BatchSize: 1
        StartingPosition: LATEST
  ```
4. Lambda 関数を書いてください。
  ```js title="lambdas/record-processor/index.js"
  "use strict";
  const { unmarshall } = require('@aws-sdk/util-dynamodb');

  exports.handler = async function (event, context) {
    for (const record of event?.Records) {
      if (record?.eventName != "INSERT") {
        continue;
      }

      // process new records
      const item = unmarshall(record?.dynamodb?.NewImage);
      console.log("processing item", item);
    }
  };
  ```
5. `copilot svc deploy` を実行して、Lambda 関数をデプロイします!🎉
 Service がテーブルにレコードを追加すると、Lambda 関数がトリガーされ、新しいレコードを処理することができます。