# AWS Copilot v1.19: 内部ロードバランサー, サブネット配置指定など

投稿日: 2022 年 6 月 13 日

The AWS Copilot コアチームは v1.19 リリースを発表できることを嬉しく思います！
このリリースに貢献した[@gautam-nutalapati](https://github.com/gautam-nutalapati) と [@jonstacks](https://github.com/jonstacks)に心から感謝します。
私たちのパブリック[コミュニティチャット](https://gitter.im/aws/copilot-cli)は成長していて、300 人近い方がオンラインで、
日々助け合ってます。AWS Copilot に関心とサポートを示してくださった皆様に感謝しています。

Copilot v1.19 では新機能の追加と、いくつかの改良が行われました:

* **Backend Services用のロードバランサー:** **内部向け** に Application Load Balancer を追加することが出来る様になりました。(Load Balanced Web Service 用に作成する様な `internet-facing` とは対照的に)。 [詳しくはこちら](./#internal-load-balancers).
* **サブネット配置指定**:
ECS タスクの起動場所をより細かく制御出来る様になりました。サブネットの配置を `public` と `private` に加えて、特定のサブネットを Copilot に指定出来る様になりました。希望するサブネットの ID をワークロード Manifest に追加するだけで指定できます。
```yaml
# in copilot/{service name}/manifest.yml
network:
  vpc:
    placement:
      subnets: ["SubnetID1", "SubnetID2"]
```
* **ホストゾーン-Aレコード管理**:
Service Manifest に、エイリアスと共にホストゾーンの ID を記載出来る様になりました。インポートした証明書を使用する Environment へのデプロイ時に Copilot が A レコードを追加します。([#3608](https://github.com/aws/copilot-cli/pull/3608), [#3643](https://github.com/aws/copilot-cli/pull/3643))
```yaml
# single alias and hosted zone
http:
  alias: example.com
  hosted_zone: HostedZoneID1

# multiple aliases that share a hosted zone
http:
  alias: ["example.com", "www.example.com"]
  hosted_zone: HostedZoneID1

# multiple aliases, some of which use the top-level hosted zone
http:
  hosted_zone: HostedZoneID1
  alias:
    - name: example.com
    - name: www.example.com
    - name: something-different.com
      hosted_zone: HostedZoneID2
```
* **プライベートルートテーブルへのアクセス**:
Copilot は CloudFormation スタックからプライベートルートテーブルの ID をエクスポートする様になりました。[Addon](../docs/developing/addons/workload.ja.md) を利用して VPC ゲートウェイエンドポイントを作成する時に利用します。([#3611](https://github.com/aws/copilot-cli/pull/3611))
* **ターゲットグループのヘルスチェックに利用する `port`**:
新しい `port` フィールドにより、ヘルスチェックのために、ロードバランサーからのリクエストに利用するポートとは異なる、デフォルトではないポートを設定することが出来る様になりました。
([#3548](https://github.com/aws/copilot-cli/pull/3548))
```yaml
http:
  path: '/'
  healthcheck:
    port: 8080
```

* **バグフィックス:**
    * Application から Service が削除された場合に、`app init --resource-tags` で適用されたタグを保持します([#3582](https://github.com/aws/copilot-cli/pull/3582))。
    * Network Load Balancer を利用した Load Balanced Web Service のオートスケーリングフィールドを有効化する際の不具合を修正しました([#3578](https://github.com/aws/copilot-cli/pull/3578))。
    * Fargate Windows タスクに対する `copilot svc exec` を有効にしました([#3566](https://github.com/aws/copilot-cli/pull/3566))。

このリリースには互換性を破る変更はありません。

## Copilotとは？

AWS Copilot CLI は AWS 上でプロダクションレディなコンテナ化されたアプリケーションのビルド、リリース、そして運用のためのツールです。
開発のスタートからステージング環境へのプッシュ、本番環境へのリリースまで、Copilot はアプリケーション開発ライフサイクル全体の管理を容易にします。
Copilot の基礎となるのは、 AWS CloudFormation です。CloudFormation により、インフラストラクチャを 1 回の操作でコードとしてプロビジョニングできます。
Copilot は、さまざまなタイプのマイクロサービスの作成と運用の為に、事前定義された CloudFormation テンプレートと、ユーザーフレンドリーなワークフローを提供します。デプロイメントスクリプトを記述する代わりに、アプリケーションの開発に集中できます。

より詳細な AWS Copilot の紹介については、[Overview](../docs/concepts/overview.ja.md) を確認してください。

<a id="internal-load-balancers"></a>
## 内部ロードバランサー
_Contributed by [Janice Huang](https://github.com/huanjani) and [Danny Randall](https://github.com/dannyrandall)_  
Copilot の Environment とワークロードを開始する際にいくつかの設定し、[内部ロードバランサー](https://docs.aws.amazon.com/ja_jp/elasticloadbalancing/latest/classic/elb-internal-load-balancers.html)を作成することが出来る様になりました。内部ロードバランサーは、プライベート IP アドレスのみを利用します。

内部ロードバランサーは Environment レベルのリソースです。許可された Service 間で共有されます。`copilot env init` を実行すると、ALB をサポートする為に、いつくかの特定のリソースをインポートできます。 `https` を利用している Service に対しては、 [`--import-cert-arns`](../docs/commands/env-init.ja.md#what-are-the-flags)フラグを使って、既存のプライベート証明書の ARN をインポートしてください。現時点では、Copilot は Environment で利用している VPC にパブリックサブネットが *無い場合* にインポートした証明書を内部 ALB と関連づけます。つまりプライベートサブネットのみの場合にインポートします。Environment の VPC 内から入力トラフィックを ALB が受け付ける様にしたい場合は、[`--internal-alb-allow-vpc-ingress`](../docs/commands/env-init.ja.md#what-are-the-flags) フラグを利用します ; そうしない場合、デフォルトでは、内部 ALB へのアクセスは、Environment 内に Copilot が作成した Service のみに限定されます。

内部ロードバランサーの背後に設置できる唯一の Service タイプは[Backend Service](../docs/concepts/services.ja.md#backend-service)です。Service をデプロイした Environment で ALB を作成する様に Copilot に指示をする為に、`http` フィールドを Backend Service ワークロードの Manifest に追加してください。


```yaml
# in copilot/{service name}/manifest.yml
http:
  path: '/'
  network:
    vpc:
      placement: private
  # for https
  alias: example.aws
  hosted_zone: Z0873220N255IR3MTNR4
```
より詳細については、[内部 ALB](../docs/developing/internal-albs.ja.md)のドキュメントを確認してください!

## 次は？

以下のリンクより、新しい Copilot CLI バージョンをダウンロードし、[GitHub](https://github.com/aws/copilot-cli/) や [コミュニティチャット](https://gitter.im/aws/copilot-cli)に
フィードバックを残してください。

* [最新 CLI バージョン](../docs/getting-started/install.ja.md)のダウンロード
* [スタートガイド](../docs/getting-started/first-app-tutorial.ja.md)を試す
* [GitHub](https://github.com/aws/copilot-cli/releases/tag/v1.19.0) でリリースノートの全文を読む

今回のリリースの翻訳はソリューションアーキテクトの浅野が担当しました。

