# Environment のリソースをカスタマイズする

Copilot を使って新しく [Environment](../concepts/environments.ja.md) を作成するとき既存の VPC リソースをインポートできます。(これは以下のように[`env init` を実行するときのフラグ](../commands/env-init.ja.md#_2)またはインタラクティブに質問に答えることで実現できます。)

```bash
% copilot env init
What is your environment's name? env-name
Which credentials would you like to use to create name? [profile default]

  Would you like to use the default configuration for a new environment?
    - A new VPC with 2 AZs, 2 public subnets and 2 private subnets
    - A new ECS Cluster
    - New IAM Roles to manage services and jobs in your environment
  [Use arrows to move, type to filter]
    Yes, use default.
    Yes, but I'd like configure the default resources (CIDR ranges).
  > No, I'd like to import existing resources (VPC, subnets).
```

インポート機能を使用し、インターネットに接続しないワークロード用に 2 つのパブリックサブネットと 2 つのプライベートサブネットを持つ VPC、2 つのパブリックサブネットのみでプライベートサブネットを持たない VPC (default VPC など)、または 2 つのプライベートサブネットのみでパブリックサブネットを持たない VPC を取り込む事ができます。(分離されたネットワークについてより詳細に知りたい方は、[こちら](https://github.com/aws/copilot-cli/discussions/2378)をご覧ください。)

## Copilot のデフォルトリソースの変更
デフォルト設定を選択すると、 Copilot は [AWS のベストプラクティス](https://aws.amazon.com/blogs/containers/amazon-ecs-availability-best-practices/)に従って 2 つのアベイラビリティゾーンにまたがった VPC と 2 つのパブリックサブネットおよびプライベートサブネットを作成します。
追加のアベイラビリティゾーンや CIDR 範囲を変更したい場合、以下の様な変更を選択できます。
```bash 
$ copilot env init --container-insights
What is your environment's name? env-name
Which credentials would you like to use to create name? [profile default]

  Would you like to use the default configuration for a new environment?
    - A new VPC with 2 AZs, 2 public subnets and 2 private subnets
    - A new ECS Cluster
    - New IAM Roles to manage services and jobs in your environment
  [Use arrows to move, type to filter]
    Yes, use default.
  > Yes, but I'd like configure the default resources (CIDR ranges).
    No, I'd like to import existing resources (VPC, subnets).
    
  What VPC CIDR would you like to use? [? for help] (10.0.0.0/16)
  
  Which availability zones would you like to use?  [Use arrows to move, space to select, type to filter, ? for more help]
  [x]  us-west-2a
  [x]  us-west-2b
  > [x]  us-west-2c
  [ ]  us-west-2d
  
  What CIDR would you like to use for your public subnets? [? for help] (10.0.0.0/24,10.0.1.0/24) 10.0.0.0/24,10.0.1.0/24,10.0.2.0/24
  What CIDR would you like to use for your private subnets? [? for help] (10.0.2.0/24,10.0.3.0/24) 10.0.3.0/24,10.0.4.0/24,10.0.5.0/24
```

## 制約

- 既存の VPC をインポートする場合は、[VPC のセキュリティのベストプラクティス](https://docs.aws.amazon.com/ja_jp/vpc/latest/userguide/vpc-security-best-practices.html)と、[Amazon VPC FAQ の「セキュリティとフィルタリング」のセクション](https://aws.amazon.com/jp/vpc/faqs/#Security_and_Filtering)に準拠することをお勧めします。
- プライベートホストゾーンをご利用の場合は、[こちら](https://docs.aws.amazon.com/ja_jp/Route53/latest/DeveloperGuide/hosted-zone-private-considerations.html#hosted-zone-private-considerations-vpc-settings)にあるように `enableDnsHostname` と `enableDnsSupport` を true に設定してください。
- [プライベートサブネット](../manifest/lb-web-service.ja.md#network-vpc-placement)に、インターネットに面したワークロードをデプロイする場合は VPC に [NAT ゲートウェイ](https://docs.aws.amazon.com/ja_jp/vpc/latest/userguide/vpc-nat-gateway.html)が必要です。
