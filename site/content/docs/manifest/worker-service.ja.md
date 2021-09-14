以下は `'Worker Service'` Manifest で利用できるすべてのプロパティのリストです。[Service の概念](../concepts/services.ja.md)説明のページも合わせてご覧ください。

???+ note "Worker Service の Manifest のサンプル"

    ```yaml
    # Service 名は、ロググループや ECS サービスなどのリソースの命名に使用されます。
    name: orders-worker
    type: Worker Service

    # コンテナや Service の設定
    image:
      build: ./orders/Dockerfile

    subscribe:
      topics:
        - name: events
          service: api
        - name: events
          service: fe
      queue:
        retention: 96h
        timeout: 30s
        dead_letter:
          tries: 10

    cpu: 256
    memory: 512
    count: 1

    variables:
      LOG_LEVEL: info
    secrets:
      GITHUB_TOKEN: GITHUB_TOKEN

    # 上記で定義された値は、environment で上書きすることができます。
    environments:
      test:
        count:
          spot: 2
      production:
        count: 2
    ```

<a id="name" href="#name" class="field">`name`</a> <span class="type">String</span>
Service の名前。

<div class="separator"></div>

<a id="type" href="#type" class="field">`type`</a> <span class="type">String</span>  
The architecture type for your service. [Worker Services](../concepts/services.en.md#worker-service) are not reachable from the internet or elsewhere in the VPC. They are designed to pull messages from their associated SQS queues, which are populated by their subscriptions to SNS topics created by other Copilot services' `publish` fields.

Service のアーキテクチャタイプ。[Worker Service](../concepts/services.ja.md#worker-service) は、インターネットや VPC 外からはアクセスできません。Worker Service は関連する SQS キューからメッセージをプルするように設計されています。SQS キューは、他 のCopilot Service の `publish` フィールドで作成された SNS トピックへのサブスクリプションによって生成されます。


<div class="separator"></div>

<a id="subscribe" href="#subscribe" class="field">`subscribe`</a> <span class="type">Map</span>
`subscribe` セクションでは、Worker Service が、同じ Application や Environment にある他の Copilot Service が公開する SNS トピックへのサブスクリプションを作成できるようにします。各トピックは独自の SQS キューを定義できますが、デフォルトではすべてのトピックが Worker Service のデフォルトキューにサブスクライブされます。

```yaml
subscribe:
  topics:
    - name: events
      service: api
      queue: # api-events トピックの固有のキューを定義します。
        timeout: 20s 
    - name: events
      service: fe
  queue: # デフォルトでは、すべてのトピックからのメッセージは、共有キューに入ります。
    timeout: 45s
    retention: 96h
    delay: 30s
```

<span class="parent-field">subscribe.</span><a id="subscribe-queue" href="#subscribe-queue" class="field">`queue`</a> <span class="type">Map</span>
デフォルトでは、サービスレベルのキューが常に作成されます。`queue` では、そのデフォルトキューの特定の属性をカスタマイズすることができます。

<span class="parent-field">subscribe.queue.</span><a id="subscribe-queue-delay" href="#subscribe-queue-delay" class="field">`delay`</a> <span class="type">Duration</span>
キュー内のすべてのメッセージの配信を遅延させる時間を秒単位で指定します。デフォルトは 0 秒です。指定できる範囲は 0 秒 - 15 分です。

<span class="parent-field">subscribe.queue.</span><a id="subscribe-queue-retention" href="#subscribe-queue-retention" class="field">`retention`</a> <span class="type">Duration</span>
Retention はメッセージが削除される前にキューに残っている時間を指定します。デフォルトは 4 日です。指定できる範囲は 60 秒 - 336 時間 です。

<span class="parent-field">subscribe.queue.</span><a id="subscribe-queue-timeout" href="#subscribe-queue-timeout" class="field">`timeout`</a> <span class="type">Duration</span>
Timeout はメッセージが配信された後に利用できない時間の長さを定義します。デフォルトは 30 秒です。範囲は 0 秒 - 12 時間です。

<span class="parent-field">subscribe.queue.dead_letter.</span><a id="subscribe-queue-dead-letter-tries" href="#subscribe-queue-dead-letter-tries" class="field">`tries`</a> <span class="type">Integer</span>
指定された場合、DLQ(デッドレターキュー)を作成し、メッセージを `tries` 回試行した後に DLQ にルーティングするリドライブポリシーを設定します。つまり、Worker Service がメッセージの処理に `tries` 回成功しなかった場合、メッセージ送信はリトライされずに DLQ にルーティングされるため、あとからメッセージの内容を確認して失敗の原因分析に役立てることができます。

<span class="parent-field">subscribe.</span><a id="subscribe-topics" href="#subscribe-topics" class="field">`topics`</a> <span class="type">Array of `topic`s</span>
Worker Service がサブスクライブすべき SNS トピックの情報が含まれています。

<span class="parent-field">topic.</span><a id="topic-name" href="#topic-name" class="field">`name`</a> <span class="type">String</span>
必須項目。サブスクライブする SNS トピックの名前。

<span class="parent-field">topic.</span><a id="topic-service" href="#topic-service" class="field">`service`</a> <span class="type">String</span>
必須項目。この SNS トピックが公開されているサービスです。トピック名と合わせて、Copilot Environment 内で SNS トピックを一意に識別します。

{% include 'image-config.ja.md' %}

{% include 'image-healthcheck.ja.md' %}

{% include 'common-svc-fields.ja.md' %}
