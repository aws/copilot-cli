---
title: 'AWS Copilot v1.20: Envronment Manifest とその先へ!'
twitter_title: 'AWS Copilot v1.20'
image: 'https://user-images.githubusercontent.com/879348/179278910-1e1ae7e7-cb57-46ff-a11c-07919f485c79.png'
image_alt: 'Environment Manifest'
image_width: '1106'
image_height: '851'
---

# AWS Copilot v1.20: Envronment Manifest とその先へ!

投稿日: 2022 年 7 月 19 日

AWS Copilot コアチームは、Copilot v1.20 のリリースを発表します。
このリリースに貢献してくれた [@gautam-nutalapati](https://github.com/gautam-nutalapati)、[@codekitchen](https://github.com/codekitchen), そして [@kangere](https://github.com/kangere/) に感謝します。私たちのパブリックな[コミュニティチャット](https://gitter.im/aws/copilot-cli)は成長しており、オンラインでは 300 人以上、[GitHub](http://github.com/aws/copilot-cli/) では 2.3k 以上のスターを獲得しています。AWS Copilot へご支援、ご支持いただいている皆様お一人お一人に感謝をいたします。

Copilot v1.20 では、いくつかの新機能と改良が施されています。

* **Environment Manifest**: Infrastructure as Code のすべての利点を環境にもたらす [Manifest ファイル](../docs/manifest/environment.ja.md)を使用して、Environment を作成および更新できるようになりました。既存の Environment を移行する方法については、[詳細な手順](#environment-manifest)を参照してください。
* **オートスケーリングクールダウンのサポート**: Service Manifest で[オートスケーリングクールダウン](#%E3%82%AA%E3%83%BC%E3%83%88%E3%82%B9%E3%82%B1%E3%83%BC%E3%83%AA%E3%83%B3%E3%82%B0%E3%82%AF%E3%83%BC%E3%83%AB%E3%83%80%E3%82%A6%E3%83%B3%E3%81%AE%E3%82%B5%E3%83%9D%E3%83%BC%E3%83%88)を指定できるようになりました。
* **ビルドロールの追加ポリシー**: Pipeline Manifest の `additional_policy` フィールドを通じて、CodeBuild ビルドプロジェクトのサービスロールにおける追加ポリシーを指定できるようになりました。ビルドプロジェクトロールに追加する追加ポリシードキュメントの指定方法については、[詳細な手順](../docs/manifest/pipeline.ja.md)を参照してください。 [(#3709)](https://github.com/aws/copilot-cli/pull/3709)
* **スケジュールされた Job の呼び出し**: 新しい `copilot job run` コマンドを使用して、既存のスケジュールされた Job をアドホックに実行できるようになりました。 [(#3692)](https://github.com/aws/copilot-cli/pull/3692)
* **デフォルトセキュリティグループを拒否する**: Service Manifest の `security_groups` に `deny_default` というオプションを追加し、デフォルトで適用される Environment のセキュリティグループの Ingress を削除するようにしました。 [(#3682)](https://github.com/aws/copilot-cli/pull/3682)
* **ALBを使った Backend Service の予測可能なエイリアス**: 内部 ALB が設定されている Backend Service にエイリアスを指定しない場合、デフォルトの ALB ホスト名ではなく、`svc.env.app.internal` というホスト名で到達できるようになりました。 ([#3668](https://github.com/aws/copilot-cli/pull/3668))

???+ note "AWS Copilot とは?"

    AWS Copilot CLI は AWS 上でプロダクションレディなコンテナ化されたアプリケーションのビルド、リリース、そして運用のためのツールです。
    開発のスタートからステージング環境へのプッシュ、本番環境へのリリースまで、Copilot はアプリケーション開発ライフサイクル全体の管理を容易にします。
    Copilot の基礎となるのは、 AWS CloudFormation です。CloudFormation により、インフラストラクチャを 1 回の操作でコードとしてプロビジョニングできます。
    Copilot は、さまざまなタイプのマイクロサービスの作成と運用の為に、事前定義された CloudFormation テンプレートと、ユーザーフレンドリーなワークフローを提供します。デプロイメントスクリプトを記述する代わりに、アプリケーションの開発に集中できます。

    より詳細な AWS Copilot の紹介については、[Overview](../docs/concepts/overview.ja.md) を確認してください。

## Environment Manifest

v1.20 以前は、クライアントは追加の設定で Environment を更新することができませんでした。例えば、Environment がドメインと関連付けられていない場合、ユーザーは `env init --name copy --import-cert-arns` を実行して証明書付きの新しい Environment を作成し、古い Environment を取り壊す必要がありました。このリリースから、ユーザーは Environment を再作成することなく、[Manifest](../docs/manifest/environment.ja.md)を使用して Environment を変更することができます。
今後、新しい Environment リソースは `env init` コマンドのフラグの代わりに `manifest.yml` ファイルで設定されるようになります。

### ウォークスルー
**[1\]** `copilot env init` は、アカウントに Environment を即座にデプロイ**しなくなりました**。代わりに、このコマンドはローカルのワークスペースに [manifest.yml](../docs/manifest/environment.ja.md) ファイルを書き込みます。

??? example "`copilot env init` の実行"

    ```console
    $ copilot env init
    Environment name: prod-pdx
    Credential source: [profile default]
    Default environment configuration? Yes, use default.
    ✔ Wrote the manifest for environment prod-pdx at copilot/environments/prod-pdx/manifest.yml
    ...additional output messages
    ```

    ```console
    $ cat copilot/environments/prod-pdx/manifest.yml
    # The manifest for the "prod-pdx" environment.
    # Read the full specification for the "Environment" type at:
    #  https://aws.github.io/copilot-cli/docs/manifest/environment/

    # Your environment name will be used in naming your resources like VPC, cluster, etc.
    name: prod-pdx
    type: Environment

    # Import your own VPC and subnets or configure how they should be created.
    # network:
    #   vpc:
    #     id:

    # Configure the load balancers in your environment, once created.
    # http:
    #   public:
    #   private:

    # Configure observability for your environment resources.
    observability:
      container_insights: false
    ```

**[2\]** Manifest を修正した後、新しい `copilot env deploy` コマンドを実行して Environment スタックを作成または更新することができます。

??? example "`copilot env deploy` の実行"

    ```console
    $ copilot env deploy
    Name: prod-pdx
    ✔ Proposing infrastructure changes for the demo-prod-pdx environment.
    - Creating the infrastructure for the demo-prod-pdx environment.              [update complete]  [110.6s]
      - An ECS cluster to group your services                                     [create complete]  [9.1s]
      - A security group to allow your containers to talk to each other           [create complete]  [6.3s]
      - An Internet Gateway to connect to the public internet                     [create complete]  [18.5s]
      - Private subnet 1 for resources with no internet access                    [create complete]  [6.3s]
      - Private subnet 2 for resources with no internet access                    [create complete]  [6.3s]
      - A custom route table that directs network traffic for the public subnets  [create complete]  [15.5s]
      - Public subnet 1 for resources that can access the internet                [create complete]  [6.3s]
      - Public subnet 2 for resources that can access the internet                [create complete]  [6.3s]
      - A private DNS namespace for discovering services within the environment   [create complete]  [47.2s]
      - A Virtual Private Cloud to control networking of your AWS resources       [create complete]  [43.6s]
    ```

これで終わりです🚀! ワークフローは、`copilot svc` と `copilot job` コマンドの動作と同じです。

### 既存 Environment の移行
 
既存の Environment 用の [manifest.yml](../docs/manifest/environment.ja.md) ファイルを作成するために、Copilot は `copilot env show`  コマンドに新しい `--manifest` フラグを導入しました。
以下の例では、既存の `"prod"` Environment 用の Manifest ファイルを生成します。

**[1\]** 最初に、現在の git リポジトリまたは新しいリポジトリに、Environment Manifest のための必須ディレクトリ構造を作成します。

???+ example "prod のディレクトリ構造"

    ```console
    # 1. Navigate to your git repository.
    $ cd my-sample-repo/
    # 2. Create the directory for the "prod" environment  
    $ mkdir -p copilot/environments/prod
    ```

**[2\]** `copilot env show --manifest` コマンドを実行して Manifest を生成し、"prod" フォルダにリダイレクトします。

???+ example "Manifest の生成"

    ```console
    $ copilot env show -n prod --manifest > copilot/environments/prod/manifest.yml
    ```

これで完了です! Manifest ファイルを[仕様](../docs/manifest/environment.ja.md)の任意のフィールドで変更し、`copilot env deploy` を実行してスタックを更新することができるようになりました。

### 継続的デリバリ

最後に、Copilot は Service または Job として、Environment に対して同じ[継続的デリバリーの Pipeline](../docs/concepts/pipelines.ja.md)のワークフローを提供します。

**[1\]** [Manifest ファイルが作成される](#%E6%97%A2%E5%AD%98-environment-%E3%81%AE%E7%A7%BB%E8%A1%8C)と、既存の `copilot pipeline init` コマンドを実行して、デプロイステージを記述するための Pipeline の [`manifest.yml`](../docs/manifest/pipeline.ja.md) ファイルや、CloudFormation 設定ファイルを生成するための "Build" ステージで使用する `buildspec.yml` を作成することが可能です。

??? example "Pipeline Manifest と buildspec の作成"

    ```console
    $ copilot pipeline init                
    Pipeline name: env-pipeline
    What type of continuous delivery pipeline is this? Environments
    1st stage: test
    2nd stage: prod

    ✔ Wrote the pipeline manifest for copilot-pipeline-test at 'copilot/pipelines/env-pipeline/manifest.yml'    
    ✔ Wrote the buildspec for the pipeline's build stage at 'copilot/pipelines/env-pipeline/buildspec.yml'
    ```

**[2\]** AWS CodePipeline スタックを作成または更新するために、`copilot pipeline deploy` を実行します。

??? example "Pipeline の作成"

    ```console
    $ copilot pipeline deploy                                                 
    Are you sure you want to redeploy an existing pipeline: env-pipeline? Yes
    ✔ Successfully deployed pipeline: env-pipeline
    ```

## オートスケーリングクールダウンのサポート
Service Manifest に、オートスケーリングクールダウン期間を設定する機能が追加されました。`Load Balanced`、`Backend`、および `Worker` Service では、`count` の下にあるオートスケーリングフィールドを構成して、カスタムクールダウン期間を持つことができるようになりました。以前は、`cpu_percentage` などの各スケーリングメトリックは、120 秒の 'in' クールダウンと 60 秒の 'out' クールダウンが固定されていました。今回、グローバルクールダウン期間を設定できるようになりました。

??? example "一般的なオートスケーリングクールダウンの使用"

    ```
    count:
      range: 1-10
      cooldown:
        in: 30s
        out: 30s
      cpu_percentage: 50
    ```

また、クールダウンを個別に設定し、一般的なクールダウンを上書きすることも可能です。

??? example "特定のオートスケーリングクールダウンを使用する"

    ```
    count:
      range: 1-10
      cooldown:
        in: 2m
        out: 2m
      cpu_percentage: 50
      requests:
        value: 10
        cooldown:
          in: 30s
          out: 30s
    ```

## 次は?

以下のリンクより、新しい Copilot CLI バージョンをダウンロードし、[GitHub](https://github.com/aws/copilot-cli/) や [コミュニティチャット](https://gitter.im/aws/copilot-cli)にフィードバックを残してください。

* [最新 CLI バージョン](../docs/getting-started/install.ja.md)のダウンロード
* [スタートガイド](../docs/getting-started/first-app-tutorial.ja.md)を試す
* [GitHub](https://github.com/aws/copilot-cli/releases/tag/v1.20.0) でリリースノートの全文を読む

今回のリリースの翻訳はソリューションアーキテクトの杉本が担当しました。

