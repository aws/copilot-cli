# パブリッシュ / サブスクライブ アーキテクチャ

Copilot [Worker Services](../manifest/worker-service.ja.md)は、すべての Service タイプと Job タイプに共通する `publish` フィールドを利用して、サービス間でメッセージを受け渡すためのパブリッシュ / サブスクライブロジックを簡単に作成できます。

AWS 上での一般的なパターンは、メッセージの配信と処理を行うための SNS と SQS の組み合わせです。[SNS](https://docs.aws.amazon.com/ja_jp/sns/latest/dg/welcome.html) は堅牢なメッセージ配信システムで、メッセージの配信を保証しながら複数のサブスクライブしたエンドポイントにメッセージを送ることができます。

[SQS](https://docs.aws.amazon.com/ja_jp/AWSSimpleQueueService/latest/SQSDeveloperGuide/welcome.html)は、メッセージの非同期処理を可能にするメッセージキューです。キューには 1 つまたは複数の SNS トピックや、AWS EventBridge からのイベントを投入できます。

この 2 つのサービスを組み合わせることで、メッセージの送受信を効果的に疎結合にできます。つまり、パブリッシャーは自分のトピックをサブスクライブしているキューを意識する必要がなく、また Worker Service のコードはメッセージがどこから来るかを気にする必要がありません。

## パブリッシャーからのメッセージ送信

既存のサービスから SNS へのメッセージのパブリッシュを許可するには、Manifest に `publish` フィールドを設定するだけです。
SNS トピックの機能を表す名前を付けることを提案します。

```yaml
# api サービス用の manifest.yml
name: api
type: Backend Service

publish:
  topics:
    - name: ordersTopic
```

これにより、[SNS トピック](https://docs.aws.amazon.com/ja_jp/sns/latest/dg/welcome.html)が作成されます。また、トピックにリソースポリシーが設定され、AWS アカウントにある SQS キューがサブスクリプションを作成できるようになります。

また Copilot は、任意の SNS トピックの ARN をコンテナ内の環境変数 `COPILOT_SNS_TOPIC_ARNS` に注入します。
JSON 文字列の形式 :
```json
{
  "firstTopicName": "arn:aws:sns:us-east-1:123456789012:firstTopic",
  "secondTopicName": "arn:aws:sns:us-east-1:123456789012:secondTopic",
}
```

### Javascript での例
パブリッシャーのサービスがデプロイされると、AWS SDK を介して SNS にメッセージを送信できるようになります。

```javascript
const { SNSClient, PublishCommand } = require("@aws-sdk/client-sns");
const client = new SNSClient({ region: "us-west-2" });
const {ordersTopic} = JSON.parse(process.env.COPILOT_SNS_TOPIC_ARNS);
const out = await client.send(new PublishCommand({
   Message: "hello",
   TopicArn: ordersTopic,
 }));
```
## Worker Service でトピックをサブスクライブ

Worker Service で既存の SNS トピックをサブスクライブするには、Worker Service の Manifest を編集する必要があります。
Manifest の [`subscribe`](../manifest/worker-service/#subscribe) フィールドを使用して、Environment 内の他のサービスが公開する既存の SNS トピックへのサブスクリプションを定義します。この例では、前セクションの `api` サービスが公開する `ordersTopic` トピックを使用しています。また Worker Service のキューをカスタマイズして、DLQ(デッドレターキュー) を使えるようにします。
`tries` フィールドは失敗したメッセージを DLQ に送信し、失敗についての詳細な分析する前に、何回再配送を試みるかを SQS に伝えます。


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

Copilot は、この Worker Service のキューと、`api` サービスの `ordersTopic` トピックの間にサブスクリプションを作成します。また、キューの URI を、コンテナ内の環境変数 `COPILOT_QUEUE_URI` に注入します。

1 つ以上のトピック固有キューを指定した場合、`COPILOT_TOPIC_QUEUE_URIS` 変数を使ってそれらのキュー URI にアクセスできます。この変数は、トピック固有のキューの一意な識別子からその URI への JSON Map です。

例えば、`merchant` Service からの `orders` トピックと `merchant` Service からの FIFO トピック `transactions` のトピック別キューを持つワーカーサービスは、以下のような JSON 構造を持つことになります。

```json
// COPILOT_TOPIC_QUEUE_URIS
{
  "merchantOrdersEventsQueue": "https://sqs.eu-central-1.amazonaws.com/...",
  "merchantTransactionsfifoEventsQueue": "https://sqs.eu-central-1.amazonaws.com/..."
}
```

### Javascript での例

Worker Service 内のコンテナの中心となるビジネスロジックには、キューからメッセージをプルすることが含まれます。これを AWS SDK で行うには、選択した言語用の SQS クライアントを使用します。例えば Javascript でキューからメッセージをプルしたり、処理や削除をするためには、以下のようなコードスニペットになります。

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

// ここでメッセージを処理します。

await client.send( new DeleteMessageCommand({
    QueueUrl: process.env.COPILOT_QUEUE_URI,
    ReceiptHandle: out.Messages[0].ReceiptHandle,
}));
```