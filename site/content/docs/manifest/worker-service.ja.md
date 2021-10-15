以下は `'Worker Service'` Manifest で利用できるすべてのプロパティのリストです。[Service の概念](../concepts/services.ja.md)説明のページも合わせてご覧ください。

???+ note "Worker Service の Manifest のサンプル"

    ```yaml
    # Service 名は、ロググループや ECS サービスなどのリソースの命名に使用されます。
    name: orders-worker
    type: Worker Service

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
      production:
        count:
          range:
            min: 1
            max: 50
            spot_from: 26
          queue_delay:
            acceptable_latency: 1m
            msg_processing_time: 250ms
    ```

<a id="name" href="#name" class="field">`name`</a> <span class="type">String</span>
Service の名前。

<div class="separator"></div>

<a id="type" href="#type" class="field">`type`</a> <span class="type">String</span>  

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

<span class="parent-field">topic.</span><a id="topic-queue" href="#topic-queue" class="field">`queue`</a> <span class="type">Boolean or Map</span>
任意項目。トピックに対する SQS キューの設定です。`true` を指定した場合、キューはデフォルト設定で作成されます。トピックに対応したキューに関する特性の属性についてカスタマイズする場合は、このフィールドを Map で指定していします。

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

<div class="separator"></div>

あるいは、Map を指定してオートスケーリングの設定も可能です。
```yaml
count:
  range: 1-10
  cpu_percentage: 70
  memory_percentage: 80
  queue_delay:
    acceptable_latency: 10m
    msg_processing_time: 250ms
```

<span class="parent-field">count.</span><a id="count-range" href="#count-range" class="field">`range`</a> <span class="type">String or Map</span>
メトリクスに指定した値に基づいて、Service が保つべきタスク数の最小と最大を範囲指定できます。
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

<span class="parent-field">range.</span><a id="count-range-min" href="#count-range-min" class="field">`min`</a> <span class="type">Integer</span>
Service がオートスケーリングを利用する場合の最小タスク数。

<span class="parent-field">range.</span><a id="count-range-max" href="#count-range-max" class="field">`max`</a> <span class="type">Integer</span>
Service がオートスケーリングを利用する場合の最大タスク数。

<span class="parent-field">range.</span><a id="count-range-spot-from" href="#count-range-spot-from" class="field">`spot_from`</a> <span class="type">Integer</span>
Service の何個目のタスクから Fargate Spot キャパシティプロバイダーを利用するか。

<span class="parent-field">count.</span><a id="count-cpu-percentage" href="#count-cpu-percentage" class="field">`cpu_percentage`</a> <span class="type">Integer</span>
Service が保つべき平均 CPU 使用率を指定し、それによってスケールアップ・ダウンします。

<span class="parent-field">count.</span><a id="count-memory-percentage" href="#count-memory-percentage" class="field">`memory_percentage`</a> <span class="type">Integer</span>
Service が保つべき平均メモリ使用率を指定し、それによってスケールアップ・ダウンします。

<span class="parent-field">count.</span><a id="count-queue-delay" href="#count-queue-delay" class="field">`queue_delay`</a> <span class="type">Integer</span>
タスク単位の許容可能なバックログをトラッキングし、許容可能なキュー遅延を維持するようにスケールアップ・ダウンします。
タスク単位の許容可能なバックログとは、`acceptable_latency` を `msg_processing_time`で割る事で計算されます。例えば、メッセージが到着後、10分以内に処理できれば良いとします。またメッセージを処理するのに平均 250 ミリ秒かかるとすると、この時、`acceptableBacklogPerTask = 10 * 60 / 0.25 = 2400` となります。各タスクは2,400 件のメッセージを処理することになります。
ターゲットトラッキングポリシーはタスクあたり 2400メッセージ以下の処理となる様に Service をスケールアップ・ダウンします。詳細については、[docs](https://docs.aws.amazon.com/ja_jp/autoscaling/ec2/userguide/as-using-sqs-queue.html)を確認してください.

<span class="parent-field">count.queue_delay.</span><a id="count-queue-delay-acceptable-latency" href="#count-queue-delay-acceptable-latency" class="field">`acceptable_latency`</a> <span class="type">Duration</span>
メッセージがキューに格納されている許容可能な時間。例えば、`"45s"`、 `"5m"`、`10h` を指定します。

<span class="parent-field">count.queue_delay.</span><a id="count-queue-delay-msg-processing-time" href="#count-queue-delay-msg-processing-time" class="field">`msg_processing_time`</a> <span class="type">Duration</span>
SQS メッセージ 1 件あたりの平均処理時間。例えば、`"250ms"`、`"1s"` を指定します。

{% include 'exec.ja.md' %}

{% include 'entrypoint.ja.md' %}

{% include 'command.ja.md' %}

{% include 'network.ja.md' %}

{% include 'envvars.ja.md' %}

{% include 'secrets.ja.md' %}

{% include 'storage.ja.md' %}

{% include 'publish.ja.md' %}

{% include 'logging.ja.md' %}

{% include 'taskdef-overrides.ja.md' %}

{% include 'environments.ja.md' %}
