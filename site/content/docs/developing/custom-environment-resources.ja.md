# Environment のリソースをカスタマイズする

Copilot を使って新しく [Environment](../concepts/environments.ja.md) を作成するとき既存の VPC リソースをインポートすることもできます。(これは以下のように[`env init` を実行するときのフラグ](../commands/env-init.ja.md#_2)またはインタラクティブに質問に答えることで実現できます。)

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

デフォルト設定を選択すると、 Copilot は [AWS のベストプラクティス](https://aws.amazon.com/blogs/containers/amazon-ecs-availability-best-practices/)に従って 2 つのアベイラビリティゾーンにまたがった VPC と 2 つのパブリックサブネットおよびプライベートサブネットを作成します。多くのケースではこの設定はいいものですが、 Copilot は既存のリソースをインポートするときに柔軟に対応できます。例えば、インターネットに面していない 2 つのプライベートサブネットだけあってパブリックサブネットがない VPC をインポートすることができます(分離されたネットワークに関してもっと知りたい方は[こちら](https://github.com/aws/copilot-cli/discussions/2378)をご覧ください)。

## 制約

- 既存の VPC をインポートする場合は、[VPC のセキュリティのベストプラクティス](https://docs.aws.amazon.com/ja_jp/vpc/latest/userguide/vpc-security-best-practices.html)と、[Amazon VPC FAQ の「セキュリティとフィルタリング」のセクション](https://aws.amazon.com/jp/vpc/faqs/#Security_and_Filtering)に準拠することをお勧めします。
- プライベートホストゾーンをご利用の場合は、[こちら](https://docs.aws.amazon.com/ja_jp/Route53/latest/DeveloperGuide/hosted-zone-private-considerations.html#hosted-zone-private-considerations-vpc-settings)にあるように`enableDnsHostname` と `enableDnsSupport` を true に設定してください。
- [プライベートサブネット](../manifest/lb-web-service.ja.md#network-vpc-placement)に、インターネットに面したワークロードをデプロイする場合は VPC に [NAT ゲートウェイ](https://docs.aws.amazon.com/ja_jp/vpc/latest/userguide/vpc-nat-gateway.html)が必要です。
