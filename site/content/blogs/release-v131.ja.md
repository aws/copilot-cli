---
title: 'AWS Copilot v1.31: NLB 設定の拡張、タスク失敗時のログの改善、`copilot deploy` の機能拡張'
twitter_title: 'AWS Copilot v1.31'
image: ''
image_alt: ''
image_width: '1051'
image_height: '747'
---

# AWS Copilot v1.31: NLB 設定の拡張、タスク失敗時のログの改善、`copilot deploy` の機能拡張

投稿日: 2023 年 10 月 5 日

AWS Copilot コアチームは Copilot v1.31 のリリースを発表します。

私たちのパブリックな[コミュニティチャット](https://app.gitter.im/#/room/#aws_copilot-cli:gitter.im)は成長しており、オンラインでは 500 人以上、[GitHub](http://github.com/aws/copilot-cli/) では 3k 以上のスターを獲得しています 🚀。
AWS Copilot へご支援、ご支持いただいている皆様お一人お一人に感謝をいたします。

Copilot v1.31 ではより柔軟で効率的な開発を支援する大きな機能強化が行われました:

- **NLB 設定の拡張**: Copilot が管理する [Network Load Balancer (NLB)](../docs/manifest/lb-web-service.ja.md#nlb) にセキュリティグループを追加可能になりました。また、NLB の設定で UDP プロトコルが指定可能になっています。詳細は、[こちらのセクション](#nlb-enhancements)をご参照ください。
- **タスク失敗時のログの改善**: Copilot では、デプロイ中にタスクが失敗した場合に、より詳細なログを提供するようになりました。これにより、トラブルシューティングが容易になります。
- **`copilot deploy` の機能拡張**: 複数のワークロードを一度にデプロイできるようになりました。また、`--all` オプションにより、すべてのローカルワークロードのデプロイも可能になっています。
- **静的サイトへ ACM 証明書のインポート**: 自己署名の ACM 証明書を、静的サイトへインポートできるようになりました。詳細は、[こちらのセクション](#importing-an-acm-certificate-for-your-static-site)をご参照ください。

??? note "AWS Copilot とは？"

    AWS Copilot CLI は AWS 上でプロダクションレディなコンテナ化されたアプリケーションのビルド、リリース、そして運用のためのツールです。
    開発のスタートからステージング環境へのプッシュ、本番環境へのリリースまで、Copilot はアプリケーション開発ライフサイクル全体の管理を容易にします。
    Copilot の基礎となるのは、 AWS CloudFormation です。CloudFormation により、インフラストラクチャを 1 回の操作でコードとしてプロビジョニングできます。
    Copilot は、さまざまなタイプのマイクロサービスの作成と運用の為に、事前定義された CloudFormation テンプレートと、ユーザーフレンドリーなワークフローを提供します。
    デプロイメントスクリプトを記述する代わりに、アプリケーションの開発に集中できます。

    より詳細な AWS Copilot の紹介については、[Overview](../docs/concepts/overview.ja.md) を確認してください。

<a id="nlb-enhancements"></a>

## NLB 設定の拡張
Copilot にて、NLB の設定で UDP プロトコルが指定可能になりました。NLB が使用するプロトコルは [nlb.port](https://aws.github.io/copilot-cli/docs/manifest/lb-web-service/#nlb-port) フィールドで指定します。
```
nlb:
  port: 8080/udp
```

!!!info
    NLB セキュリティグループは、NLB へのパブリックトラフィックをフィルタリングして、アプリケーションのセキュリティを強化できる AWS の新機能です。詳細については、[AWS ブログ](https://aws.amazon.com/blogs/containers/network-load-balancers-now-support-security-groups/)をご参照ください。Copilot でこの機能を利用するために、`NetworkLoadBalancer` および `TargetGroup` リソースを再作成する必要があります。Copilot v1.31 時点では、`udp` プロトコルを指定した場合のみ、`NetworkLoadBalancer` および `TargetGroup` リソースの再作成 (NLB へのセキュリティグループの紐付け) が行われます。ただし、Copilot v1.33 では、`udp` 以外のプロトコルに対してもこの変更が適用されます。すなわち、DNS エイリアスを利用していない場合、NLB のドメイン名が変更されます。DNS エイリアスを利用している場合、エイリアスはセキュリティグループが設定された新しい NLB を参照するようになります。

<a id="copilot-deploy-enhancements"></a>

## `copilot deploy` の機能拡張
`copilot deploy` で複数のワークロードを一度にデプロイできるようになりました。`--name` フラグで複数のワークロードを指定できます。新しい `--all` フラグを `--init-wkld` フラグを組み合わせて利用すると、すべてのローカルワークロードを初期してデプロイできます。また、Service 名を指定する際に "deployment order" タグを利用可能になりました。

たとえば、複数のワークロードを含む新しいリポジトリをクローンした場合、次のコマンドで環境とすべての Service を初期化できます。
```console
copilot deploy --init-env --deploy-env -e dev --all --init-wkld
```

別の例として、他の Service が公開するトピックをサブスクライブしている Worker Service がある場合、`--all` と組み合わせて名前と順序を指定できます。
```console
copilot deploy --all -n fe/1 -n worker/2
```
上記のコマンドでは、まず `fe` をデプロイした後、次に `worker` をデプロイした後に、ワークスペース内の残りの Service や Job がデプロイされます。

<a id="better-task-failure-logs"></a>

## タスク失敗時のログの改善
Copilot v1.31 以前のバージョンでは、ECS タスクが停止した原因を確認する場合、AWS マネジメントコンソールから、「ECS」->「サービス」->「停止済みのタスク」->「停止理由」にページ遷移して確認する必要がありました。

この機能拡張により、`copilot [noun] deploy` は CloudFormation のデプロイの進捗トラッカーの中に ECS タスクの停止理由を表示するようになりました。Copilot は、Load Balanced Web Service、Backend Service、Worker Service のデプロイ中に発生した、直近の 2 つのタスクの失敗を表示します。

```console
  - An ECS service to run and maintain your tasks in the environment cluster
    Deployments                                                                                                              
               Revision  Rollout        Desired  Running  Failed  Pending                                                            
      PRIMARY  11        [in progress]  1        0        1       0                                                                  
      ACTIVE   8         [completed]    1        1        0       0                                                                  
    Latest 2 stopped tasks                                                                                                   
      TaskId    CurrentStatus   DesiredStatus                                                                                        
      6b1d6e32  DEPROVISIONING  STOPPED                                                                                              
      9802d212  STOPPED         STOPPED                                                                                              
                                                                                                                                     
    ✘ Latest 2 tasks stopped reason                                                                                 
      - [6b1d6e32,9802d212]: Essential container in task exited                                                                      
                                                                                                                                     
    Troubleshoot task stopped reason                                                                                         
      1. You can run `copilot svc logs --previous` to see the logs of the last stopped task.                                
      2. You can visit this article: https://repost.aws/knowledge-center/ecs-task-stopped.          
```

<a id="importing-an-acm-certificate-for-your-static-site"></a>

## 静的サイトへ ACM 証明書のインポート
Copilot では、[Static Site の Manifest](../docs/manifest/static-site.ja.md) に新しいフィールド `http.certificate` が追加されました。`us-east-1` の検証済み ACM 証明書の ARN を次のように指定できます。

```yaml
http:
  alias: example.com
  certificate: "arn:aws:acm:us-east-1:1234567890:certificate/e5a6e114-b022-45b1-9339-38fbfd6db3e2"
```

`example.com` は、インポートする証明書のドメイン名または Subject Alternative Name (SAN) でなければならないことに注意してください。HTTPS トラフィックには、Copilot が管理する証明書の代わりに、インポートされた証明書が利用されます。