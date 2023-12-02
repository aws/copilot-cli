以下は `'Worker Service'` Manifest で利用できるすべてのプロパティのリストです。[Service の概念](../concepts/services.ja.md)説明のページも合わせてご覧ください。

???+ note "Worker Service の サンプル Manifest"

    === "Single queue"

        ```yaml
        # 他の Service から発行された複数のトピックから、単一の SQS キューにメッセージを集めます。
        name: cost-analyzer
        type: Worker Service

        image:
          build: ./cost-analyzer/Dockerfile

        subscribe:
          topics:
            - name: products
              service: orders
              filter_policy:
                event:
                - anything-but: order_cancelled
            - name: inventory
              service: warehouse
          queue:
            retention: 96h
            timeout: 30s
            dead_letter:
              tries: 10

        cpu: 256
        memory: 512
        count: 3
        exec: true

        secrets:
          DB:
            secretsmanager: '${COPILOT_APPLICATION_NAME}/${COPILOT_ENVIRONMENT_NAME}/mysql'
        ```

    === "Spot autoscaling"

        ```yaml
        # キャパシティに余裕がある場合は、Fargate Spot のタスクにバーストします。
        name: cost-analyzer
        type: Worker Service

        image:
          build: ./cost-analyzer/Dockerfile

        subscribe:
          topics:
            - name: products
              service: orders
            - name: inventory
              service: warehouse

        cpu: 256
        memory: 512
        count:
          range:
            min: 1
            max: 10
            spot_from: 2
          queue_delay: # 1 つのメッセージ処理に 250ms かかると仮定して、10 分以内にメッセージが処理されることを確認します。
            acceptable_latency: 10m
            msg_processing_time: 250ms
        exec: true
        ```

    === "Separate queues"

        ```yaml
        # 各トピックに個別のキューを割り当てます。
        name: cost-analyzer
        type: Worker Service

        image:
          build: ./cost-analyzer/Dockerfile

        subscribe:
          topics:
            - name: products
              service: orders
              queue:
                retention: 5d
                timeout: 1h
                dead_letter:
                  tries: 3
            - name: inventory
              service: warehouse
              queue:
                retention: 1d
                timeout: 5m
        count: 1
        ```

<a id="name" href="#name" class="field">`name`</a> <span class="type">String</span>  
Service の名前。

<div class="separator"></div>

<a id="type" href="#type" class="field">`type`</a> <span class="type">String</span>  

Service のアーキテクチャタイプ。[Worker Service](../concepts/services.ja.md#worker-service) は、インターネットや VPC 外からはアクセスできません。Worker Service は関連する SQS キューからメッセージをプルするように設計されています。SQS キューは、他の Copilot Service の `publish` フィールドで作成された SNS トピックへのサブスクリプションによって生成されます。


<div class="separator"></div>

<a id="subscribe" href="#subscribe" class="field">`subscribe`</a> <span class="type">Map</span>  
`subscribe` セクションでは、Worker Service が、同じ Application や Environment にある他の Copilot Service が公開する SNS トピックへのサブスクリプションを作成できるようにします。各トピックは独自の SQS キューを定義できますが、デフォルトではすべてのトピックが Worker Service のデフォルトキューにサブスクライブされます。

デフォルトキューの URI は、環境変数 `COPILOT_QUEUE_URI` としてコンテナにインジェクトされます。

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
デフォルトでは、サービスレベルのキューが常に作成されます。`queue` では、そのデフォルトキューの特定の属性をカスタマイズできます。

<span class="parent-field">subscribe.queue.</span><a id="subscribe-queue-delay" href="#subscribe-queue-delay" class="field">`delay`</a> <span class="type">Duration</span>  
キュー内のすべてのメッセージの配信を遅延させる時間を秒単位で指定します。デフォルトは 0 秒です。指定できる範囲は 0 秒 - 15 分です。

<span class="parent-field">subscribe.queue.</span><a id="subscribe-queue-retention" href="#subscribe-queue-retention" class="field">`retention`</a> <span class="type">Duration</span>  
Retention はメッセージが削除される前にキューに残っている時間を指定します。デフォルトは 4 日です。指定できる範囲は 60 秒 - 336 時間です。

<span class="parent-field">subscribe.queue.</span><a id="subscribe-queue-timeout" href="#subscribe-queue-timeout" class="field">`timeout`</a> <span class="type">Duration</span>
Timeout はメッセージが配信された後に利用できない時間の長さを定義します。デフォルトは 30 秒です。範囲は 0 秒 - 12 時間です。

<span class="parent-field">subscribe.queue.</span><a id="subscribe-queue-fifo" href="#subscribe-queue-fifo" class="field">`fifo`</a> <span class="type">Boolean or Map</span>
SQS キューで FIFO (first in, first out) 順を有効化します。操作やイベントの順番が重要であったり、重複が許容されないシナリオに対処します。

```yaml
subscribe:
  topics:
    - name: events
      service: api
    - name: events
      service: fe
  queue: # 両方の FIFO SNS トピックからのメッセージは、 共有の FIFO SQS キューに入ります。
    fifo: true
```
キューで FIFO 機能を有効化する場合、Copilot はソース SNS トピックも[FIFO](../include/publish.ja.md#publish-topics-topic-fifo)であることを要求します。

または、 高度な SQS FIFO キュー設定を指定できます。
```yaml
subscribe:
  topics:
    - name: events
      service: api
      queue: # api-event トピックに対応した標準キューを定義します。
        timeout: 20s
    - name: events
      service: fe
  queue: # デフォルトでは、全ての FIFO トピックからのメッセージは、共有の FIFO SQS キューに入ります。
    fifo:
      content_based_deduplication: true
      high_throughput: true
```

<span class="parent-field">subscribe.queue.fifo.</span><a id="subscribe-queue-fifo-content-based-deduplication" href="#subscribe-queue-fifo-content-based-deduplication" class="field">`content_based_deduplication`</a> <span class="type">Boolean</span>
パブリッシュされたメッセージごとにメッセージ本文が一意である事が保証されている場合、SNS FIFO トピックのコンテンツベースの重複排除を有効化できます。

<span class="parent-field">subscribe.queue.fifo.</span><a id="subscribe-queue-fifo-deduplication-scope" href="#subscribe-queue-fifo-deduplication-scope" class="field">`deduplication_scope`</a> <span class="type">String</span>
FIFO キューで高スループットが必要な場合、メッセージ重複排除をメッセージグループで行うかキュー全体で行うかを指定します。設定可能な値は、"messageGroup" と "queue" です。

<span class="parent-field">subscribe.queue.fifo.</span><a id="subscribe-queue-fifo-throughput-limit" href="#subscribe-queue-fifo-throughput-limit" class="field">`throughput_limit`</a> <span class="type">String</span>
FIFO キューで高スループットが必要な場合、FIFO キュースループットの上限をキュー全体に適用するか、メッセージグループ単位で適用するかを指定します。設定可能な値は、"perQueue" と "perMessageGroupId" です。

<span class="parent-field">subscribe.queue.fifo.</span><a id="subscribe-queue-fifo-high-throughput" href="#subscribe-queue-fifo-high-throughput" class="field">`high_throughput`</a> <span class="type">Boolean</span>
有効にした場合、 FIFO キューにおいて、より高い秒間トランザクション (TPS) が利用できます。`deduplication_scope` および `throughput_limit` と相互排他的です。

<span class="parent-field">subscribe.queue.dead_letter.</span><a id="subscribe-queue-dead-letter-tries" href="#subscribe-queue-dead-letter-tries" class="field">`tries`</a> <span class="type">Integer</span>  
指定された場合、DLQ(デッドレターキュー)を作成し、メッセージを `tries` 回試行した後に DLQ にルーティングするリドライブポリシーを設定します。つまり、Worker Service がメッセージの処理に `tries` 回成功しなかった場合、メッセージ送信はリトライされません。 メッセージは DLQ にルーティングされるため、あとからメッセージの内容を確認して失敗の原因分析に役立てることができます。

<span class="parent-field">subscribe.</span><a id="subscribe-topics" href="#subscribe-topics" class="field">`topics`</a> <span class="type">Array of `topic`s</span>  
Worker Service がサブスクライブすべき SNS トピックの情報が含まれています。

<span class="parent-field">subscribe.topics.topic</span><a id="topic-name" href="#topic-name" class="field">`name`</a> <span class="type">String</span>  
必須項目。サブスクライブする SNS トピックの名前。

<span class="parent-field">subscribe.topics.topic</span><a id="topic-service" href="#topic-service" class="field">`service`</a> <span class="type">String</span>  
必須項目。この SNS トピックが公開されているサービスです。トピック名と合わせて、Copilot Environment 内で SNS トピックを一意に識別します。

<span class="parent-field">subscribe.topics.topic</span><a id="topic-filter-policy" href="#topic-filter-policy" class="field">`filter_policy`</a> <span class="type">Map</span>  
任意項目。SNS サブスクリプションフィルターポリシーを指定します。このポリシーは、着信メッセージの属性を評価します。フィルターポリシーは JSON で指定します。例えば以下の様になります。
```json
filter_policy: {"store":["example_corp"],"event":[{"anything-but":"order_cancelled"}],"customer_interests":["rugby","football","baseball"],"price_usd":[{"numeric":[">=",100]}]}
```
または、YAML の MAP を利用して記述します。
```yaml
filter_policy:
  store:
    - example_corp
  event:
    - anything-but: order_cancelled
  customer_interests:
    - rugby
    - football
    - baseball
  price_usd:
    - numeric:
      - ">="
      - 100
```
フィルターポリシーの書き方に関するさらに詳しい情報については、[SNS documentation](https://docs.aws.amazon.com/sns/latest/dg/sns-subscription-filter-policies.html)を確認してください。

<span class="parent-field">subscribe.topics.topic.</span><a id="topic-queue" href="#topic-queue" class="field">`queue`</a> <span class="type">Boolean or Map</span>
任意項目。トピックに対する SQS キューの設定です。`true` を指定した場合、キューはデフォルト設定で作成されます。トピックに対応したキューに関する特性の属性についてカスタマイズする場合は、このフィールドを Map で指定します。
1 つ以上のトピック固有キューを指定した場合、`COPILOT_TOPIC_QUEUE_URIS` 変数を使ってそれらのキュー URI にアクセスできます。この変数は、トピック固有のキューの一意な識別子からその URI への JSON Map です。

例えば、`merchant` Service からの `orders` トピックと `merchant` Service からの FIFO トピック `transactions` のトピック別キューを持つワーカーサービスは、以下のような JSON 構造を持つことになります。

```json
// COPILOT_TOPIC_QUEUE_URIS
{
  "merchantOrdersEventsQueue": "https://sqs.eu-central-1.amazonaws.com/...",
  "merchantTransactionsfifoEventsQueue": "https://sqs.eu-central-1.amazonaws.com/..."
}
```

<span class="parent-field">subscribe.topics.topic.queue.</span><a id="subscribe-topics-topic-queue-fifo" href="#subscribe-topics-topic-queue-fifo" class="field">`fifo`</a> <span class="type">Boolean or Map</span>
任意項目。トピックの SQS FIFO キューに対する設定です。`true` を指定した場合、 FIFO キューがデフォルトの FIFO 設定で作成されます。
トピックに対応したキューに対する特定の属性についてカスタマイズする場合は、このフィールドを Map で指定します。

{% include 'image.ja.md' %}

{% include 'image-config.ja.md' %}

{% include 'task-size.ja.md' %}

{% include 'platform.ja.md' %}

<div class="separator"></div>

<a id="count" href="#count" class="field">`count`</a> <span class="type">Integer or Map</span>
次の様に指定すると、
```yaml
count: 5
```
Service は、希望するタスク数を 5 に設定し、Service 内に 5 つのタスクが起動している様に保ちます。

<span class="parent-field">count.</span><a id="count-spot" href="#count-spot" class="field">`spot`</a> <span class="type">Integer</span>

`spot` サブフィールドに数値を指定することで、Service の実行に Fargate Spot キャパシティを利用できます。
```yaml
count:
  spot: 5
```
!!! info
    ARM アーキテクチャで動作するコンテナでは、Fargate Spot はサポートされていません。

<div class="separator"></div>

あるいは、Map を指定してオートスケーリングの設定も可能です。
```yaml
count:
  range: 1-10
  cpu_percentage: 70
  memory_percentage:
    value: 80
    cooldown:
      in: 80s
      out: 160s
  queue_delay:
    acceptable_latency: 10m
    msg_processing_time: 250ms
    cooldown:
      in: 30s
      out: 60s
```

<span class="parent-field">count.</span><a id="count-range" href="#count-range" class="field">`range`</a> <span class="type">String or Map</span>
メトリクスで指定した値に基づいて、Service が保つべきタスク数の最小と最大を範囲指定できます。
```yaml
count:
  range: n-m
```
これにより Application Auto Scaling がセットアップされ、`MinCapacity` に `n` が、`MaxCapacity` に `m` が設定されます。

あるいは次の例に挙げるように `range` フィールド以下に `min` と `max` を指定し、加えて `spot_from` フィールドを利用することで、一定数以上のタスクを実行する場合に Fargate Spot キャパシティを利用する設定が可能です。

```yaml
count:
  range:
    min: 1
    max: 10
    spot_from: 3
```

上記の例では Application Auto Scaling は 1-10 の範囲で設定されますが、最初の２タスクはオンデマンド Fargate キャパシティに配置されます。Service が３つ以上のタスクを実行するようにスケールした場合、３つ目以降のタスクは最大タスク数に達するまで Fargate Spot に配置されます。

<span class="parent-field">count.range.</span><a id="count-range-min" href="#count-range-min" class="field">`min`</a> <span class="type">Integer</span>
Service がオートスケーリングを利用する場合の最小タスク数。

<span class="parent-field">count.range.</span><a id="count-range-max" href="#count-range-max" class="field">`max`</a> <span class="type">Integer</span>
Service がオートスケーリングを利用する場合の最大タスク数。

<span class="parent-field">count.range.</span><a id="count-range-spot-from" href="#count-range-spot-from" class="field">`spot_from`</a> <span class="type">Integer</span>
Service の何個目のタスクから Fargate Spot キャパシティプロバイダーを利用するか。

<span class="parent-field">count.</span><a id="count-cooldown" href="#count-cooldown" class="field">`cooldown`</a> <span class="type">Map</span>
指定されたすべてのオートスケーリングフィールドのデフォルトクールダウンとして使用されるクールダウンスケーリングフィールド。

<span class="parent-field">count.cooldown.</span><a id="count-cooldown-in" href="#count-cooldown-in" class="field">`in`</a> <span class="type">Duration</span>
Service をスケールアップするためのオートスケーリングクールダウン時間。

<span class="parent-field">count.cooldown.</span><a id="count-cooldown-out" href="#count-cooldown-out" class="field">`out`</a> <span class="type">Duration</span>
Service をスケールダウンさせるためのオートスケーリングクールダウン時間。

`cpu_percentage` および `memory_percentage`  は `count` のオートスケーリングフィールドであり、フィールドの値として定義するか、または `value` と `cooldown` にて関連する詳細情報を含むマップとして定義することができます。
```yaml
value: 50
cooldown:
  in: 30s
  out: 60s
```
The cooldown specified here will override the default cooldown.

<span class="parent-field">count.</span><a id="count-cpu-percentage" href="#count-cpu-percentage" class="field">`cpu_percentage`</a> <span class="type">Integer or Map</span>
Service が保つべき平均 CPU 使用率を指定し、それによってスケールアップ・ダウンします。

<span class="parent-field">count.</span><a id="count-memory-percentage" href="#count-memory-percentage" class="field">`memory_percentage`</a> <span class="type">Integer</span>
Service が保つべき平均メモリ使用率を指定し、それによってスケールアップ・ダウンします。

<span class="parent-field">count.</span><a id="count-queue-delay" href="#count-queue-delay" class="field">`queue_delay`</a> <span class="type">Integer</span>
タスク単位の許容可能なバックログをトラッキングし、許容可能なキュー遅延を維持するようにスケールアップ・ダウンします。
タスク単位の許容可能なバックログとは、`acceptable_latency` を `msg_processing_time` で割って計算されます。例えば、メッセージが到着後、10 分以内に処理できれば良いとします。またメッセージを処理するのに平均 250 ミリ秒かかるとすると、この時、`acceptableBacklogPerTask = 10 * 60 / 0.25 = 2400` となります。各タスクは 2,400 件のメッセージを処理することになります。
ターゲットトラッキングポリシーはタスクあたり 2400 メッセージ以下の処理となる様に Service をスケールアップ・ダウンします。詳細については、[docs](https://docs.aws.amazon.com/ja_jp/autoscaling/ec2/userguide/as-using-sqs-queue.html)を確認してください。

<span class="parent-field">count.queue_delay.</span><a id="count-queue-delay-acceptable-latency" href="#count-queue-delay-acceptable-latency" class="field">`acceptable_latency`</a> <span class="type">Duration</span>
メッセージがキューに格納されている許容可能な時間。例えば、`"45s"`、 `"5m"`、`10h` を指定します。

<span class="parent-field">count.queue_delay.</span><a id="count-queue-delay-msg-processing-time" href="#count-queue-delay-msg-processing-time" class="field">`msg_processing_time`</a> <span class="type">Duration</span>
SQS メッセージ 1 件あたりの平均処理時間。例えば、`"250ms"`、`"1s"` を指定します。

{% include 'exec.ja.md' %}

{% include 'deployment.ja.md' %}

```yaml 
deployment:
  rollback_alarms:
    cpu_utilization: 70    // Percentage value at or above which alarm is triggered.
    memory_utilization: 50 // Percentage value at or above which alarm is triggered.
    messages_delayed: 5    // Number of delayed messages in the queue at or above which alarm is triggered. 
```

{% include 'entrypoint.ja.md' %}

{% include 'command.ja.md' %}

{% include 'network.ja.md' %}

{% include 'envvars.ja.md' %}

{% include 'secrets.ja.md' %}

{% include 'storage.ja.md' %}

{% include 'publish.ja.md' %}

{% include 'logging.ja.md' %}

{% include 'observability.ja.md' %}

{% include 'taskdef-overrides.ja.md' %}

{% include 'environments.ja.md' %}
