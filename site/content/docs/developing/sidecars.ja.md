# サイドカー
サイドカーは主となるコンテナと共に実行され補助的な役割を担うコンテナのことです。サイドカーの役割はロギングやコンフィグレーション、プロキシリクエストの処理などの周辺的なタスクを実行することです。

AWS はまた ECS サービスとシームレスに統合できるいくつかのプラグインを提供しており、[Firelens](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/using_firelens.html)や[AWS X-Ray](https://aws.amazon.com/xray/)、[AWS App Mesh](https://aws.amazon.com/app-mesh/)など多岐に渡ります。

マニフェストの中で[`storage` フィールド](../developing/storage.md)を使って主となるコンテナ用の EFS ボリュームを定義した場合、定義した任意のサイドカーコンテナはそのボリュームをマウントできます。

## Copilot でサイドカーを追加するには？
Copilot のマニフェストでサイドカーを追加するには2つの方法があります：[一般的なサイドカー](#general-sidecars)を指定する方法または[サイドカーパターン](#sidecar-patterns)を使用する方法です。

### 一般的なサイドカー
サイドカーイメージの URL を指定する必要があります。オプションで公開するポートや[プライベートレジストリ](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/private-auth.html)の認証パラメータを指定できます。

``` yaml
sidecars:
  {{ sidecar name }}:
    # 公開するポート (オプション)
    port: {{ port number }}
    # サイドカーイメージの URL (必須)
    image: {{ image url }}
    # プライベートレジストリの認証情報を保存しているシークレットの ARN (オプション)
    credentialsParameter: {{ credential }}
    # サイドカーコンテナの環境変数 (オプション)
    variables: {{ env var }}
    # サイドカーコンテナで用いるシークレット (オプション)
    secrets: {{ secret }}
    # サービスレベルで指定する EFS ボリュームのマウントパス (オプション)
    mount_points:
      - # サイドカーからマウントするときのソースボリューム (必須)
        source_volume: {{ named volume }}
        # サイドカーからボリュームをマウントするときのパス (必須)
        path: {{ path }}
        # サイドカーにボリュームに対する読み込みのみを許可するかどうか (デフォルトでは true)
        read_only: {{ bool }}
```

以下は load balanced web service のマニフェストで[nginx](https://www.nginx.com/)サイドカーコンテナを指定する例。

``` yaml
name: api
type: Load Balanced Web Service

image:
  build: api/Dockerfile
  port: 3000

http:
  path: 'api'
  healthcheck: '/api/health-check'
  # ロードバランサーのターゲットコンテナは service のコンテナの代わりにサイドカーの'nginx'を指定しています。
  targetContainer: 'nginx'

cpu: 256
memory: 512
count: 1

sidecars:
  nginx:
    port: 80
    image: 1234567890.dkr.ecr.us-west-2.amazonaws.com/reverse-proxy:revision_1
    variables:
      NGINX_PORT: 80
```

以下は Service とサイドカーコンテナ両方で EFS ボリュームを用いるマニフェストの一部です。

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

### サイドカーパターン
サイドカーパターンは Copilot であらかじめ定義されたサイドカーの構成です。現在サポートされているパターンは Firelens だけですが将来的にさらにパターンを追加していく予定です！

``` yaml
logging:
  # Fulent Bitのイメージ (オプション。デフォルトでは "amazon/aws-for-fluent-bit:latest" を使用)
  image: {{ image URL }}
  # Firelens ログドライバーにログを送信するときの設定 (オプション)
  destination:
    {{ config key }}: {{ config value }}
  # ログに ECS メタデータを含むかどうか (オプション。デフォルトでは true )
  enableMetadata: {{ true|false }}
  # ログの設定に渡すシークレット (オプション)
  secretOptions:
    {{ key }}: {{ value }}
  # カスタムの Fluent Bit イメージのなかで設定ファイルへのフルパス
  configFilePath: {{ config file path }}
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

Firelens がデータをフォワードするためにタスクロールに対して必要な許可を追加で与える必要があるかもしれません。[アドオン](../developing/additional-aws-resources.md)のなかで許可を追加できます。例えば以下のように設定できます。

``` yaml
Resources:
  FireLensPolicy:
    Type: AWS::IAM::ManagedPolicy
    Properties:
      PolicyDocument:
        Version: 2012-10-17
        Statement:
        - Effect: Allow
          Action:
          - logs:CreateLogStream
          - logs:CreateLogGroup
          - logs:DescribeLogStreams
          - logs:PutLogEvents
          Resource: "{{ resource ARN }}"
Outputs:
  FireLensPolicyArn:
    Description: An addon ManagedPolicy gets used by the ECS task role
    Value: !Ref FireLensPolicy
```

!!!補足
    Firelens ログドライバーは主となるコンテナのログを様々な目的地にルーティングできるため、[`svc logs`](../commands/svc-logs.md)コマンドは CloudWatch で Copilot service のために作成したロググループに送信された場合のみログをトラックできます。

!!!補足
    ** この機能をより簡単かつ強力にする予定です！**現在サイドカーにはリモートイメージを用いることしかサポートしておらず、ユーザーはローカルのサイドカーイメージをビルドしてプッシュする必要があります。しかしローカルのイメージや Dockerfiles をサポートする予定です。さらに Firelens を使って主となるコンテナだけでなく他のサイドカーのログもルーティングできるようにする予定です。 