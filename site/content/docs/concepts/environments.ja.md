はじめて `copilot init` コマンドを実行すると、あわせて _test_ Environment を作成するかを尋ねられます。この _test_ Environment にはセキュアなネットワーク(e.g. VPC、サブネット、セキュリティグループ)を作成するために必要な AWS リソースや、複数の Service での共有を目的とした Application Load Balancer や ECS クラスタのようなリソースも含まれます。Service をこの _test_ Environment にデプロイすると、この Service は _test_ Environment のネットワークやリソースを利用します。Application は複数の Environment を持つことができ、それぞれが互いに独立したネットワークやその他のインフラストラクチャリソースを持ちます。

あなたの Copilot の利用開始にあわせて Copilot は _test_ Environment (テスト環境)を作成しますが、これとは異なる Environment、例えば _production_ Environment (本番環境)を新たに作るというのはごく一般的なことでしょう。この _production_ Environment は _test_ Environment とは完全に独立したもので、_production_ Environment 用のネットワークスタックやそこにデプロイされる Service を持ちます。独立したテスト環境と本番環境を持つことで、まずはテスト環境にデプロイし、問題ないことを確認した上で本番環境にデプロイするというオペレーションが可能になります。

下に載せた図は、_API_ と _Backend_ という２つの Service を持つ _MyApp_ という Application を表しています。これら２つの Service は _test_ と _production_ という２つの Environment にデプロイされています。_test_ Environment では両方の Service が１つのコンテナを実行している一方で、_production_ Environment ではそれぞれが３つのコンテナを実行しているのが分かるでしょうか。このように、Service はデプロイ先の Environment ごとに異なる設定を持つことができます。詳しくは[環境変数の利用ガイド](../developing/environment-variables.md)もご覧ください。

![](https://user-images.githubusercontent.com/879348/85873795-7da9c480-b786-11ea-9990-9604a3cc5f01.png)

## Environment の作成

Application 内に新しい Environment を作るには、作業ディレクトリにて `copilot env init` コマンドを実行します。コマンドを実行すると、作成する Environment の名前、そしてこの Environment の作成に利用する AWS プロファイルを Copilot が尋ねます。このプロファイルは AWS [名前付きプロファイル](https://docs.aws.amazon.com/ja_jp/cli/latest/userguide/cli-configure-profiles.html)と呼ばれるもので、特定の AWS アカウントやリージョンと紐づいた設定です。プロファイルを選ぶと、その新しい Environment は選択されたプロファイルと紐づいた AWS アカウントとリージョンに作成されます。


```bash
$ copilot env init
```

`copilot env init` コマンドの実行後は、Copilot が数分間に渡って Environment 内のリソースをセットアップしていく様子を確認できます。すべてのリソースが作成されると、この Environment は Application アカウントにリンクされます。これにより、今後 Copilot コマンドを実行するユーザーがこの Environment アカウントそのものにアクセスができなくとも Application にリンクされた Environment として管理できるようになります。このリンク処理の中では、必要に応じて ECR リポジトリの作成と設定も行われます。

### Service のデプロイ

新しい Environment を作った時点ではまだデプロイされた Service はありません。デプロイしたい Service のディレクトリから `copilot deploy` コマンドを実行すると、どの Environment にデプロイしたいのかを尋ねられます。

## Environment インフラストラクチャ

![](https://user-images.githubusercontent.com/879348/85873802-800c1e80-b786-11ea-8b2c-779b01abbaf4.png)


### VPC やネットワークリソース

各 Environment はそれぞれがマルチ AZ 構成の VPC を持ちます。VPC は Environment のネットワーク上の境界であり、VPC 内に入ってくるあるいは出ていくトラフィックを許可する、またはブロックするものとして機能します。Copilot は VPC を２つのアベイラビリティゾーンにまたがって作成するため、可用性とコストのバランス確保に役立ちます。各 AZ にはパブリックとプライベートのサブネットが１つずつ作成されます。

Service はパブリックサブネットで実行されますが、ロードバランサーを経由したトラフィックのみを受け取ることができます。

### ロードバランサーと DNS

"Load Balanced Web Service" タイプの Service を作ると、Copilot は Application Load Balancer を作成します。同一 Environment 内にデプロイされたすべての "Load Balanced Web Service" タイプの Service は、Service 固有のリスナーを作成してこのロードバランサーを共有します。ロードバランサーは VPC 内の各 Service と通信できるようにセットアップされます。

所有するドメイン名を Route 53 に登録するよう、Application 作成時にオプションとして設定できます。ドメイン名の利用が設定されている場合、Copilot は各 Environment の作成時に `environment-name.app-name.your-domain.com` のような形でサブドメインを登録し、ACM を通して発行した証明書を Application Load Balancer に設定します。これにより Service が HTTPS を利用できるようになります。

## Environment のカスタマイズ

既存リソースをインポートするためにコマンドのフラグを利用して Environment をインタラクティブにカスタマイズしたり、あるいはデフォルトの Environment リソースを変更したりできます。現時点では VPC リソースのみがカスタマイズ可能です。他のリソースをカスタマイズしたいというリクエストがある場合、お気軽にユースケースを添えた GitHub イシューを作成してくださいね！

## Environment の中身を掘り下げてみよう

Environment のセットアップが完了したので、Copilot を使って確認してみましょう。確認の手段として以下のような方法がよく利用されます。

### Application 内にある Environment の一覧を確認したい

Application 内にある全ての Environment を確認するには `copilot env ls` コマンドを利用します。

```bash
$ copilot env ls
test
production
```

### Environment に含まれるものを確認したい

`copilot env show` コマンドを実行すると、Environment のサマリ情報を表示します。以下は _test_ Environment についての情報を確認する出力の例ですが、この Environment が作成されたアカウントやリージョン情報、あるいはデプロイされた Service や Environment 内のリソースに付与されるリソースタグ情報などが含まれます。さらに、`--resources` フラグを利用することでこの Environment に紐づけられたすべての AWS リソースを確認できます。

```bash
$ copilot env show --name test
About

  Name              test
  Production        false
  Region            us-west-2
  Account ID        693652174720

Services

  Name              Type
  ----              ----
  api               Load Balanced Web Service
  backend           Backend Service


Tags

  Key                  Value
  ---                  -----
  copilot-application  my-app
  copilot-environment  test
```
