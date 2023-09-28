---
title: 'AWS Copilot v1.29: Pipeline テンプレートのオーバライドと CloudFront キャッシュの無効化'
twitter_title: 'AWS Copilot v1.29'
image: ''
image_alt: ''
image_width: '1051'
image_height: '747'
---

# AWS Copilot v1.29: AWS Copilot v1.29: Pipeline テンプレートのオーバライドと CloudFront キャッシュの無効化！

投稿日: 2023 年 7 月 20 日

AWS Copilot コアチームは Copilot v1.29 のリリースを発表します。

本リリースにご協力いただいた [@tjhorner](https://github.com/tjhorner)、[@build-with-aws-copilot](https://github.com/build-with-aws-copilot) に感謝します。 
私たちのパブリックな[コミュニティチャット](https://app.gitter.im/#/room/#aws_copilot-cli:gitter.im) は成長しており、オンラインでは 500 人以上、[GitHub](http://github.com/aws/copilot-cli/) では 2.9k 以上のスターを獲得しています。
AWS Copilot へご支援、ご支持いただいている皆様お一人お一人に感謝をいたします。

Copilot v1.29 ではより柔軟で効率的な開発を支援する大きな機能強化が行われました:

- **Pipeline オーバーライド**: [v1.27.0](https://aws.github.io/copilot-cli/ja/blogs/release-v127/#copilot-aws-cloudformation)では、ワークロードと Environment の CloudFormation テンプレートに対する CDK と YAML パッチオーバーライドを導入しました。今回のリリースで、Copilot Pipeline テンプレートに対しても同様の拡張性を利用できる様になりました！[詳細セクションはこちらをご覧ください](#pipeline-overrides)。
- **Static Site 機能拡張**: CloudFront キャッシュの無効化と Static Site に合わせた運用コマンドで[最近追加されたワークロードタイプ](https://aws.github.io/copilot-cli/ja/blogs/release-v128/#static-site-service-type)を改善しました。[詳細セクションはこちらをご覧ください](#static-site-enhancements)。

???+ note "AWS Copilot とは？"

    AWS Copilot CLI は AWS 上でプロダクションレディなコンテナ化されたアプリケーションのビルド、リリース、そして運用のためのツールです。
    開発のスタートからステージング環境へのプッシュ、本番環境へのリリースまで、Copilot はアプリケーション開発ライフサイクル全体の管理を容易にします。
    Copilot の基礎となるのは、 AWS CloudFormation です。CloudFormation により、インフラストラクチャを 1 回の操作でコードとしてプロビジョニングできます。
    Copilot は、さまざまなタイプのマイクロサービスの作成と運用の為に、事前定義された CloudFormation テンプレートと、ユーザーフレンドリーなワークフローを提供します。
    デプロイメントスクリプトを記述する代わりに、アプリケーションの開発に集中できます。

    より詳細な AWS Copilot の紹介については、[Overview](../docs/concepts/overview.ja.md) を確認してください。

<a id="#pipeline-overrides"></a>
## Pipeline オーバーライド
Copilot Pipeline は CDK と YAML パッチオーバライドにより、より軽快で拡張性があります！この機能は Pipeline の CloudFormation テンプレートを安全かつ簡単に変更する方法を提供します。
他のオーバライドコマンドと同様に、`copilot pipeline override` を実行し、CloudFormation テンプレートのカスタマイズを行います。CDK または YAML が使用できます。
`copilot pipeline deploy` に対する新しい `--diff` フラグは、デプロイを実施する前に、最後にデプロイした CloudFormation テンプレートとローカルでの変更との変更点についてプレビューできます。プレビュー後、Copilot は処理を継続するか確認します。確認をスキップする場合は、`copilot pipeline deploy --diff --yes` の様に `--yes` フラグを使用します。

オーバライドについてより学び、サンプルを確認する場合は、[CDK オーバライドガイド](../docs/developing/overrides/cdk.ja.md) と [YAML パッチオーバライドガイド](../docs/developing/overrides/yamlpatch.ja.md)を確認してください。

<a id="#static-site-enhancements"></a>
## Static Site の拡張
よりダイナミックな開発のために、Copilot は、Static Site ワークロードを再デプロイする度に CloudFront エッジキャッシュを無効化する様になりました。更新されたコンテンツをすぐに確認し、配信できます。

運用コマンドには Static Site 向けの追加項目があります：
Static Site ワークロードにおける `copilot svc show` コマンドは S3 バケットのコンテンツをツリー形式で表示する様になりました。

```console
Service name: static-site
About

  Application  my-app
  Name         static-site
  Type         Static Site

Routes
  Environment  URL
  -----------  ---
  test         https://d399t9j1xbplme.cloudfront.net/

S3 Bucket Objects

  Environment  test
.
├── ReadMe.md
├── error.html
├── index.html
├── Images
│   ├── SomeImage.PNG
│   └── AnotherImage.PNG
├── css
│   ├── Style.css
│   ├── all.min.css
│   └── bootstrap.min.css
└── images
     └── bg-masthead.jpg
```

Static Site ワークロードにおける `copilot svc status` コマンドでは、S3 バケットのオブジェクト数、合計サイズを表示します。

## 次は？

以下のリンクより、新しい Copilot CLI バージョンをダウンロードし、[GitHub](https://github.com/aws/copilot-cli/) や [コミュニティチャット](https://gitter.im/aws/copilot-cli)にフィードバックを残してください。

- [最新 CLI バージョン](../docs/getting-started/install.ja.md)のダウンロード
- [スタートガイド](../docs/getting-started/first-app-tutorial.ja.md)を試す
- [GitHub](https://github.com/aws/copilot-cli/releases/tag/v1.29.0) でリリースノートの全文を読む


今回のリリースの翻訳はソリューションアーキテクトの浅野が担当しました。