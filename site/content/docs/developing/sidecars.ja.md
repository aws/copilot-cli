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

## 実行例

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

以下は Service とサイドカーコンテナ両方で EFS ボリュームを用いる Manifest の一部です。

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
サイドカーパターンは Copilot であらかじめ定義されたサイドカーの構成です。現在サポートされているパターンは FireLens だけですが将来的にさらにパターンを追加していく予定です！

``` yaml
logging:
  # Fluent Bitのイメージ (オプション。デフォルトでは "public.ecr.aws/aws-observability/aws-for-fluent-bit:latest" を使用)
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

FireLens がログを転送するためにタスクロールに対して必要なアクセス許可を追加で与える必要があるかもしれません。[Addon](../developing/additional-aws-resources.ja.md) のなかで許可を追加できます。例えば以下のように設定できます。

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
          Resource: "<resource ARN>"
Outputs:
  FireLensPolicyArn:
    Description: An addon ManagedPolicy gets used by the ECS task role
    Value: !Ref FireLensPolicy
```

!!!info
    FireLens ログドライバーは主となるコンテナのログを様々な宛先へルーティングできる一方で、 [`svc logs`](../commands/svc-logs.ja.md) コマンドは CloudWatch Logs で Copilot Service のために作成したロググループに送信された場合のみログをトラックできます。

!!!info
    ** この機能をより簡単かつパワフルにする予定です！**現時点ではサイドカーはリモートイメージの利用のみをサポートしており、ユーザーはローカルのサイドカーイメージをビルドしてプッシュする必要があります。しかしローカルのイメージや Dockerfile をサポートする予定です。さらに FireLens 自身については主となるコンテナだけでなく他のサイドカーのログもルーティングできるようになる予定です。
