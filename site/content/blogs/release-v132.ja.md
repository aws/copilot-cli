---
title: 'AWS Copilot v1.32: `run local --proxy`、`run local --watch`、既存 ALB のインポートをサポート'
twitter_title: 'AWS Copilot v1.32'
image: ''
image_alt: ''
image_width: '1051'
image_height: '747'
---

# AWS Copilot v1.32: `run local --proxy`、`run local --watch`、既存 ALB のインポートをサポート

投稿日: 2023 年 11 月 9 日

AWS Copilot コアチームは Copilot v1.32 のリリースを発表します。

私たちのパブリックな[コミュニティチャット](https://app.gitter.im/#/room/#aws_copilot-cli:gitter.im)は成長しており、オンラインでは 500 人以上、[GitHub](http://github.com/aws/copilot-cli/) では 3,100 以上のスターを獲得しています 🚀。
AWS Copilot へご支援、ご支持いただいている皆様お一人お一人に感謝をいたします。

Copilot v1.32 ではより柔軟で効率的な開発を支援する大きな機能強化が行われました:

- **`copilot run local --proxy`**: ローカルコンテナから Environment の Services や RDS インスタンスへのアウトバウンドトラフィックをプロキシします。詳細は、[こちらのセクション](#proxy-with-copilot-run-local)をご参照ください。
- **`copilot run local --watch`**: コードに変更を加えた際に、コンテナを自動的にリビルドします。詳細は、[こちらのセクション](#watch-flag-for-copilot-run-local)をご参照ください。
- **ALB のインポート**: Load Balanced Web Service のフロントに、既存の ALB を利用できます。詳細は、[こちらのセクション](#imported-albs)をご参照ください。

??? note "AWS Copilot とは？"

    AWS Copilot CLI は AWS 上でプロダクションレディなコンテナ化されたアプリケーションのビルド、リリース、そして運用のためのツールです。
    開発のスタートからステージング環境へのプッシュ、本番環境へのリリースまで、Copilot はアプリケーション開発ライフサイクル全体の管理を容易にします。
    Copilot の基礎となるのは、 AWS CloudFormation です。CloudFormation により、インフラストラクチャを 1 回の操作でコードとしてプロビジョニングできます。
    Copilot は、さまざまなタイプのマイクロサービスの作成と運用の為に、事前定義された CloudFormation テンプレートと、ユーザーフレンドリーなワークフローを提供します。
    デプロイメントスクリプトを記述する代わりに、アプリケーションの開発に集中できます。

    より詳細な AWS Copilot の紹介については、[Overview](../docs/concepts/overview.ja.md) を確認してください。

<a id="proxy-with-copilot-run-local"></a>

## `copilot run local` のプロキシ機能
`copilot run local` の新しい `--proxy` フラグにより、ローカルコンテナが Environment にデプロイされた Service と通信できるようになりました。これにより、ローカル開発の体験が向上します。

たとえば、`users` と `orders` という 2 つの Service があり、どちらも [Service Connect](../docs/manifest/lb-web-service.ja.md#network-connect) を有効にしている Environment があるとします。さらに、`orders` にはデータを保存するために [RDS addon](../docs/developing/addons/workload.ja.md) がデプロイされているとします。`copilot run local --proxy --name orders` を実行することで、ローカルの `orders` コンテナは以下のコンポーネントと通信ができます:

- `users` Service: Service Connect URL を利用します (デフォルト: `http://users:<port>`)
- `orders` Service の RDS データベース: DB インスタンス URL (例: `app-env-orders-random-characters.us-west-2.rds.amazonaws.com:5432`) や、DB クラスター URL を利用します

<a id="watch-flag-for-copilot-run-local"></a>

## `copilot run local` の watch フラグ
`--watch` フラグを利用することで、ワークスペースを監視し、コードに変更を加えた際にコンテナを自動的にリビルドできます。これにより、継続的な開発が可能になります。`--watch` フラグを `--proxy` フラグと一緒に利用することで、アプリケーションをリビルドするたびにプロキシを設定する手間を省略できるため、非常に便利です。

<a id="imported-albs"></a>

## ALB のインポート
[Load Balanced Web Service の Manifest](../docs/manifest/lb-web-service.ja.md) で、新しいフィールド `http.alb` がサポートされました。Copilot が Environment 内に新しい Application Load Balancer を作成し、すべての Load Balanced Web Service 間で共有するのではなく、特定の Load Balanced Web Service (LBWS) 用に既存のインターネット向け ALB を指定することもできます。LBWS Manifest に、VPC に存在する既存の ALB の ARN または ALB 名を指定します:

```yaml
http:
  alb: [name or ARN]
```

インポートされた ALB については、Copilot は証明書などの DNS 関連リソースを管理しません。