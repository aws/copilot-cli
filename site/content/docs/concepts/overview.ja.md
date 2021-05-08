# コンセプト

Copilot を利用すると、簡単にコンテナを AWS 上にセットアップ、デプロイできます。しかし、"Getting Started" セクションで見てきたことは、その旅路の最初のステップでしかありません。同じアプリケーションを１つはテスト環境で、もう１つは本番環境で実行したい場合はどうすればよいでしょうか？新しいサービスを追加したい場合は？追加したサービスも含めたすべてのサービスのデプロイをどのように管理しますか？ここからは Copilot のコアコンセプトの世界に飛び込み、Copilot がどのようにしてこれらのことを手助けしてくれるのか理解していきましょう。

## [Application](./applications.ja.md)
<!-- textlint-disable ja-technical-writing/ja-no-weak-phrase -->
Application は Service と Environment（後述）を包括する概念です。Copilot をこれから使い始めようというとき、あなたが最初に行うことが Application の命名です。Application の名前はあなたが作ろうとしているプロダクトのハイレベルな概要と言えるでしょう。_"frontend"_ と _"api"_ という２つの Service を持つ _"chat"_ という名前の Application、のような例が挙げられますね。例として挙げた _"frontend"_ と _"api"_ の２つの Service は、例えば _"test"_ と _"production"_ という Environment にデプロイされるかもしれません。
<!-- textlint-enable ja-technical-writing/ja-no-weak-phrase -->

## [Environment](./environments.ja.md)

噂によると、バグのない完璧なコードを最初から書くことができる人というのが世の中にはいるそうです。そのような方々には脱帽しますが、私たちは新しいコードを本番環境へのデプロイよりも前に顧客向けではない場所でテストできることが重要だと考えています。Copilot の世界では、私たちは _Environment_
を使ってこれを実現します。各 Environment ではそれぞれ異なるバージョンの<!-- textlint-disable ja-technical-writing/ja-no-redundant-expression -->サービスを実行できる<!-- textlint-enable ja-technical-writing/ja-no-redundant-expression -->ため、テスト環境や本番環境といったものを作れます。テスト環境にサービスをデプロイし、全てが問題ないことを確認したら本番環境にデプロイする、といった流れです。各 Environment は互いに独立しているため、たとえバグをテスト環境に混入させたとしても本番環境を利用しているお客様にとってはなんの影響もありません。

ここまでは単一のサービスについて話をしてきましたが、もしあなたがさらにもう１つサービスを追加したくなったとしたらどうでしょう？ここではすでにあるフロントエンドサービスから利用するバックエンドサービスを足すシチュエーションを想像してみましょう。<!-- textlint-disable ja-technical-writing/max-ten -->各 Environment には、デプロイされた全てのサービスで共有するリソース群、例えばネットワーク（VPC、サブネット、セキュリティグループなど）、ECS クラスタ、ロードバランサのようなものが含まれます。<!-- textlint-enable ja-technical-writing/max-ten -->仮にあなたがフロントエンドとバックエンドサービスの両方を同一の Environment にデプロイすると、それらのサービスは同一のネットワークや ECS クラスタを利用することになります。

## [Service](./services.ja.md)

Copilot における Service は、AWS 上で実行したいあなたのコードとそれに必要なインフラストラクチャリソースを指す概念です。Copilot であなたが Service をセットアップしようとすると、Copilot はどのような「タイプ」の Service を作りたいのかをあなたに尋ねます。この Service の「タイプ」は、あなたのコードを実行するために作成されるインフラストラクチャを決定する要素となります。仮にあなたがインターネット越しのリクエストを受け付けるコードを実行したいと考えているのであれば、Copilot は Application Load Balancer と AWS Fargate を利用する Amazon ECS サービスを作成します。

どのようなタイプのサービスを作っているのかを Copilot に教えると、Copilot はあなたのコードに含まれる Dockerfile からコンテナイメージを作成し、それを Amazon ECR リポジトリへセキュアに格納します。Copilot はそれと同時にあなたの Service の設定情報を格納した _Manifest_ と呼ばれるシンプルなファイルを作成します。Service に割り当てたいメモリや CPU の量、あるいは Service をいくつ並列で実行したいのかといったような設定もここに含まれます。

## [Job](./jobs.ja.md)

Job は、イベントによって起動されるエフェメラルな Amazon ECS タスクを指す概念です。Job の作業が完了するとそのタスクは削除されます。Service 同様、Copilot はスケジュールされたタスクをクイックな作成に必要な情報をあなたに質問します。Manifest ファイルを用意することで任意の、あるいはより高度な設定も可能です。

## [Pipeline](./pipelines.ja.md)

ここまでの概念で、いくつかの Environment にデプロイされた複数の Service を持つ Application を手に入れましたが、このような複数の概念を維持しつつデプロイを行っていくのは大変な作業になりがちです。そこで Copilot では、あなたが Git リポジトリ(現時点では GitHub、BitBucket、CodeCommit をサポート)にコードの変更をプッシュしたら Service をデプロイしてくれるリリースパイプラインをセットアップできます。これが Pipeline という概念です。コードのプッシュが検知されると、Pipeline は Service をビルド、コンテナイメージを ECR にプッシュし、Environment にデプロイします。

パイプラインのセットアップとしてよくみられるパターンの１つは、テスト環境にデプロイしたサービスに対して自動テストを実行してから本番環境にデプロイする、というものです。
