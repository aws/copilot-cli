# AWS Copilot v1.16: マルチパイプライン、 SNS サブスクリプションフィルターなど

投稿日: 2022 年 4 月 6 日

The AWS Copilot コアチームは Copilot v1.16 リリースを発表できることを嬉しく思います。
このリリースに貢献した 7 人の新しいコミュニティ開発者を歓迎します。[@codekitchen](https://github.com/codekitchen),
[@shingos](https://github.com/shingos) 、 [@csantos](https://github.com/csantos) 、 [@rfma23](https://github.com/rfma23)、
[@g-grass](https://github.com/g-grass) 、 [@isleys](https://github.com/isleys) 、
[@kangere](https://github.com/kangere) 。 [パブリックコミュニティチャット](https://gitter.im/aws/copilot-cli) は成長していて、270 人以上の方がオンラインで、日々助け合ってます。また、 最近 [GitHub](http://github.com/aws/copilot-cli/) で 2.1 K を超える Star を獲得しました。
AWS Copilot に関心とサポートを示してくださった皆様に感謝しています。


Copilot v1.16 にはいくつかの新機能と改善が加えられました:

* **マルチパイプライン:** `copilot pipeline init` は、リポジトリ内の別々のブランチを追跡する複数の AWS CodePipeline を作成します。詳細は、
[こちら](./#create-multiple-pipelines-per-branch)を確認してください。
* **SNS サブスクリプションフィルターポリシー:** `filter_policy` フィールドを利用して、 Worker Service は、サブスクライブしたトピックに対する SNS メッセージをフィルターできます。
詳細は、[こちら](./#define-messages-filter-policy-in-publishsubscribe-service-architecture)を確認してください。
* **その他 たくさんの改善点:**
    * `deploy` コマンドに `--no-rollback` フラグが追加されました。デプロイが失敗した場合にスタックの自動ロールバックを無効化します([#3341](https://github.com/aws/copilot-cli/pull/3341))。 新しいフラグは、インフラストラクチャの更新失敗時にトラブルシューティングをする際に役立ちます。例えば、デプロイが失敗した場合、CloudFormation はスタックをロールバックするため、失敗時に発生したログを削除します。このフラグは、問題のトラブルシューティングの為にログを保存するのに役立ちます。保存後にロールバックし、Manifest を再度更新できます。
    * `package` コマンドに `--upload-assets` フラグが追加されました。CloudFormation テンプレートを生成する前に、ECR または S3 にアセットをプッシュします ([#3268](https://github.com/aws/copilot-cli/pull/3268))。 このフラグによりパイプラインの buildspec を大幅に簡略化ができます。 buildspec を再作成する場合は、ファイルを削除し、`copilot pipeline init` を再度実行します。その際に、プロンプトでは、既存のパイプライン名を指定します。
    * Environment で `task run` 実行中にセキュリティグループを追加できるようになりました ([#3365](https://github.com/aws/copilot-cli/pull/3365))。
    * Copilot が CI 環境で実行されている場合(環境変数 `CI` を `true` を設定している場合)に、Docker 進行状況の更新ノイズが少なくなりました ([#3345](https://github.com/aws/copilot-cli/pull/3345))。
    * App Runner がサポートされていないリージョンに対して、App Runner Service をデプロイするときに警告ログを出力するようになりました([#3326](https://github.com/aws/copilot-cli/pull/3326))。
    * `app show` および `env show` コマンドは、テーブル形式で Service や Job がデプロイされた Environment を出力するようになりました。([#3379](https://github.com/aws/copilot-cli/pull/3379)、[#3316](https://github.com/aws/copilot-cli/pull/3316))。
    * Pipeline Manifest の `build.buildspec` を使って buildspec パスをカスタマイズできるようになりました([#3403](https://github.com/aws/copilot-cli/pull/3403))。

## AWS Copilot とは?

AWS Copilot CLI は AWS 上でプロダクションレディなコンテナ化されたアプリケーションのビルド、リリース、そして運用のためのツールです。
開発のスタートからステージング環境へのプッシュ、本番環境へのリリースまで、Copilot はアプリケーション開発ライフサイクル全体の管理を容易にします。
Copilot の基礎となるのは、 AWS CloudFormation です。CloudFormation により、インフラストラクチャを 1 回の操作でコードとしてプロビジョニングできます。
Copilot は、 さまざまなタイプのマイクロサービスの作成と運用の為に、事前定義された CloudFormation テンプレートと、ユーザーフレンドリーなワークフローを提供します。
デプロイメントスクリプトを記述する代わりに、アプリケーションの開発に集中できます。


より詳細な AWS Copilot の紹介については、[Overview](../docs/concepts/overview.ja.md) を確認してください。

<a id="create-multiple-pipelines-per-branch"></a>
## ブランチごとにマルチパイプラインを作成する
_Contributed by [Efe Karakus](https://github.com/efekarakus/), [Janice Huang](https://github.com/huanjani/)_

リリースプロセスを自動化することは、ソフトウエアデリバリの最も重要な部分の 1 つです。 AWS Copilot はそのプロセスをできるだけ簡単に設定できる様にしたいと考えています。
アプリケーションの全ての Environment で `copilot deploy` を手動で実行する代わりに、
いくつかの `copilot pipeline` コマンドを実行するだけで、 `git push` するたびに環境に対して自動的にリリースされる継続的デリバリーパイプラインをセットアップできます。

生成された CodePipeline は、次の様な基本構造になります。

* Source ステージ: 設定済みの GitHub、Bitbucket、または CodeCommit のリポジトリーのブランチに対して push した際に、新しいパイプライン実行がトリガーされます。
* Build: リポジトリからコードがダウンロードされると、Service 用のコンテナイメージがビルドされ、すべての Environment の Amazon ECR リポジトリにプッシュされます。加えて、Addon テンプレート、Lambda 関数 zip ファイル、環境変数ファイルなどのすべての入力ファイルが S3　にアップロードされます。
* Deploy ステージ: コードがビルドされた後、任意または全ての Environment にデプロイでき、オプションでデプロイ後のテストや手動承認を使用できます。

以前は、 Copilot では git リポジトリごとに 1 つのパイプラインしか作成できませんでした。`copilot pipeline init` を実行すると単一の Pipeline Manifest が生成されます。例えば、以下の Manifest ファイルは、まず "test" にリリースします。デプロイが成功すると "prod" Environment にリリースする CodePipeline をモデル化します。 

```
$ copilot pipeline init
1st stage: test
2nd stage: prod
✔ Wrote the pipeline manifest for my-pipeline at 'copilot/pipeline.yml'

Required follow-up actions:
- Commit and push the buildspec.yml, pipeline.yml, and .workspace files of your copilot directory to your repository.
- Run `copilot pipeline deploy` to create your pipeline.

$ cat copilot/pipeline.yml
name: my-pipeline
source:
  provider: GitHub
  properties:
    branch: main
    repository: https://github.com/user/repo
stages:
    - name: test
    - name: prod
    # requires_approval: true
    # test_commands: [echo 'running tests', make test]
```

このモデルは "main" ブランチへの全てのコミットを Environment 全体でリリースしたいユーザに適しています。
代替モデルは、ブランチごとにパイプラインを作成することです。例えば、"main" ブランチにいくつかの変更をコミットし、満足したら "test" ブランチに変更をマージして "test" Environment にデプロイします。次に "prod" ブランチにマージします。v1.15 までは単一の Pipeline Manifest のみがサポートされていたため、このモデルは実現できませんでした。

v1.16 からは、Copilot ユーザは git リポジトリに複数のパイプラインを作成できる様になったので、ブランチごとに別々のパイプラインを設定できます。例えば、 マージ時のコンフリクトを気にすることなく、`copilot pipeline init` を git リポジトリの別々のブランチで実行できます。

```
$ git checkout test
$ copilot pipeline init
Pipeline name: repo-test
1st stage: test
Your pipeline will follow branch 'test'.

✔ Wrote the pipeline manifest for repo-test at 'copilot/pipelines/repo-test/manifest.yml'
Required follow-up actions:
- Commit and push the copilot/ directory to your repository.
- Run `copilot pipeline deploy` to create your pipeline.

$ git checkout prod
$ copilot pipeline init
Pipeline name: repo-prod
1st stage: prod
Your pipeline will follow branch 'prod'.

✔ Wrote the pipeline manifest for repo-prod at 'copilot/pipelines/repo-prod/manifest.yml'
Required follow-up actions:
- Commit and push the copilot/ directory to your repository.
- Run `copilot pipeline deploy` to create your pipeline.
```

"test" ブランチに対して変更がマージされると、 "repo-test" パイプラインがトリガーされます。同様に、"prod"ブランチに対して変更を反映すると "repo-prod" パイプラインをトリガーできます。

パイプラインについてより詳しく学びたい場合は、[Copilot のドキュメント](../docs/concepts/pipelines.ja.md)を確認してください。

<a id="define-messages-filter-policy-in-publishsubscribe-service-architecture"></a>
## パブリッシュ/サブスクライブサービスアーキテクチャにおけるメッセージフィルターポリシーの定義
_Contributed by [Penghao He](https://github.com/iamhopaul123/)_

マイクロサービスアーキテクチャにおける共通的なニーズは、サービス間でメッセージをやり取りするための堅牢なパブリッシュ / サブスクライブロジックを簡単に実装できることです。
AWS Copilot は、ロジックを簡単にするため、Amazon SNS と Amazon SQS のサービスを組み合わせて実現します。
AWS Copilot では、 単一または複数のサービスが名前付き SNS トピックにメッセージを発行する様に設定し、 メッセージを受信して処理するワーカーサービスを作成できます。
AWS Copilot は SNS トピック、SQS キュー、必要なポリシーなどの、 パブサブインフラストラクチャを設定し、自動プロビジョニングします。
詳細については、 [AWS Copilot のドキュメント内の Publish/Subscribe architecture](../docs/developing/publish-subscribe.ja.md)を確認してください。

デフォルトでは、Amazon SNS トピックのサブスクライバーはトピックにパブリッシュされた全てのメッセージを受け取ります。
メッセージのサブセットを受け取る為には、サブスクライバーはトピックサブスクリプションに対してフィルターポリシーを割り当てる必要があります。例えば、トピックに注文をバブリッシュするサービスがあるとします。

```yaml
`# manifest.yml for api service
name: api
type: Backend Service
publish:
  topics:
    - name: ordersTopic`
```

続いて、 `ordersTopic` の全てのタイプのメッセージを処理するワーカーです。

```yaml
name: orders-worker
type: Worker Service

subscribe:
  topics:
    - name: ordersTopic
      service: api
  queue:
    dead_letter:
      tries: 5
```

AWS Copilot はサブスクリプションを作成し、必要な全てのインフラストラクチャをプロビジョニングするので、コードの作成に集中できます。
ただし、価格が 100 ドルを超えるキャンセルされた注文のメッセージのみを処理する新しいワーカーを作成する必要があるとします。

Copilot v1.16 リリース以降は、コードでこれらのメッセージを除外する必要はありません。SNS サブスクリプションフィルターポリシーを定義するだけで済みます。

```yaml
name: orders-worker
type: Worker Service

subscribe:
  topics:
    - name: ordersTopic
      service: api
      filter_policy:
        store:
            - example_corp
        event:
            - order_canceled
        price_usd:
            - numeric:
              - ">="
              - 100
  queue:
    dead_letter:
      tries: 5
```

このフィルターポリシーを設定すると、Amazon SNS はこれらの属性とのマッチングにより全てのメッセージをフィルターします。
SNS フィルターについてより学びたい場合は、[Copilot のドキュメント](../docs/manifest/worker-service.ja.md#topic-filter-policy)を確認してください。

## 次は？

以下のリンクをたどって、新しい Copilot CLI バージョンをダウンロードし、[GitHub](https://github.com/aws/copilot-cli/) や [コミュニティチャット](https://gitter.im/aws/copilot-cli)に
フィードバックを残してください。

* [最新 CLI バージョン](../docs/getting-started/install.ja.md)のダウンロード
* [スタートガイド](../docs/getting-started/first-app-tutorial.ja.md)を試す
* [GitHub](https://github.com/aws/copilot-cli/releases/tag/v1.16.0) でリリースノートの全文を読む

今回のリリースの翻訳はソリューションアーキテクトの浅野が担当しました。

