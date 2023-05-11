以下は `'Backend Service'` Manifest で利用できるすべてのプロパティのリストです。[Copilot Service の概念](../concepts/services.ja.md)説明のページも合わせてご覧ください。

???+ note "api service のサンプル Manifest"

    === "Serving Internal Traffic"

        ```yaml
            name: api
            type: Backend Service

            image:
              build: ./api/Dockerfile
              port: 8080
              healthcheck:
                command: ["CMD-SHELL", "curl -f http://localhost:8080 || exit 1"]
                interval: 10s
                retries: 2
                timeout: 5s
                start_period: 0s

            network:
              connect: true

            cpu: 256
            memory: 512
            count: 2
            exec: true
    
            env_file: ./api/.env
            environments:
              test:
                deployment:
                  rolling: "recreate"
                count: 1
        ```

    === "Internal Application Load Balancer"

        ```yaml
        # Service は、次の場所でアクセス可能です。
        # http://api.${COPILOT_ENVIRONMENT_NAME}.${COPILOT_APPLICATION_NAME}.internal
        # これは VPC 内のみの内部ロードバランサーの内側にあります。
        name: api
        type: Backend Service
    
        image:
          build: ./api/Dockerfile
          port: 8080

        http:
          path: '/'
          healthcheck:
            path: '/_healthcheck'
            success_codes: '200,301'
            healthy_threshold: 3
            interval: 15s
            timeout: 10s
            grace_period: 30s
          deregistration_delay: 50s

        network:
          vpc:
            placement: 'private'

        count:
          range: 1-10
          cpu_percentage: 70
          requests: 10
          response_time: 2s

        secrets:
          GITHUB_WEBHOOK_SECRET: GH_WEBHOOK_SECRET
          DB_PASSWORD:
            secretsmanager: 'demo/test/mysql:password::'
        ```

    === "With a domain"

        ```yaml
        # プライベート証明書がインポートされている Environment であれば、
        # HTTPS のエンドポイントを Service に割り当てることができます。
        # https://aws.github.io/copilot-cli/docs/manifest/environment#http-private-certificates を参照してください。

        name: api
        type: Backend Service
    
        image:
          build: ./api/Dockerfile
          port: 8080

        http:
          path: '/'
          alias: 'v1.api.example.com'
          hosted_zone: AN0THE9H05TED20NEID # v1.api.example.com のレコードをホストゾーンに挿入します。

        count: 1
        ```

    === "Event-driven"

        ```yaml
        # https://aws.github.io/copilot-cli/docs/developing/publish-subscribe/ を参照してください。
        name: warehouse
        type: Backend Service
    
        image:
          build: ./warehouse/Dockerfile
          port: 80

        publish:
          topics:
            - name: 'inventory'
            - name: 'orders'
              fifo: true
        variables:
          DDB_TABLE_NAME: 'inventory'

        count:
          range: 3-5
          cpu_percentage: 70
          memory_percentage: 80
        ```

    === "Shared file system"

        ```yaml
        # http://localhost:8000/copilot-cli/docs/developing/storage.ja.md#ファイルシステム を参照してください。
        name: sync
        type: Backend Serivce

        image:
          build: Dockerfile

        variables:
          S3_BUCKET_NAME: my-userdata-bucket

        storage:
          volumes:
            userdata: 
              path: /etc/mount1
              efs:
                id: fs-1234567
        ```

    === "Expose Multiple Ports"

        ```yaml
        name: 'backend'
        type: 'Backend Service'
    
        image:
          build: './backend/Dockerfile'
          port: 8080
    
        http:
          path: '/'
          target_port: 8083           # Traffic on "/" is forwarded to the main container, on port 8083. 
          additional_rules:
            - path: 'customerdb'
              target_port: 8081       # Traffic on "/customerdb" is forwarded to the main container, on port 8081.
            - path: 'admin' 
              target_port: 8082       # Traffic on "/admin" is forwarded to the sidecar "envoy", on port 8082.
              target_container: envoy
    
        sidecars:
          envoy:
            port: 80
            image: aws_account_id.dkr.ecr.us-west-2.amazonaws.com/envoy-proxy-with-selfsigned-certs:v1
        ```    

<a id="name" href="#name" class="field">`name`</a> <span class="type">String</span>  
Service 名。

<div class="separator"></div>

<a id="type" href="#type" class="field">`type`</a> <span class="type">String</span>  
Service のアーキテクチャ。[Backend Services](../concepts/services.ja.md#backend-service) はインターネット側からはアクセスできませんが、[サービスディスカバリ](../developing/svc-to-svc-communication.ja.md#service-discovery)の利用により他の Service からはアクセスできます。

<div class="separator"></div>

<a id="http" href="#http" class="field">`http`</a> <span class="type">Map</span>  
http セクションは Service と内部 Application Load Balancer の連携に関するパラメーターを含みます。

<span class="parent-field">http.</span><a id="http-path" href="#http-path" class="field">`path`</a> <span class="type">String</span>  
このパスに対するリクエストが Service に転送されます。各 Load Balanced Web Service は、ユニークなパスでリッスンする必要があります。

{% include 'http-healthcheck.ja.md' %}

<span class="parent-field">http.</span><a id="http-deregistration-delay" href="#http-deregistration-delay" class="field">`deregistration_delay`</a> <span class="type">Duration</span>  
登録解除時にターゲットがクライアントとの接続を閉じるのを待つ時間を指定します。デフォルトでは 60 秒です。この値を大きくするとターゲットが安全に接続を閉じるための時間を確保できますが、新バージョンのデプロイに必要となる時間が長くなります。範囲は 0 〜 3600 です。

<span class="parent-field">http.</span><a id="http-target-container" href="#http-target-container" class="field">`target_container`</a> <span class="type">String</span>
サイドカーコンテナを指定することで、Service のメインコンテナの代わりにサイドカーでロードバランサーからのリクエストを受け取れます。
ターゲットコンテナのポートが `443` に設定されている場合、プロトコルは `HTTP` に設定され、ロードバランサーは
Fargate タスクと TLS 接続します。ターゲットコンテナにインストールされた証明書が利用されます。

<span class="parent-field">http.</span><a id="http-stickiness" href="#http-stickiness" class="field">`stickiness`</a> <span class="type">Boolean</span>  
スティッキーセッションの有効化、あるいは無効化を指定します。

<span class="parent-field">http.</span><a id="http-allowed-source-ips" href="#http-allowed-source-ips" class="field">`allowed_source_ips`</a> <span class="type">Array of Strings</span>  
Service へのアクセスを許可する CIDR IP アドレスのリストを指定します。
```yaml
http:
  allowed_source_ips: ["192.0.2.0/24", "198.51.100.10/32"]
```

<span class="parent-field">http.</span><a id="http-alias" href="#http-alias" class="field">`alias`</a> <span class="type">String or Array of Strings or Array of Maps</span>  
Service の HTTPS ドメインエイリアス。
```yaml
# String version.
http:
  alias: example.com
# Alternatively, as an array of strings.
http:
  alias: ["example.com", "v1.example.com"]
# Alternatively, as an array of maps.
http:
  alias:
    - name: example.com
      hosted_zone: Z0873220N255IR3MTNR4
    - name: v1.example.com
      hosted_zone: AN0THE9H05TED20NEID
```
<span class="parent-field">http.</span><a id="http-hosted-zone" href="#http-hosted-zone" class="field">`hosted_zone`</a> <span class="type">String</span>  
既存のプライベートホストゾーンの ID。内部ロードバランサーの作成後に、 Copilot がエイリアスレコードを挿入し、エイリアス名を LB の DNS 名にマッピングします。 `alias` と共に使用します。
```yaml
http:
  alias: example.com
  hosted_zone: Z0873220N255IR3MTNR4
# Also see http.alias array of maps example, above.
```
<span class="parent-field">http.</span><a id="http-version" href="#http-version" class="field">`version`</a> <span class="type">String</span>  
HTTP(S) プロトコルのバージョン。 `'grpc'`、 `'http1'`、または `'http2'` を指定します。省略した場合は、`'http1'` が利用されます。 
gRPC を利用する場合は、Application にドメインが関連付けられていなければなりません。

<span class="parent-field">http.</span><a id="http-additional-rules" href="#http-additional-rules" class="field">`additional_rules`</a> <span class="type">Array of Maps</span>
複数の ALB リスナールール設定します。

{% include 'http-additionalrules.ja.md' %}

{% include 'image-config-with-port.ja.md' %}
ポートを `443` に設定し、 内部ロードバランサーが `http` で有効化されている場合、プロトコルは `HTTPS` に設定され、ロードバランサーは Fargate タスクと TLS 接続します。ターゲットコンテナにインストールされた証明書が利用されます。

{% include 'image-healthcheck.ja.md' %}

{% include 'task-size.ja.md' %}

{% include 'platform.ja.md' %}

<div class="separator"></div>

<a id="count" href="#count" class="field">`count`</a> <span class="type">Integer or Map</span>
Service が保つべきタスクの数。

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
  cooldown:
    in: 30s
  cpu_percentage: 70
  memory_percentage:
    value: 80
    cooldown:
      out: 45s
  requests: 10000
  response_time: 2s
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

`cpu_percentage`、`memory_percentage`、`requests` および `response_time` のオプションは、オートスケーリングに関する `count` フィールドにて、フィールド値としてあるいはフィールド値とクールダウン設定に関する詳細情報を含むマップとして定義することができます。
```yaml
value: 50
cooldown:
  in: 30s
  out: 60s
```
ここで指定したクールダウン設定は、デフォルトのクールダウン設定より優先されます。

<span class="parent-field">count.</span><a id="count-cpu-percentage" href="#count-cpu-percentage" class="field">`cpu_percentage`</a> <span class="type">Integer</span>
Service が保つべき平均 CPU 使用率を指定し、それによってスケールアップ・ダウンします。

<span class="parent-field">count.</span><a id="count-memory-percentage" href="#count-memory-percentage" class="field">`memory_percentage`</a> <span class="type">Integer</span>
Service が保つべき平均メモリ使用率を指定し、それによってスケールアップ・ダウンします。

<span class="parent-field">count.</span><a id="requests" href="#count-requests" class="field">`requests`</a> <span class="type">Integer</span>
タスクで処理されるリクエスト数に応じて、スケールアップ・ダウンします。

<span class="parent-field">count.</span><a id="response-time" href="#count-response-time" class="field">`response_time`</a> <span class="type">Duration</span>
Service の平均レスポンス時間に応じて、スケールアップ・ダウンします。

{% include 'exec.ja.md' %}

{% include 'deployment.ja.md' %}

```yaml
deployment:
  rollback_alarms:
    cpu_utilization: 70    // Percentage value at or above which alarm is triggered.
    memory_utilization: 50 // Percentage value at or above which alarm is triggered.
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
