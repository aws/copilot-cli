---
title: 'AWS Copilot v1.28: Static Site Service タイプの登場！'
twitter_title: 'AWS Copilot v1.28'
image: ''
image_alt: ''
image_width: '1051'
image_height: '747'
---

# AWS Copilot v1.28: Static Site Service タイプの登場！

投稿日: 2023 年 5 月 24 日

AWS Copilot コアチームは Copilot v1.28 のリリースを発表します。
本リリースにご協力いただいた [@interu](https://github.com/interu)、[@0xO0O0](https://github.com/0xO0O0)、[@andreas-bergstrom](https://github.com/andreas-bergstrom) に感謝します。
私たちのパブリックな[コミュニティチャット](https://app.gitter.im/#/room/#aws_copilot-cli:gitter.im)は成長しており、オンラインでは 400 人以上、[GitHub](http://github.com/aws/copilot-cli/) では 2.9k 以上のスターを獲得しています。
AWS Copilot へご支援、ご支持いただいている皆様お一人お一人に感謝をいたします。

Copilot v1.28 では、いくつかの新機能と改良が施されています。

- **Static Site Service タイプ**: AWS S3 を使って静的な Web サイトをデプロイし、ホストすることができるようになりました。[詳細はこちらを確認してください。](#static-site-service-type).
- **コンテナイメージの並列ビルド**: メインコンテナとサイドカーコンテナのそれぞれのイメージを並列にビルドできるようになりました。並列ビルドにより、ビルドと AWS ECR へのコンテナイメージのプッシュにかかる全体の時間を短縮することができます。

???+ note "AWS Copilot とは?"

    AWS Copilot CLI は AWS 上でプロダクションレディなコンテナ化されたアプリケーションのビルド、リリース、そして運用のためのツールです。
    開発のスタートからステージング環境へのプッシュ、本番環境へのリリースまで、Copilot はアプリケーション開発ライフサイクル全体の管理を容易にします。
    Copilot の基礎となるのは、 AWS CloudFormation です。CloudFormation により、インフラストラクチャを 1 回の操作でコードとしてプロビジョニングできます。
    Copilot は、さまざまなタイプのマイクロサービスの作成と運用の為に、事前定義された CloudFormation テンプレートと、ユーザーフレンドリーなワークフローを提供します。
    デプロイメントスクリプトを記述する代わりに、アプリケーションの開発に集中できます。

    より詳細な AWS Copilot の紹介については、[Overview](../docs/concepts/overview.ja.md) を確認してください。

<a id="static-site-service-type"></a>
## Static Site Service タイプ
最新のワークロードタイプである Static Site Service は、Amazon S3 によってホストされ Amazon CloudFront ディストリビューションによってコンテンツ配信される静的な Web サイトを作成するために必要なすべてを準備、設定します。

例えば、単純な読み取り専用の Web サイトを立ち上げるとします。バックエンドやデータベースは必要なく、ユーザーに応じてサイトをパーソナライズしたり、情報を保存したりする必要もないでしょう。このような場合、静的なサイトを作る！ということになります。このワークロードのタイプは、比較的シンプルで素早く立ち上げることができ、パフォーマンスも高いです。

### Static Site のアップロード方法
静的アセット (HTML ファイル、CSS や JavaScript などのファイル) を作成したら、[`copilot init`](../docs/commands/init.ja.md) コマンドを、または `copilot app init` と `copilot env init` を実行済みの場合は [`copilot svc init`](../docs/commands/svc-init.ja.md) を使って、静的サイトの作成を開始します。`--sources` フラグを使用して、静的リソースのディレクトリやファイルへのパス (プロジェクトルートからの相対パス) を渡すことができます。またはプロンプトが表示されたら、ディレクトリ/ファイルを選択することもできます。

Manifest が入力され、`copilot/[service name]` フォルダに保存されます。そこで、必要に応じてアセットの仕様を調整することができます。デフォルトでは、すべてのディレクトリが再帰的にアップロードされます。それを望まない場合は、`exclude` と `reinclude` フィールドを活用してフィルターを追加してください。利用可能なパターンシンボルは以下のとおりです。

`*`: 全てにマッチする  
`?`: 任意の 1 文字にマッチする  
`[sequence]`: シーケンスの任意の文字にマッチする  
`[!sequence]`: シーケンスに含まれない文字にマッチする  

```yaml
# "example" Service の Manifest
# "Static Site" タイプの完全な仕様は以下を参照して下さい:
#  https://aws.github.io/copilot-cli/docs/manifest/static-site/

# Service 名は、S3 バケットなどのリソースの命名に使用されます。
name: example
type: Static Site

http:
  alias: 'example.com'

files:
  - source: src/someDirectory
    recursive: true
  - source: someFile.html

# 上記で定義された値は Environment によるオーバーライドが可能です。
# environments:
#   test:
#     files:
#       - source: './blob'
#         recursive: true
#         destination: 'assets'
#         exclude: '*'
#         reinclude:
#           - '*.txt'
#           - '*.png'
```
`exclude` と `reinclude` フィルタの詳細については、[こちら](https://awscli.amazonaws.com/v2/documentation/api/latest/reference/s3/index.html#use-of-exclude-and-include-filters)を参照してください。

[`copilot deploy`](../docs/commands/deploy.ja.md) または [`copilot svc deploy`](../docs/commands/svc-deploy.ja.md) コマンドは、S3 バケットを作成し、選択したローカルファイルをそのバケットにアップロードし、S3 バケットをオリジンとする CloudFront ディストリビューションを生成して、静的 Web サイトをプロビジョニングして起動します。S3 バケットを作成し、選択したローカルファイルをアップロードし、S3 バケットをオリジンとする CloudFront ディストリビューションを生成します。裏では Static Site Service は他の Copilot ワークロードと同様に、CloudFormation スタックを持ちます。

!!! note
    オブジェクトのアップロードは Copilot で管理されるため、Static Site S3 バケットの[サーバーアクセスロギング](https://docs.aws.amazon.com/AmazonS3/latest/userguide/ServerLogs.html)はデフォルトで有効ではありません。

## 次は？

以下のリンクより、新しい Copilot CLI バージョンをダウンロードし、[GitHub](https://github.com/aws/copilot-cli/) や [コミュニティチャット](https://gitter.im/aws/copilot-cli)にフィードバックを残してください。

- [最新 CLI バージョン](../docs/getting-started/install.ja.md)のダウンロード
- [スタートガイド](../docs/getting-started/first-app-tutorial.ja.md)を試す
- [GitHub](https://github.com/aws/copilot-cli/releases/tag/v1.28.0) でリリースノートの全文を読む

今回のリリースの翻訳はソリューションアーキテクトの杉本が担当しました。

