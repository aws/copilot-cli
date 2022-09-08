# AWS CloudFormation を使ったモデリング

[Service の Manifest](../../manifest/overview.ja.md)とデフォルトでは統合されていない任意の AWS サービスを Copilot CLI では Addon という形で追加できます。Addon の例として Service から読み書きをする必要がある DyanmoDB テーブルや S3 バケット、RDS Aurora Serverless クラスターなどが考えられます。

## S3 バケットや DynamoDB テーブル、RDS Aurora Serverless クラスターを追加する方法

Copilot は以下のコマンドを使っていくつかの種類の Addon を作成するお手伝いをします:

* [`storage init`](../../commands/storage-init.ja.md) は、 DynamoDB テーブルや S3 バケット、RDS Aurora Serverless クラスターを作成します。

ワークスペースから `copilot storage init` を実行するといくつかの質問に沿ってこれらのリソースをセットアップできます。

## 他のリソースを追加する方法

他の種類の Addon に関しては、以下の説明に則ってご自身のカスタム CloudFormation テンプレートを追加できます。

ワークスペースに `webhook` という名前の Service があるとしましょう。
```bash
.
└── copilot
    └── webhook
        └── manifest.yml
```
そして `webhook` に独自の DynamoDB テーブルを追加したいとしましょう。その場合は `webhook/` ディレクトリ以下に新しく `addons/` ディレクトリを作成し追加で CloudFormation テンプレートを作成します。
```bash
.
└── copilot
    └── webhook
        ├── addons
        │   └── mytable-ddb.yaml
        └── manifest.yaml
```
通常 `addons/` ディレクトリ以下の各ファイルは独立した Addon を表し [AWS CloudFormation (CFN) テンプレート](https://docs.aws.amazon.com/ja_jp/AWSCloudFormation/latest/UserGuide/template-anatomy.html) と解釈されます。例えばさらに Service に S3 バケット Addon を追加したい場合は、 `storage init` を実行するか、もしくは独自の `mybucket-s3.yaml` ファイルを作成するかします。

Service がデプロイされると Copilot はこれらのファイルを全てマージして単一の AWS CloudFormation テンプレートにして Service スタックにネストされたスタックを作成します。

## Addon テンプレートの構造
Addon テンプレートには任意の有効な CloudFormation テンプレートを用いることができます。しかしデフォルトでは Copilot は `App`, `Env` そして `Name` [パラメーター](https://docs.aws.amazon.com/ja_jp/AWSCloudFormation/latest/UserGuide/parameters-section-structure.html)を渡すため、必要であれば [Conditions セクション](https://docs.aws.amazon.com/ja_jp/AWSCloudFormation/latest/UserGuide/conditions-section-structure.html) または [Mappings セクション](https://docs.aws.amazon.com/ja_jp/AWSCloudFormation/latest/UserGuide/mappings-section-structure.html) でリソースのプロパティをカスタマイズできます。

### Addon リソースとの接続
ESC タスクまたは App Runner インスタンスから Addon [リソース](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/resources-section-structure.html)にアクセスするいくつかの方法を紹介します。

* ECS タスクロールや App Runner インスタンスロールに対して追加のポリシーが必要な場合は、[IAM 管理ポリシー](https://docs.aws.amazon.com/ja_jp/AWSCloudFormation/latest/UserGuide/aws-resource-iam-managedpolicy.html) Addon リソースをテンプレートに追加し、そのリソースをテンプレート内の[Output](https://docs.aws.amazon.com/ja_jp/AWSCloudFormation/latest/UserGuide/outputs-section-structure.html)に記述します。 パーミッションが ECS タスクロール、または App Runner インスタンスロールに注入されます。
* ECS サービスにセキュリティグループを追加したい場合、[セキュリティグループ](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-ec2-security-group.html)をテンプレートに定義した上で、[Outputs](https://docs.aws.amazon.com/ja_jp/AWSCloudFormation/latest/UserGuide/outputs-section-structure.html)に記述します。これによりそのセキュリティグループが ECS サービスにアタッチされます。
* AWS Secrets Manager を使って秘密情報を ECS コンテナに渡したい場合、[シークレット](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-secretsmanager-secret.html)をテンプレートに定義し、[Outputs ](https://docs.aws.amazon.com/ja_jp/AWSCloudFormation/latest/UserGuide/outputs-section-structure.html)に記述します。これによりその秘密情報がコンテナに大文字のスネークケース (SNAKE_CASE) で環境変数として渡されます。
* 任意の環境変数を追加したい場合は、渡したい値を[Outputs](https://docs.aws.amazon.com/ja_jp/AWSCloudFormation/latest/UserGuide/outputs-section-structure.html)に記述します。これにより大文字スネークケース (SNAKE_CASE) の環境変数としてコンテナに渡されます。

### テンプレートの作成
テンプレートを作成する場合は、以下に従う必要があります。

* `Parameters` セクションに `App` 、 `Env` 、 `Name`  を含める
* 少なくとも 1 つの `Resource` を含める


以下は DynamoDB テーブル Addon を作成するテンプレートの例です。

```yaml
# これらのパラメータを使ってテンプレートの中で Conditions セクションや Mappings セクションを作成できます。
Parameters:
  App:
    Type: String
    Description: Your application's name.
  Env:
    Type: String
    Description: The environment name your service, job, or workflow is being deployed to.
  Name:
    Type: String
    Description: The name of the service, job, or workflow being deployed.

Resources:
  # AWS::DynamoDB::Table:MyTable のようにここでリソースを定義します。
  # MyTable:
  #   Type: AWS::DynamoDB::Table
  #   Properties:
  #     ...

  # 1. リソースを作成するだけでなく、 ECS タスクからリソースにアクセスする必要がある場合は、
  # リソースにアクセスできるパーミッションを定義する AWS::IAM::ManagedPolicy を作成する必要があります。
  #
  # 例えば以下は MyTable に対するポリシーのサンプルです。
  MyTableAccessPolicy:
    Type: AWS::IAM::ManagedPolicy
    Properties:
      PolicyDocument:
        Version: '2012-10-17'
        Statement:
          - Sid: DDBActions
            Effect: Allow
            Action:
              - dynamodb:BatchGet*
              - dynamodb:DescribeStream
              - dynamodb:DescribeTable
              - dynamodb:Get*
              - dynamodb:Query
              - dynamodb:Scan
              - dynamodb:BatchWrite*
              - dynamodb:Create*
              - dynamodb:Delete*
              - dynamodb:Update*
              - dynamodb:PutItem
            Resource: !Sub ${ MyTable.Arn}

Outputs:
  # 1. Copilot が ESC タスクロールに対して マネージドポリシーを追加するために、IAM ManagedPolicyを Outputセクションに追加する必要があります。
  MyTableAccessPolicyArn:
    Description: "The ARN of the ManagedPolicy to attach to the task role."
    Value: !Ref MyTableAccessPolicy
    
  # 2. ECS タスクにリソースのプロパティを環境変数として注入したい場合は、
  # Output セクションを定義する必要があります。
  #
  # 例えば MyTableName という出力値はタスクに大文字のスネークケースとして注入されます。
  MyTableName:
    Description: "The name of this DynamoDB."
    Value: !Ref MyTable
```

次回リリースするとき Copilot はこのテンプレートを Service にネストされたスタックとして用います！

!!! info
    リソースを追加するために AWS 管理ポリシーを定義する場合は、以下のような [IAM でのベストプラクティス](https://docs.aws.amazon.com/ja_jp/IAM/latest/UserGuide/best-practices.html) に従うことを推奨します。
    
    * `addons/` ディレクトリで定義するポリシーでは[最小限のアクセス権を付与する](https://docs.aws.amazon.com/ja_jp/IAM/latest/UserGuide/best-practices.html#grant-least-privilege)
    * `addons/` ディレクトリで定義したリソースに対してのみアクセスできるようにポリシーを制限するために [追加セキュリティに対するポリシー条件を使用する](https://docs.aws.amazon.com/ja_jp/IAM/latest/UserGuide/best-practices.html#use-policy-conditions) 


### `Parameters` セクションのカスタマイズ

Copilot では、 `App`, `Env` そして `Name` パラメーターがテンプレートに定義されている必要があります。もし、 Service スタック内のリソースを参照するを追加したい場合は、 `addons.parameters.yml` ファイルを作成します。テンプレート内パラメーターのデータ型やデフォルト値の指定方法については、[CloudFormation テンプレートでのパラメーター定義](https://docs.aws.amazon.com/ja_jp/AWSCloudFormation/latest/UserGuide/parameters-section-structure.html)に準じます。

```term
.
└── addons/
    ├── template.yml
    └── addons.parameters.yml # このファイルを addons/ ディレクトリの下に追加します
```

`addons.parameters.yml` には、Service スタック内の値を参照するパラメーターを例えば以下のように定義できます。
```yaml
Parameters:
  ServiceName: !GetAtt Service.Name
```
最後に、新しいパラメーターを参照するようにテンプレートファイルを更新しましょう。

```yaml
Parameters:
  # AWS Copilot で必要なパラメーター
  App:
    Type: String
  Env:
    Type: String
  Name:
    Type: String
  # addons.parameters.yml で追加したパラメーター
  ServiceName:
    Type: String
```