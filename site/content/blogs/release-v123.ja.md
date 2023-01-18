---
title: 'AWS Copilot v1.23: App Runner プライベートサービス, Aurora Serverless v2 など！'
twitter_title: 'AWS Copilot v1.23'
image: ''
image_alt: ''
image_width: '1051'
image_height: '747'
---

# AWS Copilot v1.23: App Runner プライベートサービス, Aurora Serverless v2 など！

投稿日: 2022 年 11 月 1 日

AWS Copilot コアチームは Copilot v1.23 リリースを発表します。
私たちのパブリックな[コミュニティチャット](https://gitter.im/aws/copilot-cli)は成長しており、オンラインでは 300 人以上、[GitHub](http://github.com/aws/copilot-cli/) では 2.5k 以上のスターを獲得しています。
AWS Copilot へご支援、ご支持いただいている皆様お一人お一人に感謝をいたします。

Copilot v1.23 では、いくつかの新機能と改良が施されています。

- **App Runner プライベートサービス**: App Runner でプライベートサービスの起動がサポートされました。Request-Driven Web Service Manifest に `http.private` を追加すると、App Runner プライベートサービスを作成できます。[詳細はこちらを確認してください。](#app-runner-private-services)
- **`storage init` で Aurora Serverless v2 をサポート**: [詳細はこちらを確認してください。](#support-aurora-serverless-v2-in-storage-init)
- **Environment Manifest における誤った `http` フィールドの移動（後方互換性あり！）:** [詳細はこちらを確認してください。](#move-misplaced-http-fields-in-environment-manifest-backward-compatible)
- **ルートファイルシステムへのコンテナアクセスを読み取り専用に制限:** [詳細はこちらを確認してください。](../docs/manifest/lb-web-service.ja.md#storage-readonlyfs) ([#4062](https://github.com/aws/copilot-cli/pull/4062)).
- **ALB の HTTPS レイヤーに対する SSL ポリシーを設定します:** [詳細はこちらを確認してください。](../docs/manifest/environment.ja.md#http-public-sslpolicy) ([#4099](https://github.com/aws/copilot-cli/pull/4099)).
- **ALB に対する ソース IP によるアクセス制限**: [詳細はこちらを確認してください。](../docs/manifest/environment.ja.md#http-public-ingress-source-ips) ([#4103](https://github.com/aws/copilot-cli/pull/4103)).


???+ note "AWS Copilot とは?"

    AWS Copilot CLI は AWS 上でプロダクションレディなコンテナ化されたアプリケーションのビルド、リリース、そして運用のためのツールです。
    開発のスタートからステージング環境へのプッシュ、本番環境へのリリースまで、Copilot はアプリケーション開発ライフサイクル全体の管理を容易にします。
    Copilot の基礎となるのは、 AWS CloudFormation です。CloudFormation により、インフラストラクチャを 1 回の操作でコードとしてプロビジョニングできます。
    Copilot は、さまざまなタイプのマイクロサービスの作成と運用の為に、事前定義された CloudFormation テンプレートと、ユーザーフレンドリーなワークフローを提供します。
    デプロイメントスクリプトを記述する代わりに、アプリケーションの開発に集中できます。

    より詳細な AWS Copilot の紹介については、[Overview](../docs/concepts/overview.ja.md) を確認してください。

<a id="app-runner-private-services"></a>

## App Runner プライベートサービス
Copilot を使って App Runner プライベートサービスを作成出来ます。Request-Driven Web Service の Manifest を更新し、
```yaml
http:
  private: true
```
デプロイするだけです！　その Service は、Copilot Envrionment 内の他の Service からのみ到達できます。
その舞台裏では、Copilot が Envrionment 内の全てのプライベートな Service で共有される APP Runnner の VPC エンドポイントを作成しています。
既存の App Runner VPC エンドポイントがある場合、Manifest に次の様な設定をして、インポート出来ます。
```yaml
http:
  private:
    endpoint: vpce-12345
```
デフォルトでは、プライベートサービスはインターネットにのみトラフィックを送る事ができます。
Environment 内にトラフィックを送りたい場合は、Manifest に [`network.vpc.placement: 'private'`](../docs/manifest/rd-web-service.ja.md#network-vpc-placement) と設定します。

<a id="support-aurora-serverless-v2-in-storage-init"></a>

## [`storage init`](../docs/commands/storage-init.ja.md)で Aurora Serverless v2 をサポート
[Aurora Serverless v2 は今年の初めに一般利用を開始](https://aws.amazon.com/about-aws/whats-new/2022/04/amazon-aurora-serverless-v2/)しており、
現在、Copilot のストレージオプションとしてサポートされています。

以前は、次のコマンドを実行し、
```console
$ copilot storage init --storage-type Aurora
``` 
v1 クラスター用の Addon テンプレートを作成しました。現在は、**このコマンドはデフォルトで v2 用のテンプレートを作成します。**
v1 テンプレートを作成したい場合、`copilot storage init --storage-type Aurora --serverless-version v1` というコマンドを実行します。

より詳しく知りたい場合は、[`storage init` のドキュメント](../docs/commands/storage-init.ja.md)を確認してください！

<a id="move-misplaced-http-fields-in-environment-manifest-backward-compatible"></a>

## Environment Manifest における誤った `http` フィールドの移動（後方互換性あり！）

[Copilot v1.23.0](https://github.com/aws/copilot-cli/releases/tag/v1.23.0) では、 Environment Manifest における `http` フィールド下の階層を修正しました。

### 何が修正されるのか、それは何故なのか？
[Copilot v1.20.0](../blogs/release-v120.ja.md)では、Environment Manifest をリリースし、infrastructure as code の利点を Environment にも取り込みました。当時の `http` フィールド階層は次の様な形です。
```yaml
name: test
type: Environment

http:
  public:
    security_groups:
      ingress:         # [Flaw 1]
        restrict_to:   # [Flaw 2]
          cdn: true
  private:
    security_groups:
      ingress:         # [Flaw 1]
        from_vpc: true # [Flaw 2]
```
この階層設計には、2 つの欠点があります。

1. **`security_groups` 下の `ingress` は曖昧。** 各セキュリティグループには、対応する ingress があります。複数のセキュリティグループの "ingress" が何を意味するのか不明です。*（ここでは、 Copilot がアプリケーションロードバランサーに適用するデフォルトのセキュリティグループを Ingress に設定する事を意味していました。）*


2. **`restrict_to` が冗長。** `http.public` 下の `ingress` は制限され、`http.private` 下の `ingress` は許可されることが明確に示唆されなければなりません。`from_vpc` の `"from"` も同様の冗長性の問題があります。

これらを修正することで、次の様な Envrionment Manifest になります。
```yaml
name: test
type: Environment

http:
  public:
    ingress:
      cdn: true
  private:
    ingress:
      vpc: true
```

### 私はどうすればいいですか？

短い答え: 現時点ではありません。

#### (推奨) 正しい階層に Manifest を修正する
既存の Manifest は動作し続けますが(これについては後述します)、 Manifest を修正された階層に更新することをお勧めします。 以下は、影響を受けるフィールドを更新する方法のスニペットです。

???+ note "正しい階層に Manifest を修正する"

    === "パブリック ALB に対する CDN"

        ```yaml
        # If you have
        http:
          public:
            security_groups:
              ingress:      
                restrict_to: 
                  cdn: true
        
        # Then change it to
        http:
          public:
            ingress:
              cdn: true
        ```

    === "プライベート ALB に対する VPC ingress"
        ```yaml
        # If you have
        http:
          private:
            security_groups:
              ingress:      
                from_vpc: true
        
        # Then change it to
        http:
          private:
            ingress:
              vpc: true
        ```


#### 既存の Manifest は動作し続けます
Envrionment Manifest を正しい階層にすぐに修正しなくても大丈夫です。`http.public.security_groups.ingress`（不備のあるバージョン）と `http.public.ingress`（修正されたバージョン）の両方を含む Manifest に更新しない限り、既存の Manifest は動作し続けます。


例えば、 v1.23.0 のリリース前に、Manifest が次の様なものだったとします。
```yaml
# Flawed hierarchy but will keep working.
http:
  public:
    security_groups:
      ingress:      
        restrict_to: 
          cdn: true
```
同じ Manifest は v1.23.0. 以後も動作し続けるでしょう。

しかし、ある時点で、次の様に Manifest を修正したとします。
```yaml
# Error! Both flawed hierarchy and corrected hierarchy are present.
http:
  public:
    security_groups:
      ingress:      
        restrict_to: 
          cdn: true
    ingress:
      source_ips:
        - 10.0.0.0/24
        - 10.0.1.0/24
```
Copilot は、Manifest に、`http.public.security_groups.ingress`（不備のあるバージョン）と `http.public.ingress`（修正されたバージョン）の両方が存在することを検出します。エラーとなり、修正されたバージョンの `http.public.ingress` だけが存在する様に、Manifest を更新する様な提案が表示されます。
```yaml
# Same configuration but written in the corrected hierarchy.
http:
  public:
    ingress:
        cdn: true
        source_ips:
            - 10.0.0.0/24
            - 10.0.1.0/24
```
## 次は？

以下のリンクより、新しい Copilot CLI バージョンをダウンロードし、[GitHub](https://github.com/aws/copilot-cli/) や [コミュニティチャット](https://gitter.im/aws/copilot-cli)にフィードバックを残してください。

* [最新 CLI バージョン](../docs/getting-started/install.ja.md)のダウンロード
* [スタートガイド](../docs/getting-started/first-app-tutorial.ja.md)を試す
* [GitHub](https://github.com/aws/copilot-cli/releases/tag/v1.23.0) でリリースノートの全文を読む

今回のリリースの翻訳はソリューションアーキテクトの浅野が担当しました。
