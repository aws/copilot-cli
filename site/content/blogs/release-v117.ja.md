# AWS Copilot v1.17: Request-Driven Web Service のためのトレース

投稿日: 2022 年 4 月 12 日

v1.16 のリリースからまだ1週間しか経っていませんが、The AWS Copilot コアチームは AWS App Runner とともに AWS X-Ray with OpenTelemetry を使った統合トレースのサポートを発表しています。App Runner のリリースについては、[こちら](https://aws.amazon.com/jp/blogs/containers/tracing-an-aws-app-runner-service-using-aws-x-ray-with-opentelemetry/)をご覧ください。Copilot で Request-Driven Web Services のトレースを有効にする方法については、[後述のセクション](#request-driven-web-service-%E3%81%AE%E3%83%88%E3%83%AC%E3%83%BC%E3%82%B9%E3%82%92-aws-x-ray-%E3%81%AB%E9%80%81%E4%BF%A1%E3%81%99%E3%82%8B)をご覧ください。


このリリースに貢献した [@kangere](https://github.com/kangere) に感謝を申し上げます。私たちのパブリックな[コミュニティチャット](https://gitter.im/aws/copilot-cli)は常に成長しており、オンラインでは 270 人以上の方々が日々助け合っています。AWS Copilot へご支援、ご支持いただいている皆様お一人お一人に感謝をいたします。

Copilot v1.17 では、新機能の追加といくつかの改善が行われました:

* **Request-Driven Web Service におけるトレース:** AWS App Runner サービスの AWS X-Ray トレースサポートのリリースに伴い、Request-Driven Web Service の Manifest に `observability.tracing: awsxray` を追加して、AWS X-Ray にトレースを送信することができるようになりました。[詳細はこちら](#request-driven-web-service-%E3%81%AE%E3%83%88%E3%83%AC%E3%83%BC%E3%82%B9%E3%82%92-aws-x-ray-%E3%81%AB%E9%80%81%E4%BF%A1%E3%81%99%E3%82%8B)をご覧ください。
* **スケジュールされたジョブを無効化可能:** Manifest でスケジュールを "none" に設定し、イベントルールを無効にすることで、スケジュールされたジョブを簡単にオフにすることができます。([#3447](https://github.com/aws/copilot-cli/pull/3447))
  ```yaml
  on:
    schedule: "none"
  ```
* **プログレストラッカーの視認性向上:** 現在、プログレストラッカーは、HTTPリスナールールの作成など、より多くの情報を表示しています。([#3430](https://github.com/aws/copilot-cli/pull/3430) および [#3432](https://github.com/aws/copilot-cli/pull/3432))
* **バグフィックス:** パイプライン名候補のカラー化書式を削除 ([#3437](https://github.com/aws/copilot-cli/pull/3437))

このリリースには、破壊的な変更はありません。

## AWS Copilot とは?

AWS Copilot CLI は AWS 上でプロダクションレディなコンテナ化されたアプリケーションのビルド、リリース、そして運用のためのツールです。
開発のスタートからステージング環境へのプッシュ、本番環境へのリリースまで、Copilot はアプリケーション開発ライフサイクル全体の管理を容易にします。
Copilot の基礎となるのは、 AWS CloudFormation です。CloudFormation により、インフラストラクチャを 1 回の操作でコードとしてプロビジョニングできます。
Copilot は、さまざまなタイプのマイクロサービスの作成と運用の為に、事前定義された CloudFormation テンプレートと、ユーザーフレンドリーなワークフローを提供します。
デプロイメントスクリプトを記述する代わりに、アプリケーションの開発に集中できます。

より詳細な AWS Copilot の紹介については、[Overview](../docs/concepts/overview.ja.md) を確認してください。

## Request-Driven Web Service のトレースを AWS X-Ray に送信する
_Contributed by [Wanxian Yang](https://github.com/Lou1415926/)_

Request-Driven Web Service で生成されたトレースを AWS X-Ray に送信することができるようになりました。これにより、Amazon CloudWatch コンソールや AWS X-Ray コンソールを通して、サービスマップやトレースを可視化することができます。

この機能を使うには、まず [AWS Distro for OpenTelemetry](https://aws.amazon.com/jp/otel/?otel-blogs.sort-by=item.additionalFields.createdDate&otel-blogs.sort-order=desc) で Service をインストルメント化する (訳注: 計装、 アプリケーションに計測のためのコードを追加する) 必要があります。[手動インストルメンテーション](https://aws-otel.github.io/docs/getting-started/python-sdk/trace-manual-instr)を行うか、より迅速で簡単なセットアップのためにアプリケーションコードを変更せずに Dockerfile を通して Service を[自動インストルメンテーション](https://aws-otel.github.io/docs/getting-started/python-sdk/trace-auto-instr)することができます。

Service をインストルメント化したら、Request-Driven Web Service の Manifest を変更し、[observability の構成](../docs/manifest/rd-web-service.ja.md#observability)を含めるだけです:
```yaml
observability:
  tracing: awsxray
```

`copilot svc deploy` を実行すると、Service で生成されたトレースが AWS X-Ray に送信され、Service の現在の状態を簡単に測定することができます。


## 次は？

以下のリンクより、新しい Copilot CLI バージョンをダウンロードし、[GitHub](https://github.com/aws/copilot-cli/) や [コミュニティチャット](https://gitter.im/aws/copilot-cli)に
フィードバックを残してください。

* [最新 CLI バージョン](../docs/getting-started/install.ja.md)のダウンロード
* [スタートガイド](../docs/getting-started/first-app-tutorial.ja.md)を試す
* [GitHub](https://github.com/aws/copilot-cli/releases/tag/v1.17.0) でリリースノートの全文を読む

今回のリリースの翻訳はソリューションアーキテクトの杉本が担当しました。

