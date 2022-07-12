# 内部 Application Load Balancers

デフォルトでは、 Load Balanced Web Service 用に作成された Environment 内の ALB は [インターネットからアクセス可能](https://docs.aws.amazon.com/elasticloadbalancing/latest/classic/elb-internet-facing-load-balancers.html)です。プライベート IP アドレスのみを利用する[内部ロードバランサー](https://docs.aws.amazon.com/elasticloadbalancing/latest/classic/elb-internal-load-balancers.html)を作るためには、Environment とワークロードを開始する時に、幾つかの設定が必要です。

## Environment

内部ロードバランサーは Environment レベルのリソースです。許可された Service 間で共有されます。`copilot env init` を実行すると、ALB をサポートする為に、いつくかの特定のリソースをインポートできます。 `https` を利用している Service に対しては、 [`--import-cert-arns`](../docs/commands/env-init.ja.md#what-are-the-flags)フラグを使って、既存のプライベート証明書の ARN をインポートしてください。Environment の VPC 内から入力トラフィックを ALB が受け付ける様にしたい場合は、[`--internal-alb-allow-vpc-ingress`](../docs/commands/env-init.jaf.md#what-are-the-flags) フラグを利用します ; そうしない場合、デフォルトでは、内部 ALB へのアクセスは、Environment 内に Copilot が作成した Service のみに限定されます。

!!!info
    現時点では、Copilot は Environment で利用している VPC にパブリックサブネットが *無い場合* にインポートした証明書を内部 ALB と関連づけます。そうでなければ、デフォルトのインターネットからアクセス可能なロードバランサーに関連づけられます。Environment を初期化する場合に、 `--import-vpc-id` と `--import-private-subnets` フラグを利用して、VPC とサブネットの ID を `--import-cert-arns` と共に設定できます。より管理しやすい様に、`copilot env init` を実行する時に `--import-cert-arns` フラグのみを使用し、プロンプトに従って既存の VPC リソースのインポートも可能です。パブリックサブネットのインポートは省略されます。(Copilot の env config は近日中に、より柔軟になる予定です。お楽しみに！)

## Service

内部ロードバランサーの背後に設置できる唯一の Service タイプは[Backend Service](../docs/concepts/services.ja.md#backend-service)です。Service をデプロイした Environment で ALB を作成する様に Copilot に指示をする為に、`http` フィールドを Backend Service ワークロードの Manifest に追加してください。

```yaml
# in copilot/{service name}/manifest.yml
http:
  path: '/'
network:
  vpc:
    placement: private
```

!!!attention
    現時点では、まだデプロイされていない新しい Backend Service を利用する必要があります。近いうちに、内部ロードバランサーを既存の Backend Service に追加できる様になります！

## 高度な設定

### サブネット配置
内部 ALB を配置するプライベートサブネットを正確に指定できます。

`copilot env init` を実行する時に、[`--internal-alb-subnets`](../commands/env-init.ja.md#what-are-the-flags) フラグを利用し、ALB を配置したいサブネットの ID を指定します。

### エイリアス、ヘルスチェックなど
Backend Service で利用する `http` フィールドには、 Load Balanced Web Services の `http` フィールドで利用する全てのサブフィールドと機能があります。

``` yaml
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
```

`alias` では、1. 既存のプライベートホストゾーンを持ち込むか、2. デプロイ後に Copilot とは別に独自のエイリアスレコードを追加できます。1 つのエリアスを追加する場合:
```yaml
http:
  alias: example.com
  hosted_zone: HostedZoneID1
```
または、ホストゾーンを共有する複数のエイリアスの場合:
```yaml
http:
  alias: ["example.com", "www.example.com"]
  hosted_zone: HostedZoneID1
```

または、複数のエイリアスがあり、そのうちの幾つかは、トップレベルのホストゾーンを使う場合:
```yaml
http:
  hosted_zone: HostedZoneID1
  alias:
    - name: example.com
    - name: www.example.com
    - name: something-different.com
      hosted_zone: HostedZoneID2
```

