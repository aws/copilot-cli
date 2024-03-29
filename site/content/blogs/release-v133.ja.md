---
title: 'AWS Copilot v1.33: run local `--use-task-role` と run local `depends_on` のサポート'
twitter_title: 'AWS Copilot v1.33'
image: ''
image_alt: ''
image_width: '1051'
image_height: '747'
---

# AWS Copilot v1.33: run local `--use-task-role` と run local `depends_on` のサポート

投稿日: 2024 年 1 月 8 日

AWS Copilot コアチームは Copilot v1.33 のリリースを発表します。

私たちのパブリックな[コミュニティチャット](https://app.gitter.im/#/room/#aws_copilot-cli:gitter.im)は成長しており、オンラインでは 500 人以上、[GitHub](http://github.com/aws/copilot-cli/) では 3,100 以上のスターを獲得しています 🚀。
AWS Copilot へご支援、ご支持いただいている皆様お一人お一人に感謝をいたします。

Copilot v1.33 ではより柔軟で効率的な開発を支援する大きな機能強化が行われました:

- **run local `--use-task-role`**: `--use-task-role` フラグにより、ECS タスクロールを使用したローカルテスト体験が向上しました。詳細は、[こちらのセクションを](#use-ecs-task-role-for-copilot-run-local)をご参照ください。
- **run local `depends_on` support**:  ローカルでのコンテナ実行時に Service Manifest 内の `depends_on` を考慮する様になりました。詳細は[こちらのセクションを](#container-dependencies-support-for-copilot-run-local)ご参照ください。

??? note "AWS Copilot とは？"

    AWS Copilot CLI は AWS 上でプロダクションレディなコンテナ化されたアプリケーションのビルド、リリース、そして運用のためのツールです。
    開発のスタートからステージング環境へのプッシュ、本番環境へのリリースまで、Copilot はアプリケーション開発ライフサイクル全体の管理を容易にします。
    Copilot の基礎となるのは、 AWS CloudFormation です。CloudFormation により、インフラストラクチャを 1 回の操作でコードとしてプロビジョニングできます。
    Copilot は、さまざまなタイプのマイクロサービスの作成と運用の為に、事前定義された CloudFormation テンプレートと、ユーザーフレンドリーなワークフローを提供します。
    デプロイメントスクリプトを記述する代わりに、アプリケーションの開発に集中できます。

    より詳細な AWS Copilot の紹介については、[Overview](../docs/concepts/overview.ja.md) を確認してください。

<a id="use-ecs-task-role-for-copilot-run-local"></a>
## `copilot run local` 実行時の ECS タスクロールの利用

`copilot run local` コマンドで `--use-task-role` フラグが利用できます。フラグを有効にした場合、Copilot はデプロイした Service から IAM 権限を取得し、`run local` で作成したコンテナに注入します。
これは、コンテナがクラウド上で実行されている場合と同じ権限を利用する事を意味しており、 より正確にテストが行えます。

<a id="container-dependencies-support-for-copilot-run-local"></a>
## `copilot run local` 実行時の コンテナ依存関係のサポート

`copilot run local` コマンド実行時に、Service Manifest 内で指定された [`depends_on`](../docs/manifest/lb-web-service.md#image-depends-on) を考慮する様になりました。

例:

```
image:
  build: ./Dockerfile
  depends_on:
    nginx: start

nginx:
  image:
    build: ./web/Dockerfile
    essential: true
    depends_on:
      startup: success

startup:
  image:
    build: ./front/Dockerfile
    essential: false
```

この例では、nginx サイドカーコンテナが起動した後にメインコンテナが起動します。また、startup コンテナが正常に完了した後に nginx コンテナが起動します。
