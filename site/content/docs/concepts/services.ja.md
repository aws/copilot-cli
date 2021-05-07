コンテナの素晴らしい特徴の１つは、コードを書き終わったらそれを簡単に `docker run` コマンドでローカル環境にて実行できることです。Copilot は、`copilot init` コマンドでこのコンテナの AWS 上での容易な実行を可能にします。コンテナイメージをビルド、Amazon ECR へのプッシュ、サービスのスケーラブルかつセキュアな実行に必要なインフラストラクチャのセットアップという一連の流れが Copilot によって簡単に実現できます。

## Service の作成

Service を作成して AWS 上でコンテナを実行するための方法は複数あります。もっとも簡単な方法は、Dockerfile が置かれたディレクトリで `init` コマンドを実行することです。

```bash
$ copilot init
```

このコマンドを実行すると、これから作る Service をどの Application に所属させたいかを尋ねられます。Application が存在しない場合は新規での作成を促されます。その後、Copilot はあなたが作ろうとしている Service の __タイプ__ を尋ねます。

Service のタイプを選択すると、Copilot は Dockerfile 内で記述されたヘルスチェックや開放するポート番号を検知し、そのままデプロイするかどうかをあなたに確認します。

## Service タイプの選択

ここに辿り着くまでの間に、「Copilot は Service の実行に必要なインフラストラクチャリソースを自動的にセットアップする」といった説明をしてきました。では、Copilot はどのようにして必要なインフラストラクチャが何かを知るのでしょうか？

その秘密は、Service の作成時に Copilot が Service の __タイプ__ を尋ねていることにあります。

### Load Balanced Web Service

Service でインターネット側からのリクエストを捌きたいですか？__Load Balanced Web Service__ を選べば、Copilot は Application Load Balancer やセキュリティグループ、そしてあなたの Service を Fargate で実行するための ECS サービスを作成します。

![lb-web-service-infra](https://user-images.githubusercontent.com/879348/86045951-39762880-ba01-11ea-9a47-fc9278600154.png)

### Backend Service

VPC 外部からアクセスさせる必要はないが、Application 内の他の Service からはアクセスできる必要があるという場合は、 __Backend Service__ を
作りましょう。Copilot は AWS Fargate で実行される ECS サービスを作成しますが、インターネットに向けて解放されたエンドポイントを作成することはありません。

![backend-service-infra](https://user-images.githubusercontent.com/879348/86046929-e8673400-ba02-11ea-8676-addd6042e517.png)

## Manifest と設定
<!-- textlint-disable ja-technical-writing/ja-no-weak-phrase -->
`copilot init` コマンドを実行すると、Copilot が `manifest.yml` という名前のファイルを copilot ディレクトリ内に作成していることに気づいたかもしれません。この Manifest ファイルは Service 用の共通設定やオプションを持ちます。どのようなオプションがあるかはあなたが選択した Service のタイプによって異なりますが、共通の設定には例えば Service に割り当てるメモリや CPU のリソース量、ヘルスチェック、環境変数といったものが含まれます。
<!-- textlint-enable ja-technical-writing/ja-no-weak-phrase -->

_front-end_ という名前の Load Balanced Web Service 用に作られた Manifest ファイルを覗いてみましょう。

```yaml
name: front-end
type: Load Balanced Web Service

image:
  # Service 用の Dockerfile までのパス
  build: ./Dockerfile
  # コンテナがトラフィックを受け取るために開放するポート番号
  port: 8080

http:
  # このパスに届いたリクエストが Service にルーティングされます
  # すべてのリクエストをルーティングするには "/" をパスとして指定します
  path: '/'
  # 任意のヘルスチェックパスを指定できます. デフォルトは "/" です.
  # healthcheck: '/'

# タスクに割り当てる CPU ユニット数
cpu: 256
# タスクに割り当てるメモリ量 (MiB)
memory: 512
# Service として実行される必要があるタスク数
count: 1

# 以下はより高度なユースケース用のオプション設定項目です
#
variables:                    # キー・値のペアで環境変数を設定します
  LOG_LEVEL: info

#secrets:                         # AWS Systems Manager (SSM) パラメータストア から秘密情報を取得して設定します
#  GITHUB_TOKEN: GH_SECRET_TOKEN  # キーに環境変数名、値には SSM パラメータ名を記述します

# Environment ごとに上記で設定された値を上書きできます
environments:
  prod:
    count: 2               # "prod" Environment ではタスクを2つ実行します
```
Manifest ファイルの仕様について学ぶには、[Manifest](../manifest/overview.md) ページもご覧ください。

## Service のデプロイ

Service をセットアップしたら、あるいは Manifest ファイルに変更を加えたら、deploy コマンドを実行して Service をデプロイできます。

```bash
$ copilot deploy
```

このコマンドを実行すると、続けて以下のような作業が実施されます。

1. ローカル環境でのコンテナイメージのビルド  
2. Service 用の ECR リポジトリへのプッシュ  
3. Manifest ファイルの CloudFormation テンプレートへの変換  
4. 追加インフラストラクチャがある場合、それらの CloudFormation テンプレートへのパッケージング  
5. Service と各種リソースの CloudFormation によるデプロイ  

もしすでに Environment が複数ある場合には、どの Environment に対してデプロイするのかを確認されます。

## Service の中身を掘り下げてみよう

Service のセットアップと実行が完了したので、Copilot を使って確認してみましょう。確認の手段として以下のような方法がよく利用されます。

### Service に含まれるものを確認したい

`copilot svc show` コマンドを実行すると、Service のサマリ情報を表示します。以下は Load Balanced Web Application での出力の例です。各 Environment ごとの Service 設定や Service のエンドポイント、あるいは環境変数などが確認できます。さらに、`--resources` フラグを利用することでこの Service に紐づけられたすべての AWS リソースを確認できます。

```bash
$ copilot svc show
About

  Application       my-app
  Name              front-end
  Type              Load Balanced Web Service

Configurations

  Environment       Tasks               CPU (vCPU)          Memory (MiB)        Port
  test              1                   0.25                512                 80

Routes

  Environment       URL
  test              http://my-ap-Publi-1RV8QEBNTEQCW-1762184596.ca-central-1.elb.amazonaws.com

Service Discovery

  Environment       Namespace
  test              front-end.my-app.local:8080

Variables

  Name                                Environment         Value
  COPILOT_APPLICATION_NAME            test                my-app
  COPILOT_ENVIRONMENT_NAME            test                test
  COPILOT_LB_DNS                      test                my-ap-Publi-1RV8QEBNTEQCW-1762184596.ca-central-1.elb.amazonaws.com
  COPILOT_SERVICE_DISCOVERY_ENDPOINT  test                my-app.local
  COPILOT_SERVICE_NAME                test                front-end
```

### Service のステータスを確認したい

Service のすべてのタスクは Healthy だろうか？なにかアラームが発火していないか？など、Service のステータスを確認できると便利です。Copilot では、`copilot svc status` でそのような情報のサマリを確認できます。


```bash
$ copilot svc status
Service Status

  ACTIVE 1 / 1 running tasks (0 pending)

Last Deployment

  Updated At        12 minutes ago
  Task Definition   arn:aws:ecs:ca-central-1:693652174720:task-definition/my-app-test-front-end:1

Task Status

  ID                Image Digest        Last Status         Health Status       Started At          Stopped At
  37236ed3          da3cfcdd            RUNNING             HEALTHY             12 minutes ago      -

Alarms

  Name              Health              Last Updated        Reason
  CPU-Utilization   OK                  5 minutes ago       -
```

### Service のログを確認したい

Service のログの確認も簡単です。`copilot svc logs` コマンドを実行すると、直近の Service ログを確認できます。`--follow` フラグをあわせて利用すると、Service 側のログの出力をライブに追いかけることもできます。

```bash
$ copilot svc logs
37236ed 10.0.0.30 🚑 Health-check ok!
37236ed 10.0.0.30 🚑 Health-check ok!
37236ed 10.0.0.30 🚑 Health-check ok!
37236ed 10.0.0.30 🚑 Health-check ok!
37236ed 10.0.0.30 🚑 Health-check ok!
```
