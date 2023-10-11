---
title: 'AWS Copilot v1.30: `copilot run local` コマンド、Ctrl-C 機能、デプロイ前後処理、`copilot deploy` の機能拡張'
twitter_title: 'AWS Copilot v1.30'
image: ''
image_alt: ''
image_width: '1051'
image_height: '747'
---

# AWS Copilot v1.30: `copilot run local` コマンド、Ctrl-C 機能、デプロイ前後処理、`copilot deploy` の機能拡張

投稿日: 2023 年 8 月 30 日

AWS Copilot コアチームは Copilot v1.30 のリリースを発表します。

本リリースにご協力いただいた [@Varun359](https://github.com/Varun359) に感謝します。
私たちのパブリックな[コミュニティチャット](https://app.gitter.im/#/room/#aws_copilot-cli:gitter.im) は成長しており、オンラインでは 500 人以上、[GitHub](http://github.com/aws/copilot-cli/) では 3k 以上のスターを獲得しています 🚀。
AWS Copilot へご支援、ご支持いただいている皆様お一人お一人に感謝をいたします。

Copilot v1.30 ではより柔軟で効率的な開発を支援する大きな機能強化が行われました:

- **`copilot run local`**: Copilot に新しい操作コマンドが追加され、ローカルで Service を実行できるようになりました! [詳細はこちらをご覧ください](#copilot-run-local)。
- **Ctrl-C によるデプロイのロールバック**: もう完了まで待つ必要はありません！いつでもターミナルから CloudFormation デプロイをロールバックできるようになりました。[詳細はこちらをご覧ください](#roll-back-deployments-with-ctrl-c)。
- **デプロイ前後の Pipline アクション**: ワークロード/Environment のデプロイ前後に、DB マイグレーション、統合テスト、その他のアクションを挿入できるようになりました。[詳細はこちらをご覧ください](#deployment-actions)。
- **`copilot deploy` の機能拡張**: このコマンドのスコープや柔軟性について拡充しました。[詳細はこちらをご覧ください](#copilot-deploy-enhancements)。
- **`--detach flag`**: CloudFormation スタックイベントの進行状況をターミナル上でスキップできるようにしました。[詳細はこちらをご覧ください](#use---detach-to-deploy-without-waiting)。

??? note "AWS Copilot とは？"

    AWS Copilot CLI は AWS 上でプロダクションレディなコンテナ化されたアプリケーションのビルド、リリース、そして運用のためのツールです。
    開発のスタートからステージング環境へのプッシュ、本番環境へのリリースまで、Copilot はアプリケーション開発ライフサイクル全体の管理を容易にします。
    Copilot の基礎となるのは、 AWS CloudFormation です。CloudFormation により、インフラストラクチャを 1 回の操作でコードとしてプロビジョニングできます。
    Copilot は、さまざまなタイプのマイクロサービスの作成と運用の為に、事前定義された CloudFormation テンプレートと、ユーザーフレンドリーなワークフローを提供します。
    デプロイメントスクリプトを記述する代わりに、アプリケーションの開発に集中できます。

    より詳細な AWS Copilot の紹介については、[Overview](../docs/concepts/overview.ja.md) を確認してください。

<a id="copilot-run-local"></a>
## `copilot run local`
Service に対する変更作業をしているときに `copilot run local` を使用すると、デプロイのオーバーヘッドなしにコードの変更をテストできるため、イテレーションループを高速化できます。開始するには、まず `copilot svc deploy` を実行して Service のバージョンをデプロイする必要があります。

Service のデプロイが完了したら、コードの修正を開始できます。変更をテストする準備をした後、`copilot run local` を実行すると、Copilot はプライマリコンテナとサイドカーコンテナの両方に対して以下を実行します。

1. [`image`](../docs/manifest/lb-web-service.ja.md#image) で指定されたイメージをビルドまたは pull します。
2. [`secrets`](../docs/manifest/lb-web-service.ja.md#secrets) で指定されたシークレットの値を取得します。
3. 現在の IAM ユーザー/ロールの認証情報を取得します。
4. [`variables`](../docs/manifest/lb-web-service.ja.md#variables) とステップ 2 のシークレット値及びステップ 3 の IAM 認証情報を適用した状態でステップ 1 で指定したコンテナイメージをローカルマシンで実行します。


Service からのログはターミナルにストリーミングされます。テストが終了した後に Ctrl-C を入力すると、Copilot が実行中のコンテナをすべてクリーンアップしてから終了します!

<a id="roll-back-deployments-with-ctrl-c"></a>
## Ctrl-C によるデプロイのロールバック

Service、Job、Environment がデプロイされるのを待っている間、`Ctrl-C` を押してアップデートをキャンセルできるようになりました。この操作はスタックを以前の設定にロールバックするか、初めてデプロイする場合はスタックを削除します。

'Ctrl-C' を2回押すとプログラムは終了しますが、スタックのロールバックや削除は続行されます。

`Ctrl-C` は `copilot svc deploy`、`copilot job deploy`、`copilot env deploy`、`copilot deploy` の各コマンドで有効になりました。

<a id="deployment-actions"></a>
## デプロイ時のアクション
ワークロードのデプロイの前にデータベースの移行を実行する必要があり、ワークロードの健全性チェックはその更新に依存する場合があります。あるいは、ワークロードのデプロイ後に Pipeline で E2E またはインテグレーションテストを実行したい場合もあります。これらのアクションは、[Copilot Pipeline](../docs/concepts/pipelines.ja.md) で可能になりました。

Copilot は以前から ['test_commands'](https://aws.github.io/copilot-cli/docs/manifest/pipeline/#stages-test-cmds) をサポートしていますが、[デプロイ前](https://aws.github.io/copilot-cli/docs/manifest/pipeline/#stages-predeployments)と[デプロイ後](https://aws.github.io/copilot-cli/docs/manifest/pipeline/#stages-postdeployments)のアクションによって Pipeline の機能と柔軟性が拡張されます。`test_commands` の場合、Copilot はコマンド文字列を含む [buildspec](https://docs.aws.amazon.com/codebuild/latest/userguide/build-spec-ref.html) を Pipeline の CloudFormation テンプレートにインライン化します。`pre_deployments` と `post_deployments` の場合、Copilot はローカルのワークスペース内の buildspec を読み込みます。

これらのアクションのすべての設定は、[Pipeline Manifest](../docs/manifest/pipeline.ja.md) で制御できます。複数のデプロイ前アクションと複数のデプロイ後アクションを持つことができます。なお、それぞれのアクションについて、`[pre_/post_]deployments.buildspec`フィールドに、プロジェクトルートからの相対パスで [buildspec](https://docs.aws.amazon.com/codebuild/latest/userguide/build-spec-ref.html) へのパスを指定する必要があります。Copilot はアクション用の CodeBuild プロジェクトを生成し、Pipeline および Application と同じリージョンにデプロイします。buildspec 内で Copilot コマンド (`copilot svc exec` や `copilot task run` など) を使用して、デプロイされた Environment にある VPC またはワークロードのデプロイ先の Environment にある VPC にアクセスします。なお、`$COPILOT_APPLICATION_NAME` および `$COPILOT_ENVIRONMENT_NAME` という Copilot 環境変数を使用して、buildspec ファイルを複数の Environment で再利用できます。

`depends_on` フィールドを使用して、デプロイ前およびデプロイ後のグループ内でのアクションの順序を指定することもできます。デフォルトでは、アクションは並行して実行されます。

`post_deployments` と `test_commands` は相互に排他的です。
```yaml
stages:
  - name: test
    require_approval: true
    pre_deployments:
      db_migration: # このアクションの名前
        buildspec: copilot/pipelines/demo-api-frontend-main/buildspecs/buildspec.yml # buildspec へのパス
    deployments: # 任意項目 デプロイの順序
      orders:
      warehouse:
      frontend:
        depends_on: [orders, warehouse]
    post_deployments:
      db_migration:
        buildspec: copilot/pipelines/demo-api-frontend-main/buildspecs/post_buildspec.yml
      integration:
        buildspec: copilot/pipelines/demo-api-frontend-main/buildspecs/integ-buildspec.yml
        depends_on: [db_migration] # 任意項目 アクションの順序
```

<a id="copilot-deploy-enhancements"></a>
## `copilot deploy` の機能拡張
`copilot deploy` がワークロードの初期化、Environment の初期化とデプロイをサポートするようになりました。Application と Manifest のみを含むリポジトリから開始し、単一のコマンドで動作させる Environment と Service をデプロイできるようになりました。必要なワークロードをデプロイする前に Environment をデプロイすることもできます。

例えば、"prod" Environment と "frontend" および "backend" Service の Manifest を含むリポジトリをクローンしてデプロイしたいとします。
`copilot deploy` は、必要に応じてワークロードと Environment を初期化するようプロンプトを表示し、Environment をデプロイするアカウントの認証情報を尋ねてから、Environment とワークロードをデプロイします。
```console
$ git clone myrepo
$ cd myrepo
$ copilot app init myapp
$ copilot deploy -n frontend -e prod
```

Environment がデプロイされるプロファイルとリージョンを指定します。
```console
$ copilot deploy --region us-west-2 --profile prod-profile -e prod --init-env
```

既に Environment が存在する場合は、デプロイをスキップします。
```console
$ copilot deploy --deploy-env=false 
```

< id="use---detach-to-deploy-without-waiting"></a>
## `--detach` を使用して待ち時間なしでデプロイ

通常、`deploy` コマンドを実行すると、Copilot は進捗状況をターミナルに表示し、デプロイが完了するのを待ちます。

Copilot を待機させたくない場合は、`--detach` フラグを使用します。Copilot はデプロイをトリガーしてプログラムを終了し、進行状況を表示したりデプロイ完了まで待機しません。

`--detach` フラグは、`copilot svc deploy`、`copilot job deploy`、`copilot env deploy`、`copilot deploy` コマンドで使用できます。

## 次は？

以下のリンクより、新しい Copilot CLI バージョンをダウンロードし、[GitHub](https://github.com/aws/copilot-cli/) や [コミュニティチャット](https://gitter.im/aws/copilot-cli)にフィードバックを残してください。

- [最新 CLI バージョン](../docs/getting-started/install.ja.md)のダウンロード
- [スタートガイド](../docs/getting-started/first-app-tutorial.ja.md)を試す
- [GitHub](https://github.com/aws/copilot-cli/releases/tag/v1.30.0) でリリースノートの全文を読む

今回のリリースの翻訳はソリューションアーキテクトの杉本が担当しました。
