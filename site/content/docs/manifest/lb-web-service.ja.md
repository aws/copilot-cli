以下は `'Load Balanced Web Service'` Manifest で利用できるすべてのプロパティのリストです。[Copilot Service の概念](../concepts/services.ja.md)説明のページも合わせてご覧ください。

???+ note "インターネット向け Service のサンプル Manifest"

    === "Basic"

        ```yaml
        name: 'frontend'
        type: 'Load Balanced Web Service'

        image:
          build: './frontend/Dockerfile'
          port: 8080

        http:
          path: '/'
          healthcheck: '/_healthcheck'

        cpu: 256
        memory: 512
        count: 3
        exec: true

        variables:
          LOG_LEVEL: info
        secrets:
          GITHUB_TOKEN: GITHUB_TOKEN
          DB_SECRET:
            secretsmanager: '${COPILOT_APPLICATION_NAME}/${COPILOT_ENVIRONMENT_NAME}/mysql'
        ```

    === "With a domain"

        ```yaml
        name: 'frontend'
        type: 'Load Balanced Web Service'

        image:
          build: './frontend/Dockerfile'
          port: 8080

        http:
          path: '/'
          alias: 'example.com'

        environments:
          qa:
            http:
              alias: # 証明書インポート済みの "qa" Environment
                - name: 'qa.example.com'
                  hosted_zone: Z0873220N255IR3MTNR4
        ```

    === "Larger containers"

        ```yaml
        # 例えば、外部からのトラフィックを受け入れる前に、Java サービスをウォームアップしておきたい場合など。
        name: 'frontend'
        type: 'Load Balanced Web Service'

        image:
          build:
            dockerfile: './frontend/Dockerfile'
            context: './frontend'
          port: 80

        http:
          path: '/'
          healthcheck:
            path: '/_deephealthcheck'
            port: 8080
            success_codes: '200,301'
            healthy_threshold: 4
            unhealthy_threshold: 2
            interval: 15s
            timeout: 10s
            grace_period: 2m
          deregistration_delay: 50s
          stickiness: true
          allowed_source_ips: ["10.24.34.0/23"]

        cpu: 2048
        memory: 4096
        count: 3
        storage:
          ephemeral: 100

        network:
          vpc:
            placement: 'private'
        ```

    === "Autoscaling"

        ```yaml
        name: 'frontend'
        type: 'Load Balanced Web Service'

        http:
          path: '/'
        image:
          location: aws_account_id.dkr.ecr.us-west-2.amazonaws.com/frontend:latest
          port: 80

        cpu: 512
        memory: 1024
        count:
          range: 1-10 
          cooldown:
            in: 60s
            out: 30s
          cpu_percentage: 70
          requests: 30
          response_time: 2s
        ```

    === "Event-driven"

        ```yaml
        # https://aws.github.io/copilot-cli/docs/developing/publish-subscribe/ を参照してください。
        name: 'orders'
        type: 'Load Balanced Web Service'

        image:
          build: Dockerfile
          port: 80
        http:
          path: '/'
          alias: 'orders.example.com'

        variables:
          DDB_TABLE_NAME: 'orders'

        publish:
          topics:
            - name: 'products'
            - name: 'orders'
              fifo: true
        ```

    === "Network Load Balancer"

        ```yaml
        name: 'frontend'
        type: 'Load Balanced Web Service'

        image:
          build: Dockerfile
          port: 8080

        http: false
        nlb:
          alias: 'example.com'
          port: 80/tcp
          target_container: envoy

        network:
          vpc:
            placement: 'private'

        sidecars:
          envoy:
            port: 80
            image: aws_account_id.dkr.ecr.us-west-2.amazonaws.com/envoy:latest
        ```

    === "Shared file system"

        ```yaml
        # http://localhost:8000/copilot-cli/docs/developing/storage.ja.md#ファイルシステム を参照してください。
        name: 'frontend'
        type: 'Load Balanced Web Service'

        image:
          build: Dockerfile
          port: 80
          depends_on:
            bootstrap: success
        
        http:
          path: '/'

        storage:
          volumes:
            wp:
              path: /bitnami/wordpress
              read_only: false
              efs: true

        # ブートストラップコンテナを使って、ファイルシステム内のコンテンツを用意しておきます。
        sidecars:
          bootstrap:
            image: aws_account_id.dkr.ecr.us-west-2.amazonaws.com/bootstrap:v1.0.0
            essential: false
            mount_points:
              - source_volume: wp
                path: /bitnami/wordpress
                read_only: false
        ```

    === "End-to-end encryption"

        ```yaml
        name: 'frontend'
        type: 'Load Balanced Web Service'

        image:
          build: Dockerfile
          port: 8080

        http:
          alias: 'example.com'
          path: '/'
          healthcheck:
            path: '/_health'

          # プロトコルでの指定の結果により、envoy コンテナのポートは 443 です。ヘルチェックプロトコルは `HTTPS` になります。
          # ロードバランサーは Fargate タスクと TLS 接続します。envoy コンテナに
          # インストールした証明書が利用されます。それらの証明書は自己証明書です。
          target_container: envoy

        sidecars:
          envoy:
            port: 443
            image: aws_account_id.dkr.ecr.us-west-2.amazonaws.com/envoy-proxy-with-selfsigned-certs:v1

        network:
          vpc:
            placement: 'private'
        ```

    === "Expose Multiple Ports"

        ```yaml
        name: 'frontend'
        type: 'Load Balanced Web Service'

        image:
          build: './frontend/Dockerfile'
          port: 8080

        nlb:
          port: 8080/tcp              # Traffic on port 8080/tcp is forwarded to the main container, on port 8080.
          additional_listeners:  
            - port: 8084/tcp          # Traffic on port 8084/tcp is forwarded to the main container, on port 8084.
            - port: 8085/tcp          # Traffic on port 8085/tcp is forwarded to the sidecar "envoy", on port 3000.
              target_port: 3000         
              target_container: envoy   

        http:
          path: '/'
          target_port: 8083           # Traffic on "/" is forwarded to the main container, on port 8083. 
          additional_rules: 
            - path: 'customerdb'
              target_port: 8081       # Traffic on "/customerdb" is forwarded to the main container, on port 8083.  
            - path: 'admin'
              target_port: 8082       # Traffic on "/admin" is forwarded to the sidecar "envoy", on port 8082.
              target_container: envoy    

        sidecars:
          envoy:
            port: 80
            image: aws_account_id.dkr.ecr.us-west-2.amazonaws.com/envoy-proxy-with-selfsigned-certs:v1
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
このパスに到着したリクエストが、Service に転送されます。各リスナールールは一意のパスでリクエストを受け付ける必要があります。

<span class="parent-field">http.</span><a id="http-alb" href="#http-alb" class="field">`alb`</a> <span class="type">String</span> <span class="version">[v1.32.0](../../blogs/release-v132.ja.md#imported-albs) にて追加</span>  
インポートする既存のインターネット向け ALB の ARN または ALB 名。リスナーにリスナールールが追加されます。Copilot は証明書などの DNS 関連リソースを管理しません。

{% include 'http-healthcheck.ja.md' %}

<span class="parent-field">http.</span><a id="http-deregistration-delay" href="#http-deregistration-delay" class="field">`deregistration_delay`</a> <span class="type">Duration</span>  

登録解除時にターゲットがクライアントとの接続を閉じるのを待つ時間を指定します。デフォルトでは 60 秒です。この値を大きくするとターゲットが安全に接続を閉じるための時間を確保できますが、新バージョンのデプロイに必要となる時間が長くなります。範囲は 0 〜 3600 です。

<span class="parent-field">http.</span><a id="http-target-container" href="#http-target-container" class="field">`target_container`</a> <span class="type">String</span>  
サイドカーコンテナを指定することで、Service のメインコンテナの代わりにサイドカーでロードバランサーからのリクエストを受け取れます。
ターゲットコンテナのポートが `443` に設定されている場合、プロトコルは `HTTP` に設定され、ロードバランサーは
Fargate タスクと TLS 接続します。ターゲットコンテナにインストールされた証明書が利用されます。

<span class="parent-field">http.</span><a id="http-target-port" href="#http-target-port" class="field">`target_port`</a> <span class="type">String</span>  
任意項目。トラフィックを受信するコンテナポート。デフォルトでは、ターゲットコンテナがメインコンテナの場合、`image.port` 、
ターゲットコンテナがサイドカーの場合は、`sidecars.<name>.port` です。

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
既存のプライベートホストゾーンの ID。`http.alias` とのみ使用可能です。証明書をインポートした Environment がある場合、ロードバランサーの作成後に Copilot が A レコードを挿入するホストゾーンを指定できます。
```yaml
http:
  alias: example.com
  hosted_zone: Z0873220N255IR3MTNR4
# Also see http.alias array of maps example, above.
```
<span class="parent-field">http.</span><a id="http-redirect-to-https" href="#http-redirect-to-https" class="field">`redirect_to_https`</a> <span class="type">Boolean</span>
Application Load Balancer で、HTTP から HTTPS へ自動的にリダイレクトします。デフォルトは `true` です。

<span class="parent-field">http.</span><a id="http-version" href="#http-version" class="field">`version`</a> <span class="type">String</span>  
HTTP(S) プロトコルのバージョン。 `'grpc'`、 `'http1'`、または `'http2'` を指定します。省略した場合は、`'http1'` が利用されます。 
gRPC を利用する場合は、Application にドメインが関連付けられていなければなりません。

<span class="parent-field">http.</span><a id="http-additional-rules" href="#http-additional-rules" class="field">`additional_rules`</a> <span class="type">Array of Maps</span>  
複数の ALB リスナールール設定します。

{% include 'http-additionalrules.en.md' %}

{% include 'nlb.ja.md' %}

{% include 'image-config-with-port.ja.md' %}
ポートを `443` に設定し、 ロードバランサーが `http` で有効化されている場合、プロトコルは `HTTPS` に設定され、ロードバランサーは Fargate タスクと TLS 接続します。ターゲットコンテナにインストールされた証明書が利用されます。

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
    out: 60s
  cpu_percentage: 70
  memory_percentage:
    value: 80
    cooldown:
      in: 80s
      out: 160s
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
Service をスケールアップするためのオートスケーリングのクールダウン時間。

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

<span class="parent-field">count.</span><a id="count-cpu-percentage" href="#count-cpu-percentage" class="field">`cpu_percentage`</a> <span class="type">Integer or Map</span>
Service が保つべき平均 CPU 使用率を指定し、それによってスケールアップ・ダウンします。

<span class="parent-field">count.</span><a id="count-memory-percentage" href="#count-memory-percentage" class="field">`memory_percentage`</a> <span class="type">Integer</span>
Service が保つべき平均メモリ使用率を指定し、それによってスケールアップ・ダウンします。

<span class="parent-field">count.</span><a id="requests" href="#count-requests" class="field">`requests`</a> <span class="type">Integer</span>
タスク当たりのリクエスト数を指定し、それによってスケールアップ・ダウンします。

<span class="parent-field">count.</span><a id="response-time" href="#count-response-time" class="field">`response_time`</a> <span class="type">Duration</span>
Service の平均レスポンスタイムを指定し、それによってスケールアップ・ダウンします。

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
