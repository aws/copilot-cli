# Application

Application は、Service、Environment、Pipeline といった概念を取りまとめる概念です。あなたのアプリケーションがサービス１つですべてのことをやるものであるか、マイクロサービスの集まりであるかに関係なく、Copilot は Service やそれらがデプロイされる Environment を１つの Application として取りまとめます。

例を見ていきましょう。ここでは投票を受け付け、結果を集計する投票アプリを構築しようとしているとします。

投票の受け付けと結果の集計という２つのサービスを持つ投票アプリは、`copilot init` コマンド２回で構築できます。まず最初に `copilot init` を実行すると、この Service が所属することになる Application の名前を質問されます。ここでは投票システムを構築しようとしていますので、Application を "vote" 、そして投票を受け付ける Service を "collector" と名付けることにしましょう。２回目の `init` では、既存の "vote" Application に新しい Service を追加するために、今度は Service 名のみを質問されます。こちらは集計するサービスですので "aggregator" と名付けることにしましょう。

あなたの Application 設定(ここに複数の Service や Environment が所属します)は、AWS アカウントの中に保存されますので、あなた以外の開発者も "vote" アプリの開発に参加できます。これにより、例えばあなたが１つの Service 開発に取り組む一方で、チームメイトは別の Service 開発を進めることができます。

![](https://user-images.githubusercontent.com/879348/85869625-cd858d00-b780-11ea-817c-638814049d2d.png)

## Application の作成

!!! Attention
    既存に他の目的で作成した `copilot/` ディレクトリがある場合、Copilot がそのディレクトリにファイルを作成することがあります。このような場合は、作業ディレクトリ付近に `copilot/` という名前の空のディレクトリを作成することができます。Copilot はこの空のディレクトリを代わりに使用します。

Application のセットアップは、`copilot init` コマンドで行えます。コマンドを実行すると、新しい Application をセットアップするか、あるいは既存の Application を利用するかを質問されます。

```bash
copilot init
```

Application を作成すると、Copilot はその情報をあなたの AWS アカウント内の SSM パラメータストアに保存します。Application のセットアップに利用した AWS アカウントを「Application アカウント」と呼び、このアカウントにアクセスできる人であれば誰でもその Application の開発に参加できます。

Application の配下に作られる AWS リソースには `copilot-app` という [AWS リソースタグ](https://docs.aws.amazon.com/ja_jp/general/latest/gr/aws_tagging.html) が付与されます。これにより、各リソースがどの Application に所属しているのかを知りやすくなります。

Application の名前はその AWS アカウント内の全てのリージョンにおいて一意である必要があります。

### 追加の Application 設定
`copilot app init` コマンドを利用することでより細かい設定を実施できます。例えば次のようなオプションを設定できます。

* Application、Service、Environment にて作成される AWS リソースに対する [AWS リソースタグ](https://docs.aws.amazon.com/ja_jp/general/latest/gr/aws_tagging.html) を利用した追加のタグ付け
* "Load Balanced Web Service" アーキテクチャ利用時のカスタムドメイン名設定
* Application 内に作られる全てのロールに対して、パーミッションバウンダリーとして設定する既存の IAM ポリシーの指定

```bash
$ copilot app init                                \
  --domain my-awesome-app.aws                     \
  --resource-tags department=MyDept,team=MyTeam   \
  --permissions-boundary my-pb-policy
```

## インフラストラクチャ

Copilot が作成するインフラストラクチャリソースのほとんどは特定の Environment や Service に属しますが、Application 全体にまたがって利用するリソースもいくつかあります。

![](https://user-images.githubusercontent.com/879348/85869637-d0807d80-b780-11ea-8359-6d75933c562a.png)

### ECR リポジトリ

Service で利用するコンテナイメージを格納する ECR リポジトリはリージョン別に作成されます。Application 内の各 Service はそれぞれ専用の ECR リポジトリをリージョンごとに持ちます。

上図では、ある Application が３つのリージョンにそれぞれ Environment を持っていることを示しています。各リージョンには Application 内の Service と同数の ECR リポジトリが作られることになります。この図においては ECR リポジトリが１リージョン内に３つありますので、Service も３つあることが分かります。

新たな Service を追加すると、Copilot は利用対象の各リージョンに ECR リポジトリを作成します。これはあるリージョンでの障害発生が別リージョンで動作するアプリケーションに影響を与えないようにすることと、あるいはリージョンを跨いだイメージダウンロードによるデータ転送料金の発生を避けることを目的としています。

これらの ECR リポジトリは　Environment が作成された AWS アカウントではなく、「Application アカウント」に作成されます。あわせて、各 Environment 用 AWS アカウントからのイメージ pull を許可する IAM ポリシーも設定されます。

### インフラストラクチャのリリース

Copilot は Application 内で利用している全てのリージョンに KMS キーと S3 バケットを作成します。これらのリソースは CodePipeline がリージョン跨ぎ、あるいは AWS アカウント跨ぎのデプロイを行うために利用されます。Application 内の全ての Pipeline はこれらのリソースを共有します。

ECR リポジトリ同様、これらの S3 バケットと KMS キーは同一 AWS アカウント内、あるいは別 AWS アカウント内の各 Environment から暗号化されたデプロイアーティファクトを読むことを許可する IAM ポリシーが設定されています。これにより、CodePipeline がリージョン、アカウントを跨いでデプロイできるようになっています。

## Application の中身を掘り下げてみよう

Application のセットアップが完了したので、Copilot を使って確認してみましょう。確認の手段として以下のような方法がよく利用されます。

### アカウント内に作成された Application の一覧を確認したい

現在のアカウント・リージョン内にある全ての Application を確認するには `copilot app ls` コマンドを利用します。

```bash
$ copilot app ls
vote
ecs-kudos
```

### Application に含まれるものを確認したい

`copilot app show` コマンドを実行すると、Application 内の Service や Environment を含んだサマリ情報を表示します。

```console
$ copilot app show
About

  Name              vote
  Version           v1.1.0
  URI               vote-app.aws

Environments

  Name              AccountID           Region
  ----              ---------           ------
  test              000000000000        us-east-1

Workloads

  Name              Type                        Environments
  ----              ----                        ------------
  collector         Load Balanced Web Service   prod
  aggregator        Backend Service             test, prod

Pipelines

  Name
  ----
```
