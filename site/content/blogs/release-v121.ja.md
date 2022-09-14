---
title: 'AWS Copilot v1.21: CloudFront が登場!'
twitter_title: 'AWS Copilot v1.21'
image: 'https://user-images.githubusercontent.com/10566468/184949047-f4f173ae-0b29-47fd-8c0b-14a212029587.png'
image_alt: 'v1.21 Environment Manifest と Lambda アドオン'
image_width: '1051'
image_height: '747'
---

# AWS Copilot v1.21: CloudFront が登場!

投稿日: 2022 年 8 月 17 日

AWS Copilot コアチームは、Copilot v1.21 リリースを発表します。
このリリースに貢献してくれた [@dave-moser](https://github.com/dave-moser)、[@dclark](https://github.com/dclark)、[@apopa57](https://github.com/apopa57) に特別な感謝を捧げます。私たちのパブリックな[コミュニティチャット](https://gitter.im/aws/copilot-cli)は成長しており、オンラインでは 300 人以上、[GitHub](http://github.com/aws/copilot-cli/) では 2.4k 以上のスターを獲得しています。AWS Copilot へご支援、ご支持いただいている皆様お一人お一人に感謝をいたします。

Copilot v1.21 では、いくつかの新機能と改良が施されています。

- **CloudFront を Application Load Balancer に統合**: CloudFront を Load Balanced Web Service の手前にデプロイできるようになりました。[詳細はこちらをご覧ください](#cloudfront-%E3%81%AE%E7%B5%B1%E5%90%88)。
- **Environment セキュリティグループの設定**: Environment Manifest にて Environment セキュリティグループのルールを設定します。[詳しくはこちらをご覧ください](#environment-%E3%82%BB%E3%82%AD%E3%83%A5%E3%83%AA%E3%83%86%E3%82%A3%E3%82%B0%E3%83%AB%E3%83%BC%E3%83%97%E3%81%AE%E8%A8%AD%E5%AE%9A)。
- **ELB アクセスログのサポート**: Load Balanced Web Service に対して、Elastic Load Balancing のアクセスログを有効にします。[詳しくはこちらをご覧ください](#elb-%E3%82%A2%E3%82%AF%E3%82%BB%E3%82%B9%E3%83%AD%E3%82%B0%E3%81%AE%E3%82%B5%E3%83%9D%E3%83%BC%E3%83%88)。
- **`job logs` の改善**: Job のログをたどり、ステートマシンの実行ログを見ることができるようになりました。[詳しくはこちらをご覧ください](./#)。
- **デプロイ前の CloudFormation テンプレートのパッケージ化 Addon**: Copilot は、`copilot svc deploy` コマンドで Addon テンプレートをパッケージ化するようになりました。これにより、Copilot はコンテナ化された Service と一緒に AWS Lambda 関数をデプロイできるようになりました! 導入方法については [Copilotのドキュメント](../docs/developing/addons/package.ja.md) をご覧ください。

???+ note "AWS Copilot とは?"

    AWS Copilot CLI は AWS 上でプロダクションレディなコンテナ化されたアプリケーションのビルド、リリース、そして運用のためのツールです。
    開発のスタートからステージング環境へのプッシュ、本番環境へのリリースまで、Copilot はアプリケーション開発ライフサイクル全体の管理を容易にします。
    Copilot の基礎となるのは、 AWS CloudFormation です。CloudFormation により、インフラストラクチャを 1 回の操作でコードとしてプロビジョニングできます。
    Copilot は、さまざまなタイプのマイクロサービスの作成と運用の為に、事前定義された CloudFormation テンプレートと、ユーザーフレンドリーなワークフローを提供します。デプロイメントスクリプトを記述する代わりに、アプリケーションの開発に集中できます。

    より詳細な AWS Copilot の紹介については、[Overview](../docs/concepts/overview.ja.md) を確認してください。

## CloudFront の統合

Copilot の Environment Manifest への最初の大きな追加の 1 つです! CloudFront は AWS Content Delivery Network (CDN) で、世界中にアプリケーションを配信するのに役立ちます。Environment Manifest に `cdn: true` を設定して `copilot env deploy` を実行するだけで、ディストリビューションを有効にできるようになりました。

### 現在サポートされている機能
- 公開されている Application Load Balancer (ALB) の手前に配置されたディストリビューション
- DDoS 攻撃から保護するために、ALB の Ingress は CloudFront ディストリビューション に制限
- インポートされた証明書、または Copilot で管理された証明書による HTTPS トラフィック

### CloudFront での HTTPS 対応
Copilot は次のプロセスを簡単にします! `app init` にて `--domain` を指定して Application を初期化した場合、必要な証明書が作成されるので、追加の操作は必要ありません。

ホストゾーンに独自の証明書をインポートする場合について、CloudFront 用の正しい証明書をインポートする手順を説明します。

!!!info
    CloudFront では、証明書は `us-east-1` リージョンであることが要求されます。証明書をインポートする際は、このリージョンで証明書を作成するようにしてください。

まず、[AWS Certificate Manager](https://aws.amazon.com/jp/certificate-manager/) で `us-east-1` リージョンに Application 用の証明書を作成します。この証明書に Application に関連する各ドメインを追加する必要があります。証明書を検証したら、Environment Manifest に以下のようなフィールドを追加して CloudFront 用の証明書をインポートします。
```yaml
cdn:
  certificate: arn:aws:acm:us-east-1:${AWS_ACCOUNT_ID}:certificate/13245665-h74x-4ore-jdnz-avs87dl11jd
```
`copilot env deploy` を実行すると、[Route 53](https://aws.amazon.com/jp/route53/) に Copilot で作成した CloudFront ディストリビューションを指す A レコードを作成することができます。マネジメントコンソールにて、そのレコードの仕向け先の設定として `Alias` を選択して、トラフィックのルーティング先にて CloudFront ディストリビューションへのエイリアスを選択し、デプロイされたディストリビューションの CloudFront DNS を入力するだけです。

### CloudFront へのトラフィック制限
CloudFront ディストリビューションを経由するパブリックトラフィックを制限するために、`http` にパブリックロードバランサーのための新しいフィールドが用意されています。

```yaml
http:
  public:
    security_groups:
      ingress:
        restrict_to:
          cdn: true
```
ロードバランサーのセキュリティグループを変更し、CloudFront からのトラフィックのみを受け入れるようにします。

## Environment セキュリティグループの設定
Environment Manifest で、Environment セキュリティグループのルールを設定できるようになりました。  
Environment Manifest におけるセキュリティグループルールテンプレートのサンプルは以下の通りです。
```yaml
network:
  vpc:
    security_group:
      ingress:
        - ip_protocol: tcp
          ports: 80
          cidr: 0.0.0.0/0
      egress:
        - ip_protocol: tcp
          ports: 0-65535
          cidr: 0.0.0.0/0
```
完全な仕様については、[Environment Manifest](../docs/manifest/environment.ja.md#network-vpc-security-group) をご覧ください。

## ELB アクセスログのサポート
ロードバランサーに送信されたリクエストの詳細情報を取得する、Elastic Load Balancing のアクセスログを有効にすることができるようになりました。
アクセスログを有効にするには、いくつかの方法があります。

1. 以下のように Environment Manifest にて `access_logs: true` を指定すると、Copilot が S3 バケットを作成してパブリックロードバランサーがアクセスログを保存するようになります。
```yaml
name: qa
type: Environment

http:
  public:
    access_logs: true 
```
バケット名は `copilot env show --resources` コマンドで表示することもできます。

2. また、独自のバケット名とプレフィックスを利用することも可能です。Copilot はこれらのバケットの詳細を使用して、アクセスログを有効にします。
   Environment Manifest で以下のような設定を指定することで可能です。
```yaml
name: qa
type: Environment

http:
 public:
   access_logs:
     bucket_name: my-bucket
     prefix: my-prefix
```
独自のバケットをインポートする場合、そのバケットが存在し、ロードバランサーがアクセスログを書き込むために必要な [bucket policy](https://docs.aws.amazon.com/ja_jp/elasticloadbalancing/latest/classic/enable-access-logs.html#attach-bucket-policy) を所有していることを確認する必要があります。


## `job logs`
ついに、スケジュールされた Job の実行ログを閲覧、追跡できるようになりました。
Job の実行回数を選択したり、特定のタスク ID でログをフィルタリングしたり、ステートマシンの実行ログを表示するかどうかを選択することができます。
例えば、Job の最後の起動からのログと、すべてのステートマシンの実行データを表示することができます。
```console
$ copilot job logs --include-state-machine
Which application does your job belong to? [Use arrows to move, type to filter, ? for more help]
> app1
  app2
Which job's logs would you like to show? [Use arrows to move, type to filter, ? for more help]
> emailer (test)
  emailer (prod)
Application: app1
Job: emailer
states/app1-test-emailer {"id":"1","type":"ExecutionStarted","details": ...
states/app1-test-emailer {"id":"2","type":"TaskStateEntered","details": ...
states/app1-test-emailer {"id":"3","type":"TaskScheduled","details": ...
states/app1-test-emailer {"id":"4","type":"TaskStarted","details": ...
states/app1-test-emailer {"id":"5","type":"TaskSubmitted","details": ...
copilot/emailer/d476069 Gathered recipients
copilot/emailer/d476069 Prepared email body 
copilot/emailer/d476069 Attached headers
copilot/emailer/d476069 Sent all emails
states/app1-test-emailer {"id":"6","type":"TaskSucceeded","details": ...
states/app1-test-emailer {"id":"7","type":"TaskStateExited","details": ...
states/app1-test-emailer {"id":"8","type":"ExecutionSucceeded","details": ...

```
または、[`copilot job run`](../docs/commands/job-run.ja.md) を使って起動したタスクのログを追うこともできます。
```console
$ copilot job run -n emailer && copilot job logs -n emailer --follow
```
## 次は?

以下のリンクより、新しい Copilot CLI バージョンをダウンロードし、[GitHub](https://github.com/aws/copilot-cli/) や [コミュニティチャット](https://gitter.im/aws/copilot-cli)にフィードバックを残してください。

* [最新 CLI バージョン](../docs/getting-started/install.ja.md)のダウンロード
* [スタートガイド](../docs/getting-started/first-app-tutorial.ja.md)を試す
* [GitHub](https://github.com/aws/copilot-cli/releases/tag/v1.21.0) でリリースノートの全文を読む

今回のリリースの翻訳はソリューションアーキテクトの杉本が担当しました。

