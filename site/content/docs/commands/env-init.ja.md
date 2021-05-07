# env init
```bash
$ copilot env init [flags]
```

## コマンドの概要
`copilot env init` は、Service 実行用に新しい [Environment](../concepts/environments.md) を作成します。

質問に答えると、CLI は VPC, Application Load Balancer, ECS Cluster などの Service で共有される共通のインフラストラクチャーを作成します。さらに、デフォルトのリソース設定や既存リソースのインポートなど、[Copilot の Environment をカスタマイズ](../concepts/environments.md#customize-your-environment) できます。

[名前付きプロファイル](../credentials.md#environment-credentials) を使用して、Environment を作成する AWS アカウントとリージョンを指定します。

## フラグ
AWS Copilot CLI の全てのコマンド同様、必須フラグを省略した場合にはそれらの情報の入力をインタラクティブに求められます。必須フラグを明示的に渡してコマンドを実行することでこれをスキップできます。
```
Common Flags
      --aws-access-key-id string       Optional. An AWS access key.
      --aws-secret-access-key string   Optional. An AWS secret access key.
      --aws-session-token string       Optional. An AWS session token for temporary credentials.
      --default-config                 Optional. Skip prompting and use default environment configuration.
  -n, --name string                    Name of the environment.
      --prod                           If the environment contains production services.
      --profile string                 Name of the profile.
      --region string                  Optional. An AWS region where the environment will be created.

Import Existing Resources Flags
      --import-private-subnets strings   Optional. Use existing private subnet IDs.
      --import-public-subnets strings    Optional. Use existing public subnet IDs.
      --import-vpc-id string             Optional. Use an existing VPC ID.

Configure Default Resources Flags
      --override-private-cidrs strings   Optional. CIDR to use for private subnets (default 10.0.2.0/24,10.0.3.0/24).
      --override-public-cidrs strings    Optional. CIDR to use for public subnets (default 10.0.0.0/24,10.0.1.0/24).
      --override-vpc-cidr ipNet          Optional. Global CIDR to use for VPC (default 10.0.0.0/16).

Global Flags
  -a, --app string   Name of the application.
```

## 実行例
AWS プロファイルの "default" に、デフォルト設定を使用して test Environment を作成します。

```bash
$ copilot env init --name test --profile default --default-config
```

AWS プロファイルの "prod-admin" を利用して既存の VPC に prod-iad Environment を作成します。
```bash
$ copilot env init --name prod-iad --profile prod-admin --prod \
--import-vpc-id vpc-099c32d2b98cdcf47 \
--import-public-subnets subnet-013e8b691862966cf,subnet-014661ebb7ab8681a \
--import-private-subnets subnet-055fafef48fb3c547,subnet-00c9e76f288363e7f
```

## 出力例
![Running copilot env init](https://raw.githubusercontent.com/kohidave/copilot-demos/master/env-init.svg?sanitize=true)
