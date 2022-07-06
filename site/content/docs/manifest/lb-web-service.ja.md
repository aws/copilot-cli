以下は `'Load Balanced Web Service'` Manifest で利用できるすべてのプロパティのリストです。[Copilot Service の概念](../concepts/services.ja.md)説明のページも合わせてご覧ください。

???+ note "frontend Service のサンプル Manifest"

    ```yaml
        # Service 名はロググループや ECS サービスなどのリソースの命名に利用されます。
        name: frontend
        type: Load Balanced Web Service

        # Serviceのトラフィックを分散します。
        http:
          path: '/'
          healthcheck:
            path: '/_healthcheck'
            port: 8080
            success_codes: '200,301'
            healthy_threshold: 3
            unhealthy_threshold: 2
            interval: 15s
            timeout: 10s
            grace_period: 45s
          deregistration_delay: 5s
          stickiness: false
          allowed_source_ips: ["10.24.34.0/23"]
          alias: example.com

        nlb:
          port: 443/tls

        # コンテナと Service の構成
        image:
          build:
            dockerfile: ./frontend/Dockerfile
            context: ./frontend
          port: 80

        cpu: 256
        memory: 512
        count:
          range: 1-10
          cpu_percentage: 70
          memory_percentage: 80
          requests: 10000
          response_time: 2s
        exec: true

        variables:
          LOG_LEVEL: info
        env_file: log.env
        secrets:
          GITHUB_TOKEN: GITHUB_TOKEN

        # 上記すべての値は Environment ごとにオーバーライド可能です。
        environments:
          test:
            count:
              range:
                min: 1
                max: 10
                spot_from: 2
          staging:
            count:
              spot: 2
          production:
            count: 2
    ```

<a id="name" href="#name" class="field">`name`</a> <span class="type">String</span>  
Service の名前。

<div class="separator"></div>

<a id="type" href="#type" class="field">`type`</a> <span class="type">String</span>  
Service のアーキテクチャタイプ。 [Load Balanced Web Service](../concepts/services.ja.md#load-balanced-web-service) は、ロードバランサー及び AWS Fargate 上の Amazon ECS によって構成される、インターネットに公開するための Service です。

<div class="separator"></div>

<a id="http" href="#http" class="field">`http`</a> <span class="type">Boolean or Map</span>
http セクションは Application Load Balancer と Service との連携に関するパラメータを含みます。

Application Load Balancer を無効化する場合は、 `http: false` と指定します。 Load Balanced Web Service では、Application Load Balancer または、Network Load Balancer が少なくとも 1 つ有効となっていなければならない事に注意してください。 

<span class="parent-field">http.</span><a id="http-path" href="#http-path" class="field">`path`</a> <span class="type">String</span>  
このパスに到着したリクエストが、Service に転送されます。各 Load Balanced Web Service は一意のパスでリクエストを受け付ける必要があります。

{% include 'http-healthcheck.ja.md' %}

<span class="parent-field">http.</span><a id="http-deregistration-delay" href="#http-deregistration-delay" class="field">`deregistration_delay`</a> <span class="type">Duration</span>  

登録解除時にターゲットがクライアントとの接続を閉じるのを待つ時間を指定します。デフォルトでは 60 秒です。この値を大きくするとターゲットが安全に接続を閉じるための時間を確保できますが、新バージョンのデプロイに必要となる時間が長くなります。範囲は 0 〜 3600 です。

<span class="parent-field">http.</span><a id="http-target-container" href="#http-target-container" class="field">`target_container`</a> <span class="type">String</span>  
サイドカーコンテナを指定することで、Service のメインコンテナの代わりにサイドカーでロードバランサーからのリクエストを受け取れます。

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
ID of your existing hosted zone; must be used with `http.alias`. If you have an environment with imported certificates, you can specify the hosted zone into which Copilot should insert the A record once the load balancer is created.
既存のプライベートホストゾーンの ID。`http.alias` と共に使用します。証明書をインポートした Environment がある場合、ロードバランサーの作成後に Copilot が A レコードを挿入するホストゾーンを指定できます。
```yaml
http:
  alias: example.com
  hosted_zone: Z0873220N255IR3MTNR4
# Also see http.alias array of maps example, above.
```
<span class="parent-field">http.</span><a id="http-version" href="#http-version" class="field">`version`</a> <span class="type">String</span>  
HTTP(S) プロトコルのバージョン。 `'grpc'`、 `'http1'`、または `'http2'` を指定します。省略した場合は、`'http1'` が利用されます。 
gRPC を利用する場合は、Application にドメインが関連付けられていなければなりません。

{% include 'nlb.ja.md' %}

{% include 'image-config-with-port.ja.md' %}

{% include 'image-healthcheck.ja.md' %}

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
  memory_percentage: 80
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

<span class="parent-field">count.</span><a id="requests" href="#count-requests" class="field">`requests`</a> <span class="type">Integer</span>
タスク当たりのリクエスト数を指定し、それによってスケールアップ・ダウンします。

<span class="parent-field">count.</span><a id="response-time" href="#count-response-time" class="field">`response_time`</a> <span class="type">Duration</span>
Service の平均レスポンスタイムを指定し、それによってスケールアップ・ダウンします。

{% include 'exec.ja.md' %}

{% include 'deployment.ja.md' %}

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
