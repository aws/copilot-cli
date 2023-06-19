コンテナの素晴らしい特徴の１つは、コードを書き終わったらそれを簡単に `docker run` コマンドでローカル環境にて実行できることです。Copilot は、`copilot init` コマンドでこのコンテナの AWS 上での容易な実行を可能にします。コンテナイメージをビルド、Amazon ECR へのプッシュ、サービスのスケーラブルかつセキュアな実行に必要なインフラストラクチャのセットアップという一連の流れが Copilot によって簡単に実現できます。

## Service の作成

Service を作成して AWS 上でコンテナを実行するための方法は複数あります。もっとも簡単な方法は、Dockerfile が置かれたディレクトリで `init` コマンドを実行することです。

```console
$ copilot init
```

このコマンドを実行すると、これから作る Service をどの Application に所属させたいかを尋ねられます。Application が存在しない場合は新規での作成を促されます。その後、Copilot はあなたが作ろうとしている Service の __タイプ__ を尋ねます。

Service のタイプを選択すると、Copilot は Dockerfile 内で記述されたヘルスチェックや開放するポート番号を検知し、そのままデプロイするかどうかをあなたに確認します。

## Service タイプの選択

ここに辿り着くまでの間に、「Copilot は Service の実行に必要なインフラストラクチャリソースを自動的にセットアップする」といった説明をしてきました。では、Copilot はどのようにして必要なインフラストラクチャが何かを知るのでしょうか？

その秘密は、Service の作成時に Copilot が Service の __タイプ__ を尋ねていることにあります。

### インターネットから接続可能な Service

インターネットからアクセス可能な Service を作る際の選択肢には次の 3 つがあります。

* "Request-Driven Web Service" - Service 実行環境として AWS App Runner サービスを作成します。
* "Static Site" - 静的 Web サイト用に専用の CloudFront ディストリビューションと S3 バケットをプロビジョニングします。
* "Load Balanced Web Service" - Service 実行環境として Appplication Load Balancer (ALB)、Network Load Balancer、またはその両方を作成し、セキュリティグループ、ECS サービス (Fargate) を利用します。

#### Request-Driven Web Service
AWS App Runner を利用する Service で、受け付けるトラフィックに応じてオートスケールし、トラフィックがない場合は設定された最低インスタンス数までスケールダウンします。リクエスト量の大きな変化や恒常的な少ないリクエスト量が見込まれる HTTP サービスにとってもよりコスト効率の高い選択肢です。

ECS とは異なり、 App Runner サービスはデフォルトでは VPC とは接続されていません。 Egress トラフィックを VPC 経由でルーティングするには、
Manifest 内の[`network`](../manifest/rd-web-service.ja.md#network)フィールドを設定します。

#### Static Site
Amazon CloudFront で配信され、S3 でホスティングされた静的 Web サイトです。[CloudFront コンテンツ配信ネットワーク (CDN)](../developing/content-delivery.ja.md) を使用したキャッシングにより、コストと速度を最適化します。Copilot は、静的 Web サイトホスティング用に構成された新しい S3 バケットに静的アセットをアップロードします。

#### Load Balanced Web Service
Application Load Balancer、Network Load Balancer、または両方をトラフィックの入り口として Fargate 上でタスクを実行する ECS サービスです。
安定したリクエスト量が見込まれる場合、Service から VPC 内のリソースにアクセスする必要がある場合、あるいはより高度な設定の必要がある場合に適した HTTP または TCP サービスの選択肢です。

Application Load Balancer は Environment レベルのリソースであり、Environment 内の全ての Load Balanced Web Service で共有されることに注意しましょう。
詳細については、[こちら](environments.ja.md#load-balancers-and-dns)を確認してください。対照的に、 Network Load Balancer は Service レベルのリソースであり、 Service 間では共有されません。

下図は Application Load Balancer のみを含む Load Balanced Web Service の図です。

![lb-web-service-infra](https://user-images.githubusercontent.com/879348/86045951-39762880-ba01-11ea-9a47-fc9278600154.png)

### Backend Service

VPC 外部からアクセスさせる必要はないが、Application 内の他の Service からはアクセスできる必要があるという場合は、 __Backend Service__ を
作りましょう。Copilot は AWS Fargate で実行される ECS サービスを作成しますが、インターネットに向けて開放されたエンドポイントを作成することはありません。内部ロードバランサーを利用する Backend Service について知りたい場合は、[こちら](../developing/internal-albs.ja.md)を確認してください。

![backend-service-infra](https://user-images.githubusercontent.com/879348/86046929-e8673400-ba02-11ea-8676-addd6042e517.png)

### Worker Service
__Worker Services__ は [pub/sub アーキテクチャ](https://aws.amazon.com/pub-sub-messaging/)による非同期のサービス間通信を実装できます。

アプリケーション内のマイクロサービスはイベントを [Amazon SNS トピック](https://docs.aws.amazon.com/sns/latest/dg/welcome.html) に `パブリッシュ` でき、それを "Worker Service" がサブスクライバーとして受け取ることができます。

Worker Service は次の要素で構成されます。

  * トピックにパブリッシュされた通知を処理する 1 つまたは複数の [Amazon SQS キュー](https://docs.aws.amazon.com/ja_jp/AWSSimpleQueueService/latest/SQSDeveloperGuide/welcome.html)と、失敗した通知を処理する [デッドレターキュー](https://docs.aws.amazon.com/ja_jp/AWSSimpleQueueService/latest/SQSDeveloperGuide/sqs-dead-letter-queues.html)
  * SQS キューをポーリングし、メッセージを非同期で処理する権限を持つ AWS Fargate 上の Amazon ECS サービス


![worker-service-infra](https://user-images.githubusercontent.com/25392995/131420719-c48efae4-bb9d-410d-ac79-6fbcc64ead3d.png)

## Manifest と設定
<!-- textlint-disable ja-technical-writing/ja-no-weak-phrase -->
`copilot init` コマンドを実行すると、Copilot が `manifest.yml` という名前のファイルを `copilot/[service name]/` ディレクトリ内に作成していることに気づいたかもしれません。この Manifest ファイルは Service 用の共通設定やオプションを持ちます。どのようなオプションがあるかはあなたが選択した Service のタイプによって異なりますが、共通の設定には例えば Service に割り当てるメモリや CPU のリソース量、ヘルスチェック、環境変数といったものが含まれます。
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
Manifest ファイルの仕様について学ぶには、[Manifest](../manifest/overview.ja.md) ページもご覧ください。

## Service のデプロイ

Service をセットアップしたら、あるいは Manifest ファイルに変更を加えたら、deploy コマンドを実行して Service をデプロイできます。

```console
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

```console
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
  test              front-end.test.my-app.local:8080

Variables

  Name                                Environment         Value
  COPILOT_APPLICATION_NAME            test                my-app
  COPILOT_ENVIRONMENT_NAME            test                test
  COPILOT_LB_DNS                      test                my-ap-Publi-1RV8QEBNTEQCW-1762184596.ca-central-1.elb.amazonaws.com
  COPILOT_SERVICE_DISCOVERY_ENDPOINT  test                test.my-app.local
  COPILOT_SERVICE_NAME                test                front-end
```

### Service のステータスを確認したい

Service のすべてのタスクは Healthy だろうか？なにかアラームが発火していないか？など、Service のステータスを確認できると便利です。Copilot では、`copilot svc status` でそのような情報のサマリを確認できます。


```console
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

```console
$ copilot svc logs
37236ed 10.0.0.30 🚑 Health-check ok!
37236ed 10.0.0.30 🚑 Health-check ok!
37236ed 10.0.0.30 🚑 Health-check ok!
37236ed 10.0.0.30 🚑 Health-check ok!
37236ed 10.0.0.30 🚑 Health-check ok!
```
