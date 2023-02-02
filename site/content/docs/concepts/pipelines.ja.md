<!-- textlint-disable ja-technical-writing/ja-no-mixed-period -->自動化されたリリースプロセスを持つことはソフトウェアデリバリの重要な観点の１つです。Copilot はこれらのセットアップを可能な限り簡単にすることを追求しています 🚀
<!-- textlint-enable ja-technical-writing/ja-no-mixed-period -->

このセクションでは GitHub、Bitbucket、CodeCommit リポジトリにコードの変更がプッシュされた際に自動的なビルドを実行し、Environment へのデプロイと自動テストを実行する CodePipeline を Copilot を使ってセットアップする例を見ていきます。

!!! Attention
    AWS CodePipeline は OS ファミリーが Windows の Service をサポートしていません。
    CodePipeline は、Build Stage で Linux ベースの AWS CodeBuild を使用しているため、現時点では Copilot Pipeline で Windows コンテナをビルドできません。

<!-- textlint-disable ja-technical-writing/no-exclamation-question-mark -->
## Why?
<!-- textlint-enable ja-technical-writing/no-exclamation-question-mark -->

ソフトウェアリリースについての哲学的すぎる話はしませんが、果たしてリリースパイプラインを持つ意味とはなんでしょうか？`copilot deploy` コマンドを使えば手元のコンピューターから Amazon ECS on AWS Fargate へ直接デプロイできるのに、なぜわざわざ仲介する仕組みを挟むのでしょうか？良い質問です。あるアプリケーションにとっては、手動での `deploy` で十分なこともあるでしょう。一方で、例えば複数の Environment やあるいは自動テストが追加されていきリリースパイプラインが複雑化してくると、その退屈な作業を何らかの仕組みにオフロードしたくなるでしょう。２つの Service が２つの(例えばテスト環境と本番環境のような) Environment 内にそれぞれあるとして、それらに対してテスト環境へのデプロイ後のインテグレーションテストを手で実施することは驚くほどに面倒な作業になります。

<!-- textlint-disable ja-technical-writing/ja-no-mixed-period -->
CodePipeline のような自動化されたリリースツールは、あなたのリリース作業を管理可能なものにする手助けをしてくれます。たとえリリース作業自体がそれほど複雑なものではなかったとしても、`git push` さえすれば変更を自動的にデプロイできる体験は、ちょっとした魔法のように感じますよね 🌈
<!-- textlint-enable ja-technical-writing/ja-no-mixed-period -->

## Pipeline の構成

Copilot はいくつかのコマンドで CodePipeline をセットアップします。作業を始める前に、まずは最終的に作成される Pipeline の構成を見ていきましょう。Pipeline は次に挙げる基本的な構造を持っています。

1. __Source Stage__ - 設定した GitHub、Bitbucket、あるいは CodeCommit リポジトリにプッシュすると、Pipeline の実行が開始されます。
2. __Build Stage__ - リポジトリからコードがダウンロードされると、Service 用のコンテナイメージがビルドされ、すべての Environment の Amazon ECR リポジトリにプッシュされます。加えて、[Addon](../developing/addons/workload.ja.md) テンプレートや、 Lambda 関数 zip ファイル、[環境変数ファイル](../developing/environment-variables.ja.md)などのすべての入力ファイルが S3 にアップロードされます。

ソースコードがリポジトリホストから pull された後に、 Service のコンテナイメージがビルドされ、 各環境の ECR リポジトリにパブリッシュされます。加えて、Addon テンプレート、Lambda 関数 zip ファイル、環境変数ファイルなどのすべての入力ファイルが S3　にアップロードされます。

3. __Deploy Stages__ - ビルドが終わると、一部あるいはすべての Environment にデプロイできます。オプションとしてデプロイ完了後のテスト実行やデプロイ実行に手動承認を挟むことが可能です。

Copilot を使って CodePipeline のセットアップを済ませたら、あとは GitHub、Bitbucket、あるいは CodeCommit リポジトリにプッシュするだけです。あとは CodePipeline がデプロイまでのプロセスを実行してくれます。

CodePipeline についてより深く学びたい場合は、[CodePipeline のドキュメント](https://docs.aws.amazon.com/ja_jp/codepipeline/latest/userguide/welcome-introducing.html)も参考にしてください。

## 3 ステップで作る Pipeline
Pipeline の作成に必要な手順は３つです。

1. 最初に Pipeline の作成準備と設定を実施します。
2. 次に、`copilot/` ディレクトリ以下に作成されたファイルをコミットして、リモート Git リポジトリにプッシュします。
3. 最後にクラウド上へ Pipeline を作成して完了です！

ワークスペースのルートで以下の３コマンドを実行してみましょう。

```bash
$ copilot pipeline init
$ git add copilot/ && git commit -m "Adding pipeline artifacts" && git push
$ copilot pipeline deploy
```
!!! Note
    パイプラインが Environment にデプロイできるように、`pipeline deploy` を実行する間に、少なくとも 1 つのワークロード( Service または Job ) を開始しておく必要があります。

✨ __Application アカウント__ に新しい Pipeline が作成されたはずです！何が起きているのか、もう少し深く知りたいですよね？読み進めましょう！

## ステップ・バイ・ステップで見る Pipeline のセットアップ

### ステップ 1: Pipeline の設定

Pipeline の設定はワークスペースのレベルで作成されます。もしワークスペース内にある Service が１つの場合、Pipeline はその Service についてのみ実行されます。もしワークスペース内に複数の Service がある場合、Pipeline はそれら全てをビルドします。Pipeline のセットアップを始めるには、Service (あるいは Service 群)がある Application のワークスペースのディレクトリに `cd` コマンドなどで入り、次のコマンドを実行します。

 `copilot pipeline init`

このコマンドの実行ではまだクラウド上の Pipeline は作成しませんが、Pipeline 作成に必要ないくつかのファイルを  `copilot/pipelines` 以下に作成します。

* __Pipeline name__: パイプラインの名前を `[repository name]-[branch name]` とすることをお勧めします。( 尋ねられた場合、 デフォルト名を受け入れるには 'Enter' ボタンを入力します)。これにより複数のパイプラインを作成した場合に、ブランチごとのパイプラインワークフローに従う場合にうまく機能でします。

* __Release order__: デプロイ先の Environment (複数可)を尋ねられます - どの Environment からデプロイを実施したいか、その順番にあわせて Environment を選択しましょう。(複数の Environment に対して同時にデプロイを実行することはありません)。最初に _test_ Environment へデプロイし、その後 _prod_ Environment へデプロイする、といった設定がよくある順番でしょう。

* __Tracking repository__: デプロイ先の Environment を選択すると、次にどの Git リポジトリを CodePipeline からトラックしたいかを尋ねられます。ここで選ぶリポジトリへのプッシュが、CodePipeline の Pipeline をトリガーするリポジトリとなります。(設定したい対象のリポジトリがここでリストに表示されない場合、 `--url` フラグで明示的に Git リポジトリの URL を渡すこともできます。)

* __Tracking branch__: リポジトリを選択すると、 Copilot は現在のローカルブランチをパイプラインを利用するブランチとして指定します。これはステップ 2 で変更できます。

### ステップ 2: Pipeline 用 Manifest ファイルの更新 (オプション)

Service はシンプルな Manifest ファイルを持ちます。同様に、Pipeline にも Manifest があります。`pipeline init` コマンドを実行すると、`copilot/pipelines/[your pipeline name]` ディレクトリ内に `manifest.yml` と `buildspec.yml` という２つのファイルが作成されます。`manifest.yml` の中は次のような感じになっているはずです。 (ここでは "api-frontend" という Service が "test" と "prod" の２つの Environment にデプロイされるものと仮定しましょう)

```yaml
# Pipeline 名 "demo-api-frontend-main" の Manifest
# この YAML ファイルは Pipeline を定義します。追跡するソースリポジトリと、Environment のデプロイ順序を指定します
# 詳細はこちら: https://aws.github.io/copilot-cli/ja/docs/manifest/pipeline/

# Pipeline 名
name: demo-api-frontend-main

# このテンプレートで利用されているスキーマバージョン
version: 1

# このセクションでは Pipeline の実行をトリガーするソースを定義します
source:
  # ソースコードのプロバイダ名を記述します
  # (例: GitHub, Bitbucket, CodeCommit)
  provider: GitHub
  # ソースコードの場所を追加で指定するプロパティです
  properties:
    branch: main
    repository: https://github.com/kohidave/demo-api-frontend
    # オプション: 既存の CodeStar Connections で作成された接続名を利用することも可能です
    # connection_name: a-connection

# このセクションでは Pipeline のデプロイ先となる Environment の順序を定義します
stages:
    - # Environment 名
      name: test
      test_commands:
        - make test
        - echo "woo! Tests passed"
    - # Environment 名
      name: prod
      # requires_approval: true
```
`manifest.yml` で利用可能な全ての設定項目については [Pipeline Manifest](../manifest/pipeline.ja.md) をご覧ください。

このファイルには大きく３つのパーツがあります。最初の `name` フィールドは Pipeline に作成されるパイプラインの名称です。そして `source` セクションは Pipeline がトラックするソースリポジトリとそのブランチといった詳細を定義し、最後の `stages` セクションでは、どの Environment に対してこの Pipeline でデプロイを行いたいか定義します。この設定ファイルはいつでも変更可能ですが、変更後は Git リポジトリへのコミットとプッシュ、その後 `copilot pipeline deploy` コマンドを実行する必要があります。

よくあるケースとしては、新たなデプロイ先の Environment を増やしたいときや、Pipeline がトラックするブランチを変更したい際にこのファイルを更新することになるでしょう。あるいはもしすでに CodeStar Connections に接続済みのリポジトリがあり、Copilot で新たに作成するのではなく既存のものを利用したい場合には、その接続名を記述することになります。また、Pipeline Manifest はデプロイの手動承認を設定したり、デプロイ後に自動テストを実行したりしたい場合の設定を記述する場所でもあります。(本ページ下部の "テストの追加" もご覧ください)

### ステップ 3: Buildspec ファイルの更新 (オプション)

`pipeline init` コマンドでは、`manifest.yml` と一緒に `buildspec.yml` も `copilot/pipelines/[your pipeline name]` ディレクトリ内に作成されます。この `buldspec.yml` にはビルドとコンテナイメージのプッシュに関する指示が記述されています。もし `docker build` と一緒にユニットテストやスタイルチェックのような追加のコマンドを実行したい場合は、buildspec の `build` フェーズにそれらのコマンドを追加してください。

実際にこの buildspec が実行される際には、後方互換性の観点から `pipeline init` コマンドの実行に利用したバージョンの Copilot バイナリがダウンロードされ、利用されます。

あるいは、CodeBuild で実行するために独自の buildspec を設定できます。[`manifest.yml` file](../manifest/pipeline.ja.md)で、場所を指定します。
```yaml
build:
  buildspec:
```

### ステップ 4: リポジトリに生成されたファイルをプッシュする

`manifest.yml`、`buildspec.yml`、そして `.workspace` ファイルが作成されたので、これらをリポジトリに追加しましょう。`copilot/` ディレクトリ以下に含まれたこれらのファイルが、Pipeline が `build` ステージを正しく実行するために必要となります。

### ステップ 5: Pipeline の作成

ここからが楽しいパートです！次のコマンドを実行しましょう！

`copilot pipeline deploy`

このコマンドはあなたの `manifest.yml` を解析し、__Application と同じアカウントとリージョン__ の CodePipeline に Pipeline を作成し、Pipeline を実行します。AWS マネジメントコンソールにログイン、あるいは `copilot pipeline status` コマンドで Pipeline の実行状況を確認できます。

![処理が完了した CodePipeline の様子](https://user-images.githubusercontent.com/828419/71861318-c7083980-30aa-11ea-80bb-4bea25bf5d04.png)

!!! info
    もしあなたが GitHub あるいは Bitbucket リポジトリを利用する場合、Copilot は [CodeStar Connections](https://docs.aws.amazon.com/ja_jp/dtconsole/latest/userguide/welcome-connections.html) を利用してリポジトリとの接続を作成する手助けをします。この過程で GitHub や Bitbucket のアカウントに AWS がアクセスするための認証アプリケーションをインストールする必要があります (e.g. GitHub の場合、"AWS Connector for GitHub")。Copilot と AWS マネジメントコンソールのガイダンスにしたがってこのステップを進めてください。

### ステップ 6: Pipeline の Copilot バージョンを管理する (オプション)

Pipeline を作成した後、`buildspec.yml` の以下の行を最新バージョンに更新することで、Pipeline で使用する Copilot のバージョンを管理できます。

```yaml
...
      # Copilot Linux バイナリをダウンロードします
      - wget -q https://ecs-cli-v2-release.s3.amazonaws.com/copilot-linux-v1.16.0
      - mv ./copilot-linux-v1.16.0 ./copilot-linux
...
```

## テストの追加

パイプラインの重要な要素の１つはもちろんですが自動化されたテストの実行です。デプロイステージの後に実行されるインテグレーションテストや E2E テストのようなものを追加するには、それらのテストを実行するコマンドを `test_commands` セクションに記述します。すべてのテストが成功すると、次のステージへと進みます。

`test_commands` を追加すると、[aws/codebuild/amazonlinux2-x86_64-standard:3.0](https://docs.aws.amazon.com/ja_jp/codebuild/latest/userguide/build-env-ref-available.html) イメージを利用する CodeBuild プロジェクトが生成されます。例えば `make` のようなコマンドをはじめとして、Amazon Linux 2 で利用可能な多くのコマンドを利用できます。

テストの実行を Docker コンテナの中で実行するように設定していますか？Copilot では CodeBuild プロジェクトの Docker サポートを利用できますので、`docker build` コマンドも同様に利用可能です。

以下の例では、Pipeline は `make test` コマンドをソースコードディレクトリにて実行し、コマンドが正常に終了した場合のみ prod ステージに進みます。

```yaml
name: demo-api-frontend-main
version: 1
source:
  provider: GitHub
  properties:
    branch: main
    repository: https://github.com/kohidave/demo-api-frontend

stages:
    -
      name: test
      # make test コマンドと echo コマンドが正常に終了した場合のみ
      # prod ステージに進みます
      test_commands:
        - make test
        - echo "woo! Tests passed"
    -
      name: prod
```
