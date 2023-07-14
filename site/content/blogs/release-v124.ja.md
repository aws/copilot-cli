---
title: 'AWS Copilot v1.24: ECS Service Connect!'
twitter_title: 'AWS Copilot v1.24'
image: ''
image_alt: ''
image_width: '1051'
image_height: '747'
---

# AWS Copilot v1.24: ECS Service Connect!

投稿日: 2022 年 11 月 28 日

AWS Copilot コアチームは Copilot v1.24 リリースを発表します。
私たちのパブリックな[コミュニティチャット](https://gitter.im/aws/copilot-cli)は成長しており、オンラインでは 350 人以上、[GitHub](http://github.com/aws/copilot-cli/) では 2.5k 以上のスターを獲得しています。
AWS Copilot へご支援、ご支持いただいている皆様お一人お一人に感謝をいたします。

Copilot v1.24 では、いくつかの新機能と改良が施されています。

- **ECS Service Connect のサポート**: [詳細はこちらを確認してください。](#ecs-service-connect-support)
- **`env deploy` に `--no-rollback` を追加**: Copilot `env deploy` コマンドは新しいフラグ `--no-rollback` をサポートします。フラグを指定して、デバッキングの為に、Envrionment のデプロイにおける自動ロールバックを無効化できます。
- **Request-Driven Web Service のオートスケーリング設定**: Request-Driven Web Service のオートスケーリング設定を指定できる様になりました。例えば、Service の Manifest において、次の様に設定できます。
```yaml
count: high-availability/3
```
- **VPC フローログのログ保持期間の指定**: デフォルトでは 14 日間です。
```yaml
network:
  vpc:
    flow_logs: on
```
 または、保持期間を変更できます。
```yaml
network:
  vpc:
    flow_logs:
      retention: 30
```


???+ note "AWS Copilot とは?"

    AWS Copilot CLI は AWS 上でプロダクションレディなコンテナ化されたアプリケーションのビルド、リリース、そして運用のためのツールです。
    開発のスタートからステージング環境へのプッシュ、本番環境へのリリースまで、Copilot はアプリケーション開発ライフサイクル全体の管理を容易にします。
    Copilot の基礎となるのは、 AWS CloudFormation です。CloudFormation により、インフラストラクチャを 1 回の操作でコードとしてプロビジョニングできます。
    Copilot は、さまざまなタイプのマイクロサービスの作成と運用の為に、事前定義された CloudFormation テンプレートと、ユーザーフレンドリーなワークフローを提供します。
    デプロイメントスクリプトを記述する代わりに、アプリケーションの開発に集中できます。

    より詳細な AWS Copilot の紹介については、[Overview](../docs/concepts/overview.ja.md) を確認してください。

<a id="ecs-service-connect-support"></a>

## ECS Service Connect のサポート
[Copilot は](../docs/developing/svc-to-svc-communication.ja.md#service-connect) 新しくリリースされた [ECS Service Connect](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/service-connect.html)サポートしています! サービスディスカバリよりも Service Connect の方が、より弾力的で、負荷分散されたプライベートなサービス間のコミュニケーションを実現します。Copilot がどの様に ECS Service Connect をサポートしているかウォークスルーしましょう。

### (任意項目) サンプル Service のデプロイ
デプロイされた既存の Service が無い場合は、[チュートリアル](../docs/getting-started/first-app-tutorial.ja.md) に従って、ブラウザーからアクセスできる簡単なフロントエンド Service をデプロイしましょう。 

### Service Connect の設定
サービスディスカバリに加えて、Manifest に次の様な設定すると、Service Connect を設定できます。

```yaml
network:
  connect: true
```

!!! attention
    Service Connect を使う為に、 サーバおよびクライアント Service 共に Service Connect を有効化する必要があります。

### 作成されたエンドポイントの確認
更新した Manifest を使ったデプロイが成功した後は、Service Connect が Service に対して有効化されているはずです。Service のエンドポイント URL を取得するには、`copilot svc show` コマンドを実行します。

```
$ copilot svc show --name front-end

...
Internal Service Endpoints

  Endpoint                      Environment  Type
  --------                      -----------  ----
  front-end:80                  test         Service Connect
  front-end.test.demo.local:80  test         Service Discovery
...
```
上記のように、`front-end:80` は他のクライアント Service が呼び出すことのできる Service Connect のエンドポイントです。(これらの Service も同様に Service Connect を有効にしておく必要があります。)

### (任意項目) Service Connect を検証する
Service Connect のエンドポイント IP アドレスがサービスネットワークに追加されたことを確認する為には、 `copilot svc exec` を使用してコンテナ内部に入り、hosts ファイルを確認します。

```
$ copilot svc exec --name front-end
Execute `/bin/sh` in container frontend in task a2d57c4b40014a159d3b2e3ec7b73004.

Starting session with SessionId: ecs-execute-command-088d464a5721fuej3f
# cat /etc/hosts
127.0.0.1 localhost
10.0.1.253 ip-10-0-1-253.us-west-2.compute.internal
127.255.0.1 front-end
2600:f0f0:0:0:0:0:0:1 front-end
# exit


Exiting session with sessionId: ecs-execute-command-088d464a5721fuej3f.
```

## 次は？

以下のリンクより、新しい Copilot CLI バージョンをダウンロードし、[GitHub](https://github.com/aws/copilot-cli/) や [コミュニティチャット](https://gitter.im/aws/copilot-cli)にフィードバックを残してください。

- [最新 CLI バージョン](../docs/getting-started/install.ja.md)のダウンロード
- [スタートガイド](../docs/getting-started/first-app-tutorial.ja.md)を試す
- [GitHub](https://github.com/aws/copilot-cli/releases/tag/v1.24.0) でリリースノートの全文を読む


今回のリリースの翻訳はソリューションアーキテクトの浅野が担当しました。