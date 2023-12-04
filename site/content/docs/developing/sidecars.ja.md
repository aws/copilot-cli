# サイドカー
サイドカーは主となるコンテナと共に実行され補助的な役割を担うコンテナのことです。サイドカーの役割はロギングや設定ファイルの取得、リクエストのプロキシ処理などの周辺的なタスクを実行することです。

!!! attention
    Request-Driven Web Service はサイドカーの利用をサポートしていません。

!!! Attention
    メインコンテナに Windows イメージを使用している場合、[FireLens](https://docs.aws.amazon.com/ja_jp/AmazonECS/latest/developerguide/using_firelens.html), [AWS X-Ray](https://aws.amazon.com/jp/xray/), [AWS App Mesh](https://aws.amazon.com/jp/app-mesh/) はサポートされていません。利用しようとしているサイドカーコンテナが Windows 環境での実行をサポートしているか確認してください。

AWS はまた ECS サービスとシームレスに組み合わせられるいくつかのプラグインを提供しており、[FireLens](https://docs.aws.amazon.com/ja_jp/AmazonECS/latest/developerguide/using_firelens.html) や [AWS X-Ray](https://aws.amazon.com/jp/xray/)、[AWS App Mesh](https://aws.amazon.com/jp/app-mesh/) など多岐に渡ります。

Manifest の中で [`storage` フィールド](../developing/storage.ja.md)を使って主となるコンテナ用の EFS ボリュームを定義した場合、定義した任意のサイドカーコンテナはそのボリュームをマウントできます。

## Copilot でサイドカーを追加するには？
Copilot の Manifest でサイドカーを追加したい場合、[サイドカーコンテナを直接定義する](#サイドカーコンテナを直接定義する)あるいは[サイドカーパターン](#サイドカーパターン)を利用する方法があります。

### サイドカーコンテナを直接定義する
サイドカーコンテナイメージの URL を指定する必要があります。オプションで公開するポートや[プライベートレジストリ](https://docs.aws.amazon.com/ja_jp/AmazonECS/latest/developerguide/private-auth.html)の認証パラメータを指定できます。

{% include 'sidecar-config.ja.md' %}

<div class="separator"></div>

#### 実行例

##### サイドカーでの Environment オーバーライド
他の Service / Job の Manifest と同様に、サイドカー設定も [`environments`](../manifest/lb-web-service.ja.md#environments) フィールドを利用して環境ごとにオーバーライドできます。
次の例では、`datadog` サイドカーの環境変数 `DD_APM_ENABLED` の値を、`dev` Environment かどうかによって設定しています。

```yaml
name: api
type: Load Balanced Web Service

sidecars:
  datadog:
    port: 80
    image:
      build: src/reverseproxy/Dockerfile
    variables:
      DD_APM_ENABLED: true

environments:
  dev:
    sidecars:
      datadog:
        variables:
          DD_APM_ENABLED: false
```

##### [nginx](https://www.nginx.com/) サイドカーコンテナ
以下は Load Balanced Web Service の Manifest で [nginx](https://www.nginx.com/) サイドカーコンテナを指定する例です。

``` yaml
name: api
type: Load Balanced Web Service

image:
  build: api/Dockerfile
  port: 3000

http:
  path: 'api'
  healthcheck: '/api/health-check'
  # ロードバランサーのターゲットコンテナは Service のコンテナの代わりにサイドカーの'nginx'を指定しています。
  target_container: 'nginx'

cpu: 256
memory: 512
count: 1

sidecars:
  nginx:
    port: 80
    image:
      build: src/reverseproxy/Dockerfile
    variables:
      NGINX_PORT: 80
```

##### Service とサイドカーコンテナ両方で EFS ボリュームを利用

```yaml
storage:
  volumes:
    myEFSVolume:
      path: '/etc/mount1'
      read_only: false
      efs:
        id: fs-1234567

sidecars:
  nginx:
    port: 80
    image: 1234567890.dkr.ecr.us-west-2.amazonaws.com/reverse-proxy:revision_1
    variables:
      NGINX_PORT: 80
    mount_points:
      - source_volume: myEFSVolume
        path: '/etc/mount1'
```

##### [AWS Distro for OpenTelemetry](https://aws-otel.github.io/) サイドカー
以下は、[AWS Distro for OpenTelemetry](https://aws-otel.github.io/) のサイドカーをカスタム構成で実行した例です。このカスタム構成例では、X-Ray トレースデータを収集するだけでなく、ECS メトリクスをサードパーティに送信することができます。この例では、SSM シークレットと追加の IAM 権限が必要になります。

OpenTelemetry サイドカーを使用するには、まず有効な[設定ファイル](https://opentelemetry.io/docs/collector/configuration/)を作成します。次に、設定ファイルのサイズを確認します。標準的なパラメータは [4KB に制限](https://docs.aws.amazon.com/ja_jp/systems-manager/latest/APIReference/API_PutParameter.html#systemsmanager-PutParameter-request-Value)されています。設定ファイルが 4K より大きい場合、高度な SSM パラメータを使用する必要があります。

高度なパラメータが必要な場合は、手動で作成してタグ付けする必要があります。設定が標準パラメータに収まっている場合は、[`secret init`](../commands/secret-init.ja.md) を使用して SSM シークレットを作成します。以下の YAML ドキュメントは、"YOUR-API-KEY-HERE" と書かれた API キーを更新した後、そのまま New Relic で使用することができます。

サンプル YAML では、空のキーが含まれているのは意図的なものです。サイドカーは、これらのキーに対してコレクターのデフォルトを使用します。


```yaml
receivers:
  awsxray:
    transport: udp
  awsecscontainermetrics:

processors:
  batch:

exporters:
  awsxray:
    region: us-west-2
  otlp:
    endpoint: otlp.nr-data.net:4317
    headers: 
      api-key: YOUR-API-KEY-HERE

service:
  pipelines:
    traces:
      receivers: [awsxray]
      processors: [batch]
      exporters: [awsxray]
    metrics:
      receivers: [awsecscontainermetrics]
      exporters: [otlp]
```

X-Ray トレースの書き込みには、以下のような追加の IAM 権限が必要です。[公開されているドキュメント](./addons/workload.ja.md)に従ってこれを Addon に含めてください。

``` yaml
Resources:
  XrayWritePolicy:
    Type: AWS::IAM::ManagedPolicy
    Properties:
      PolicyDocument:
        Version: '2012-10-17'
        Statement:
          - Sid: CopyOfAWSXRayDaemonWriteAccess
            Effect: Allow
            Action:
              - xray:PutTraceSegments
              - xray:PutTelemetryRecords
              - xray:GetSamplingRules
              - xray:GetSamplingTargets
              - xray:GetSamplingStatisticSummaries
            Resource: "*"

Outputs:
  XrayAccessPolicyArn:
    Description: "The ARN of the ManagedPolicy to attach to the task role."
    Value: !Ref XrayWritePolicy
```

OpenTelemetry コレクターの設定は、環境変数としてサイドカーに渡されます。

```yaml
sidecars:
  otel_sidecar:
    image: 'public.ecr.aws/aws-observability/aws-otel-collector:latest'
    secrets:
      AOT_CONFIG_CONTENT: /copilot/${COPILOT_APPLICATION_NAME}/${COPILOT_ENVIRONMENT_NAME}/secrets/otel_config
```

### サイドカーパターン
サイドカーパターンは Copilot であらかじめ定義されたサイドカーの構成です。現在サポートされているパターンは FireLens だけですが将来的にさらにパターンを追加していく予定です！

``` yaml
logging:
  # Fluent Bitのイメージ (オプション。デフォルトでは "public.ecr.aws/aws-observability/aws-for-fluent-bit:stable" を使用)
  image: <image URL>
  # Firelens ログドライバーにログを送信するときの設定 (オプション)
  destination:
    <config key>: <config value>
  # ログに ECS メタデータを含むかどうか (オプション。デフォルトでは true )
  enableMetadata: <true|false>
  # ログの設定に渡すシークレット (オプション)
  secretOptions:
    <key>: <value>
  # カスタムの Fluent Bit イメージ内の設定ファイルのフルパス
  configFilePath: <config file path>
  # サイドカーコンテナに対する環境変数 (オプション)
  variables:
    <key>: <value>
  # サイドカーコンテナに公開するシークレット (オプション)
  secrets:
    <key>: <value>
```
例えば以下のように設定できます。

``` yaml
logging:
  destination:
    Name: cloudwatch
    region: us-west-2
    log_group_name: /copilot/sidecar-test-hello
    log_stream_prefix: copilot/
```

FireLens がログを転送するためにタスクロールに対して必要なアクセス許可を追加で与える必要があるかもしれません。[Addon](./addons/workload.ja.md) のなかで許可を追加できます。例えば以下のように設定できます。

``` yaml
Resources:
  FireLensPolicy:
    Type: AWS::IAM::ManagedPolicy
    Properties:
      PolicyDocument:
        Version: '2012-10-17'
        Statement:
        - Effect: Allow
          Action:
          - logs:CreateLogStream
          - logs:CreateLogGroup
          - logs:DescribeLogStreams
          - logs:PutLogEvents
          Resource: "<resource ARN>"
Outputs:
  FireLensPolicyArn:
    Description: An addon ManagedPolicy gets used by the ECS task role
    Value: !Ref FireLensPolicy
```

!!!info
    FireLens ログドライバーは主となるコンテナのログを様々な宛先へルーティングできる一方で、 [`svc logs`](../commands/svc-logs.ja.md) コマンドは CloudWatch Logs で Copilot Service のために作成したロググループに送信された場合のみログをトラックできます。
