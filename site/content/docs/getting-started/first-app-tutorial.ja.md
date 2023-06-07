# 最初のアプリケーションをデプロイしよう

AWS Copilot は、わずかなステップで簡単にコンテナを AWS にデプロイできます。このチュートリアルでは、サンプルのフロントエンドサービスをデプロイし、ブラウザでアクセスできるようにしてみます。この例では静的な Web サイトのサンプルを使用しますが、AWS Copilot を使用して Dockerfile を使用した任意のコンテナアプリを構築し、デプロイすることもできます。サービスのセットアップ完了後、Copilot が作成したリソースを削除して料金が発生しないようにする方法もご紹介します。

いかがですか？それでは早速やってみましょう！

## Step 1: AWS Copilot のダウンロードと設定

AWS Copilot を使用するには、AWS Copilot のバイナリ、AWS CLI、Docker Desktop（起動している必要があります）、AWS の認証情報などが必要です。

これらのツールのセットアップと設定方法については、こちらをご覧ください。

最初に `default` の AWS プロファイルがあることを確認してください。必要な場合は `aws configure` コマンドを実行して、デフォルトのプロファイルを設定してください。

## Step 2: デプロイするコードをダウンロードする

この例では、シンプルな静的 Web サイトのサンプルアプリを使用しますが、デプロイしたいものがある場合は、ターミナルを開いて Dockerfile が格納されているディレクトリへ `cd` コマンドで移動してください。

デプロイしたいものがなければ、サンプルリポジトリをクローンしてください。ターミナルで以下のコマンドをコピー＆ペーストしてください。これによってサンプルアプリのクローンが作成され、ディレクトリが変更されます。

```bash
$ git clone https://github.com/aws-samples/aws-copilot-sample-service example
$ cd example
```

## Step 3: アプリの設定をする

さて、ここからがお楽しみの始まりです。サービスコードと Dockerfile を用意して、それらを AWS にデプロイしたいと思います。それでは、AWS Copilot に手伝ってもらいましょう。

!!! Attention
    既存に他の目的で作成した `copilot/` ディレクトリがある場合、Copilot がそのディレクトリにファイルを作成することがあります。このような場合は、作業ディレクトリ付近に `copilot/` という名前の空のディレクトリを作成することができます。Copilot はこの空のディレクトリを代わりに使用します。

コードディレクトリ内で次のコマンドを実行します。

```bash
$ copilot init
```

<img width="826" alt="gettingstarted" src="https://user-images.githubusercontent.com/879348/86040246-8d304400-b9f8-11ea-9590-2878c3a1d3de.png">

## Step 4: いくつかの質問に答える

次にすることは、Copilot からのいくつかの質問に答えることです。Copilot はこれらの質問を基に、お客様のサービスに最適な AWS インフラストラクチャを選択します。質問はいくつかありますので、順を追って説明します。

1. _“What would you like to name your application?” (Application の名前は？)_ - ここでの Application とは、Service の集合体のことを指します。この例では Application には 1 つの Service しかありませんが、マルチサービスの Application を作りたい場合は Copilot で簡単に実現できます。ここでは、この Application の名前を **example-app** としましょう。
2. _“Which service type best represents your service's architecture?” (Service のアーキテクチャを最もよく表している Service タイプはどれですか？)_ - Copilot は、この Application の Service に何をさせたいかを尋ねています。プライベートなバックエンドサービスでしょうか? ここでは、Application を Web からアクセスできるようにしたいので、Enter キーを押して **Load Balanced Web Service** を選択してみましょう。
3. _“What do you want to name this Load Balanced Web Service?” (この Load Balanced Web Service の名前は？)_ - さて、Application 内の Service を何と呼びますか? お好きな名前を付けてください。しかし、ここではこの Service を **front-end** と名付けることをお勧めします。
4. _“Which Dockerfile would you like to use for front-end?” (フロントエンドにどの Dockerfile を使用しますか？)_ - ここでは、デフォルトの Dockerfile を選択してください。これは Copilot がビルドしてデプロイする Service で使う Dockerfile です。

Dockerfile を選択すると、Copilot は Service を管理するための AWS インフラのセットアップを開始します。
<img width="826" alt="init" src="https://user-images.githubusercontent.com/879348/86040314-ab963f80-b9f8-11ea-8de6-c8caea8f6abf.png">

## Step 5: Service をデプロイする

Copilot が Application を管理するためのインフラの設定を終えると、Service をテスト環境にデプロイするかどうかを尋ねられますので、**yes** と入力します。

ここで、Copilot が Service を実行するために必要なすべてのリソースをセットアップする間、数分待つでしょう⏳。Service のためのすべてのインフラがセットアップされると、Copilot はイメージを構築して Amazon ECR にプッシュし、Amazon ECS on AWS Fargate へのデプロイを開始します。

デプロイが完了すると、Service が稼働し、Copilot が URL へのリンクを出力します！🎉
<img width="834" alt="deploy" src="https://user-images.githubusercontent.com/879348/86040356-be107900-b9f8-11ea-82cd-3bf2a5eb5c9d.png">

## Step 6: クリーンアップ

Service のデプロイが完了したので、`copilot app delete` を実行します。これにより、ECS サービスや ECR リポジトリなど、Copilot が Application に設定したリソースがすべて削除されます。すべてを削除するには、次のように実行します。

```bash
$ copilot app delete
```

<img width="738" alt="delete" src="https://user-images.githubusercontent.com/879348/86040380-c9fc3b00-b9f8-11ea-87c2-6d42518d39dd.png">

## おめでとうございます！

おめでとうございます！AWS Copilot を使って、コンテナアプリを設定し、Amazon ECS on Fargate にデプロイし、削除する方法を学びました。AWS Copilot は、AWS 上でコンテナアプリを開発、リリース、運用するためのコマンドラインツールです。

Application のデプロイを楽しんでいただけましたか？AWS Copilot をより深く理解し、AWS 上で本番環境に対応したコンテナアプリを構築・管理する方法を学ぶ準備はできましたか？引き続きサイドバーの _Developing_ セクションをご覧ください。
