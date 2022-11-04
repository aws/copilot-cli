---
title: 'AWS Copilot v1.22: IAM パーミッションバウンダリーなどを試してみよう！'
twitter_title: 'AWS Copilot v1.22'
image: ''
image_alt: ''
image_width: '1051'
image_height: '747'
---

# AWS Copilot v1.22: IAM パーミッションバウンダリーなどを試してみよう！

投稿日: 2022 年 9 月 27 日

AWS Copilot コアチームは Copilot v1.22 リリースを発表します。
このリリースに貢献してくれた [@jterry75](https://github.com/jterry75)、 [@gabrielcostasilva](https://github.com/gabrielcostasilva)、 [@shingos](https://github.com/shingos)、[@hkford](https://github.com/hkford) に特別な感謝を捧げます。
私たちのパブリックな[コミュニティチャット](https://gitter.im/aws/copilot-cli)は成長しており、オンラインでは 300 人以上、[GitHub](http://github.com/aws/copilot-cli/) では 2.4k 以上のスターを獲得しています。
AWS Copilot へご支援、ご支持いただいている皆様お一人お一人に感謝をいたします。

Copilot v1.22 では、いくつかの新機能と改良が施されています。

- **IAM ロールパーミッションバウンダリー**: [詳細はこちらをご覧ください](#iam-role-permissions-boundary).
- **FIFO SNS/SQS**: [詳細はこちらをご覧ください](#fifo-snssqs).
- **CloudFront TLS ターミネーション**: CloudFront を利用してより高速な TLS の終端が可能になりました![詳細はこちらをご覧ください](#cloudfront-tls-termination).
- **Application Load Balancer と Fargate タスク間の TLS接続**: ターゲットコンテナのポートが `443` と指定されている場合に、Copilot はターゲットグループのプロトコルとヘルスチェックプロトコルを HTTPS に設定します。[Manifest のサンプルをご覧ください](../docs/manifest/lb-web-service.ja.md#__tabbed_1_8)

???+ note "AWS Copilot とは?"

    AWS Copilot CLI は AWS 上でプロダクションレディなコンテナ化されたアプリケーションのビルド、リリース、そして運用のためのツールです。
    開発のスタートからステージング環境へのプッシュ、本番環境へのリリースまで、Copilot はアプリケーション開発ライフサイクル全体の管理を容易にします。
    Copilot の基礎となるのは、 AWS CloudFormation です。CloudFormation により、インフラストラクチャを 1 回の操作でコードとしてプロビジョニングできます。
    Copilot は、さまざまなタイプのマイクロサービスの作成と運用の為に、事前定義された CloudFormation テンプレートと、ユーザーフレンドリーなワークフローを提供します。
    デプロイメントスクリプトを記述する代わりに、アプリケーションの開発に集中できます。

    より詳細な AWS Copilot の紹介については、[Overview](../docs/concepts/overview.ja.md) を確認してください。

<a id="iam-role-permissions-boundary"></a>
## IAM ロールパーミッションバウンダリー
IAM ロールの作成時に、パーミッションバウンダリーを必要とする AWS Organizations サービスコントロールポリシーが適用されている場合や、単に Application にいくつかのガードレールを追加したい場合に、Copilot がお役に立ちます。  `--permissions-boundary` フラグを使い、`copilot app init` コマンドを実行すると、既存の IAM ポリシー名を指定できます。指定したポリシーは Copilot が作成する(アプリケーション内)全ての IAM ロールに対してパーミッションバウンダリーとして付加されます。

パーミッションバウンダリーの名前を指定して、アプリケーションを初期化したい場合、次の様に指定します。  
```console
copilot app init --permissions-boundary examplePermissionsBoundaryPolicy
```
パーミッションバウンダリーは、アプリケーション内で作成される全ての IAM ロールに付加されます。
```yaml
ExampleIAMRole:
  Type: AWS::IAM::Role
  Properties:
    PermissionsBoundary: 'arn:aws:iam::123456789012:policy/examplePermissionsBoundaryPolicy'
```

## FIFO SNS/SQS
パブリッシュ/サブスクライブアーキテクチャでの、厳密なメッセージ順序と、メッセージ重複排除の為に、SNS FIFO トピックと SQS FIFO キューを利用できます。

### Manifest を構成して、Service に対して、SNS FIFO トピックを設定します。

Service の Manifest の `publish.topics` 配下に、次の様に `fifo: true` を指定すると、 Copilot は SNS FIFO トピックを作成します。

```yaml
publish:
  topics:
    - name: mytopic
      fifo: true
```

また、高度な SNS FIFO トピック設定として、次の様に指定します。
```yaml
publish:
  topics:
    - name: mytopic
      fifo:
        content_based_deduplication: true
```

FIFO トピックに関する詳細な仕様については、[Manifest 仕様](../docs/include/publish.ja.md#publish-topics-topic-fifo)をご覧ください。

### Worker Service でのSQS FIFO キュー
Worker Service の Manifest において、 `subscribe.topics.queue` または `subscribe.queue` 配下に、次の様に `fifo: true` と指定します。 Copilot は FIFO SQS キューとサブスクリプションを作成します。

```yaml
subscribe:
  topics:
    - name: mytopic
      service: myservice
      queue: 
        fifo: true # topics specific SQS FIFO queue
  queue:
    fifo: true # Configure the default SQS queue to be FIFO.
```
または、高度な FIFO SQS キュー設定として、次の様に指定します。

```yaml
subscribe:
  topics:
    - name: mytopic
      service: myservice
      queue:
        fifo:
          content_based_deduplication: true
          deduplication_scope: messageGroup
          throughput_limit: perMessageGroupId
  queue:
    fifo:
      high_throughput: true
```
FIFO キューに関する詳細な仕様については、[Manifest 仕様](../docs/manifest/worker-service.ja.md#subscribe-queue-fifo)をご覧ください。

<a id="cloudfront-tls-termination"></a>
## CloudFront TLS ターミネーション

Load Balance Web Service (LBWS) にて、CloudFront で TLS を終端する様に、Environment Manifest を設定します。

```yaml
cdn:
  terminate_tls: true
```

CloudFront を TLS ターミネーションとして利用する上記の設定は、`CF → ALB → ECS` 間で HTTP のみになる事を意味します。CloudFront のエッジは、通常、閲覧者と地理的に近い為、高速な TLS の終端が行え、閲覧者のページ読み込みが短くなります。

しかし、 Service が HTTPS を有効化している場合(Application にドメインが設定されている、Env に証明書をインポートしている)、[Load Balanced Web Service Manifest](../docs/manifest/lb-web-service.ja.md)を修正し、 ALB の http リダイレクトを off にする必要があります。

```yaml
http:
  redirect_to_https: false
```

CloudFront TLS　ターミネーションを有効化するには `env deploy` を実行する前に、`svc deploy` を利用してサービスを再デプロイします。

## 次は?

以下のリンクより、新しい Copilot CLI バージョンをダウンロードし、[GitHub](https://github.com/aws/copilot-cli/) や [コミュニティチャット](https://gitter.im/aws/copilot-cli)にフィードバックを残してください。

* [最新 CLI バージョン](../docs/getting-started/install.ja.md)のダウンロード
* [スタートガイド](../docs/getting-started/first-app-tutorial.ja.md)を試す
* [GitHub](https://github.com/aws/copilot-cli/releases/tag/v1.22.0) でリリースノートの全文を読む

今回のリリースの翻訳はソリューションアーキテクトの浅野が担当しました。