---
image: ''
image_alt: ''
image_height: '747'
image_width: '1051'
title: 'AWS Copilot v1.26: CloudWatch アラーム、環境アドオン用の「ストレージ初期化」、および RDWS シークレットサポートによるロールバックの自動化'
twitter_title: AWS Copilot v1.26
---

# AWS Copilot v1.26: CloudWatchアラームによるロールバックの自動化、envアドオン用 `storage init`、RDWSシークレットのサポート。


投稿日:2023年2月20日

AWS Copilot のコアチームは、Copilot v1.26 のリリースを発表しています。
私たちの公開されている [コミュニティチャット](https://gitter.im/aws/copilot-cli) は増え続けており、400 人以上がオンラインで、[GitHub](http://github.com/aws/copilot-cli/) には 26,000 以上のスターを獲得しています。
AWS Copilot へご支援、ご支持いただいている皆様お一人お一人に感謝をいたします。

Copilot v1.26には、いくつかの新機能と改善点があります。

*   **Service alarm-based rollback**: [詳細セクションはこちらをご覧ください。](#service-alarm-based-rollback)。
*   **storage init**: [詳細セクションはこちらをご覧ください。](#storage-init-for-environment-addons)。
*   **Request-Driven Web Service secrets support**: [詳細セクションはこちらをご覧ください。](#request-driven-web-service-secrets-support)。

???+ note "AWS Copilot とは?"
AWS Copilot CLI は AWS 上でプロダクションレディなコンテナ化されたアプリケーションのビルド、リリース、そして運用のためのツールです。
開発のスタートからステージング環境へのプッシュ、本番環境へのリリースまで、Copilot はアプリケーション開発ライフサイクル全体の管理を容易にします。
Copilot の基礎となるのは、 AWS CloudFormation です。CloudFormation により、インフラストラクチャを 1 回の操作でコードとしてプロビジョニングできます。
Copilot は、さまざまなタイプのマイクロサービスの作成と運用の為に、事前定義された CloudFormation テンプレートと、ユーザーフレンドリーなワークフローを提供します。
デプロイメントスクリプトを記述する代わりに、アプリケーションの開発に集中できます。

より詳細な AWS Copilot の紹介については、[Overview](../docs/concepts/overview.ja.md) を確認してください。

## サービスアラームベースのロールバック

[カスタム CloudWatch アラーム](https://docs.aws.amazon.com/AmazonECS/latest/userguide/deployment-alarm-failure.html) で [ECS のデプロイ状況を監視する](https://aws.amazon.com/blogs/containers/automate-rollbacks-for-amazon-ecs-rolling-deployments-with-cloudwatch-alarms/) ことができるようになりました。デプロイ中にアラームが `In alarm` 状態になった場合に、最後に完了したデプロイにロールバックするようにサービスを設定します。[サーキットブレーカー](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/deployment-circuit-breaker.html) によって、すでに失敗したデプロイをロールバックします。また、今回、障害ではないが、選択したメトリクスに従ってパフォーマンスが出ていないサービスのデプロイメントをロールバックすることもできるようになりました。

バックエンド、ワーカー、または負荷分散されたウェブサービスのマニフェストで、独自の既存の CloudWatch アラームをインポートできます。
```yaml
    deployment:
      rollback_alarms: ["MyAlarm-ELB-4xx", "MyAlarm-ELB-5xx"]
```

また、Copilotにお好みの閾値を設定して、CPUやメモリ使用率アラームを作成してもらうこともできます。
```yaml
    deployment:
      rollback_alarms:
        cpu_utilization: 70    // Percentage value at or above which alarm is triggered.
        memory_utilization: 50 // Percentage value at or above which alarm is triggered.
```

ワーカーサービスの場合は、`ApproximateNumberOfMessagesDelayed` を監視するアラームを作成することもできます。
```yaml
    deployment:
      rollback_alarms:
        messages_delayed: 5
```

Copilotがお客様に代わってアラームを作成する際、いくつかのデフォルトが設定されます。
```yaml
    ComparisonOperator: 'GreaterThanOrEqualToThreshold'
    DatapointsToAlarm: 2
    EvaluationPeriods: 3
    Period: 60
    Statistic: 'Average'
```

サービスマニフェストでロールバックアラームを設定すると、最初のデプロイ後に (ロールバックする既存のデプロイがないときに) `svc deploy` を実行するたびに、ECS はアラームをポーリングし、違反があった場合はロールバックをトリガーします。

## 環境アドオン用 `storage init` 

以前は、`copilot storage init` はワークロードに接続されたストレージアドオンだけをサポートしていました。
ストレージをデプロイするために `copilot svc deploy` を実行し、`copilot svc delete` を実行すると、サービスとともにストレージが削除されます。

これで、環境ストレージアドオンを作成できるようになりました。ストレージは `copilot env deploy` を実行するとデプロイされます。
そして、`copilot env delete` を実行して環境を削除するまで削除されません。

ワークロードストレージと同様に、環境ストレージも内部的には [環境アドオン](../docs/developing/addons/environment.ja.md) のもう1つです。

### [サービスごとのデータベース](https://docs.aws.amazon.com/prescriptive-guidance/latest/modernization-data-persistence/database-per-service.html) のデフォルト

マイクロサービスの世界では、一般的にデータストレージリソースをそれぞれマイクロサービス専用に設定することが推奨されます。
複数のサービスで共有されるモノリスストレージの代わりに。
このパターンでは、マイクロサービスの核となる特徴であるルーズカップリングが維持されます。
Copilotでは、このサービスごとのデータベースパターンに従うことを推奨しています。デフォルトでは、Copilotが生成するストレージリソース
は、1 つのサービスまたはジョブによってアクセスされることを前提としています。

!!!note ""
ただし、各ユーザーには独自の状況があります。データストレージを複数のサービス間で共有する必要がある場合は、
コパイロットで生成された CloudFormation テンプレートを変更して、目的を達成することができます。

表示される可能性のあるプロンプトの例を次に示します。
!!! info ""

    ```term
    $ copilot storage init
    What type of storage would you like to create?
    > DynamoDB            (NoSQL)
      S3                  (Objects)
      Aurora Serverless   (SQL)

    Which workload needs access to the storage?
    > api
      backend

    What would you like to name this DynamoDB Table? movies

    Do you want the storage to be created and deleted with the api service?
      Yes, the storage should be created and deleted at the same time as api
    > No, the storage should be created and deleted at the environment level
    ```

フラグを使用してプロンプトをスキップできます。次のコマンドは、上記のプロンプトと同等です。

```console
copilot storage init \
--storage-type "DynamoDB" \
--workload "api" \
--name "movies" \
--lifecycle "environment"
```

すべてのプロンプトに答えるか、フラグを使用してプロンプトをスキップすると、Copilot は DynamoDB ストレージリソースを定義する CloudFormation テンプレートを生成します。
これは、`copilot/environments` ディレクトリの下に生成されます。さらに、必要なアクセスポリシーを生成します。これは api サービスを許可するポリシーです
「映画」ストレージへのアクセス。アクセスポリシーは、ワークロードアドオンとして作成されます。つまり、そのアクセスポリシーがデプロイされ、
「api」サービスと同時に削除されました。
!!! info ""
`    copilot/
    ├── environments/
    │   ├── addons/         
    │   │   └── movies.yml                # <- The CloudFormation template that defines the "movies" DynamoDB storage.
    │   └── test/
    │       └── manifest.yml
    └── api
        ├── addons/
        │   └── movies-access-policy.yml  # <- The CloudFormation template that defines the access policy.
        └─── manifest.yml
   ```

ストレージのタイプ、およびストレージに接するワークロードのタイプによって、Copilotが生成するCloudFormationファイルの内容は異なる場合があります。


???- note "リクエスト駆動型WebサービスをフロントとするAurora Serverlessで生成されるサンプルファイル"
    ```

    # Example: an Aurora Serverless v2 storage whose lifecycle is at the environment-level, faced by a Request-Driven Web Service.
    copilot/
    ├── environments/
    │   └── addons/
    │         ├── addons.parameters.yml   # The extra parameters required by the Aurora Serverless v2 storage.
    │         └── user.yml                # An Aurora Serverless v2 storage
    └── api                               # "api" is a Request-Driven Web Service
        └── addons/
            ├── addons.parameters.yml   # The extra parameters required by the ingress resource.
            └── user-ingress.yml        # A security group ingress that grants "api" access to the "api" storage"
   ```

詳細については、[ストレージページ](../docs/developing/storage.ja.md) もチェックしてください。

## リクエスト主導型Webサービスシークレットのサポート

Copilot を使用して、シークレット (SSM パラメータストアまたは AWS シークレットマネージャーから) を環境変数として App Runner サービスに追加できるようになりました。

負荷分散型Webサービスなどの他のサービスタイプと同様に、最初にシークレットに次のタグを追加する必要があります。

| キー | 値 |
|-----------------------------------------------------------------------------------|
| `copilot-application` | シークレットにアクセスしたいアプリケーション名 |
| `copilot-environment` | シークレットにアクセスしたい環境名 |

次に、リクエスト主導型Webサービスのマニフェストを次のように更新するだけです。

```yaml
  secrets:
    GITHUB_TOKEN: GH_TOKEN_SECRET
```

これでデプロイをすると、サービスはシークレットに環境変数としてアクセスできるようになりました。

`secrets` フィールドの詳細な使用方法については、

```yaml
secrets:
  # To inject a secret from SecretsManager.
  # (Recommended) Option 1. Referring to the secret by name.
  DB:
    secretsmanager: 'demo/test/mysql'
  # You can refer to a specific key in the JSON blob.
  DB_PASSWORD:
    secretsmanager: 'demo/test/mysql:password::'
  # You can substitute predefined environment variables to keep your manifest succinct.
  DB_PASSWORD:
    secretsmanager: '${COPILOT_APPLICATION_NAME}/${COPILOT_ENVIRONMENT_NAME}/mysql:password::'

  # Option 2. Alternatively, you can refer to the secret by ARN.
  DB: 'arn:aws:secretsmanager:us-west-2:111122223333:secret:demo/test/mysql-Yi6mvL'

  # To inject a secret from SSM Parameter Store
  # Option 1. Referring to the secret by ARN.
  GITHUB_WEBHOOK_SECRET: 'arn:aws:ssm:us-east-1:615525334900:parameter/GH_WEBHOOK_SECRET'

  # Option 2. Referring to the secret by name.
  GITHUB_WEBHOOK_SECRET: GITHUB_WEBHOOK_SECRET
```

[マニフェスト仕様](../docs/manifest/rd-web-service/#secrets) を参照してください。サービスにシークレットを挿入する方法の詳細については、[シークレットページ](../docs/developing/secrets.ja.md) を参照してください。

## 次は？

以下のリンクより、新しい Copilot CLI バージョンをダウンロードし、[GitHub](https://github.com/aws/copilot-cli/) や [コミュニティチャット](https://gitter.im/aws/copilot-cli)にフィードバックを残してください。

- [最新 CLI バージョン](../docs/getting-started/install.ja.md)のダウンロード
- [スタートガイド](../docs/getting-started/first-app-tutorial.ja.md)を試す
- [GitHub](https://github.com/aws/copilot-cli/releases/tag/v1.26.0) でリリースノートの全文を読む
