# AWS CloudFormation によるワークロードリソース追加モデリング

CLI で "Addon" と呼ばれる追加の AWS リソースは、[Service または Environment の Manifest](../../manifest/overview.ja.md) がデフォルトで統合しない、任意の追加の AWS サービスです。
例えば Addon は、Service が読み取りまたは書き込みを必要とする DynamoDB テーブル、S3 バケット、または RDS Aurora Serverless クラスターとすることができます。

ワークロードの追加リソース ([Load Balanced Web Service](../../manifest/lb-web-service.ja.md) や [Scheduled Job](../../manifest/scheduled-job.ja.md) など) を定義することができます。
ワークロードの Addon のライフサイクルは、ワークロードによって管理され、ワークロードが削除されると削除されます。

または、Environment に対して追加の共有可能なリソースを定義することができます。
Environment Addon は、Environment が削除されない限り、削除されることはありません。

このページでは、ワークロードレベルの Addon を作成する方法を説明します。
Environment レベル Addon については、[AWS CloudFormation による Environment リソース追加モデリング](./environment.ja.md) を参照してください。

## どのように S3 バケット、DDB テーブル、Aurora Serverless クラスターを追加するのか？

Copilot では、特定の種類の Addon を作成するために、以下のコマンドが用意されています。

* [`storage init`](../../commands/storage-init.ja.md) は DynamoDB テーブル、S3 バケット、Aurora Serverless クラスターのいずれかを作成します。

ワークスペースにて `copilot storage init` を実行すると、これらのリソースをセットアップために、いくつかの質問形式でガイドして行きます。


## 他のリソースを追加するには？

他の種類の Addon については、独自のカスタム CloudFormation テンプレートを追加することができます。

1. カスタムテンプレートは、ワークスペースの `copilot/<workload>/addons` ディレクトリに格納することができます。
2. `copilot [svc/job] deploy` を実行すると、カスタムアドオンのテンプレートがワークロードスタックと一緒にデプロイされます。

???- note "ワークロード Addon によるワークスペースのレイアウト例"
    ```term
    .
    └── copilot
        └── webhook
            ├── addons # Service "webhook" に関連する Addon の格納
            │   └── mytable-ddb.yaml
            └── manifest.yaml
    ```

## Addon テンプレートとはどのようなものか？
ワークロード Addon テンプレートは、以下を満たす[有効な CloudFormation テンプレート](https://docs.aws.amazon.com/ja_jp/AWSCloudFormation/latest/UserGuide/template-anatomy.html)であれば、どのようなものでも使用可能です。

* 少なくとも 1 つの [`Resource`](https://docs.aws.amazon.com/ja_jp/AWSCloudFormation/latest/UserGuide/resources-section-structure.html) が含まれる。
* [`Parameters`](https://docs.aws.amazon.com/ja_jp/AWSCloudFormation/latest/UserGuide/parameters-section-structure.html) セクションに `App`、`Env`、`Name` が含まれる。

リソースプロパティは、[Conditions](https://docs.aws.amazon.com/ja_jp/AWSCloudFormation/latest/UserGuide/conditions-section-structure.html) や [Mappings](https://docs.aws.amazon.com/ja_jp/AWSCloudFormation/latest/UserGuide/mappings-section-structure.html) を使ってカスタマイズすることができます。

!!! info ""
    [Amazon IAM のベストプラクティス](https://docs.aws.amazon.com/ja_jp/IAM/latest/UserGuide/best-practices.html)に従って、追加リソースの AWS Managed Policies を定義することをお勧めします。

    * `addons/` ディレクトリに定義されているポリシーに[最小特権アクセス許可を適用します](https://docs.aws.amazon.com/ja_jp/IAM/latest/UserGuide/best-practices.html#grant-least-privilege)。
    * [セキュリティ強化のためのポリシー条件を利用して](https://docs.aws.amazon.com/ja_jp/IAM/latest/UserGuide/best-practices.html#use-policy-conditions)、`addons/` ディレクトリに定義されたリソースのみにアクセスするようにポリシーを制限します。


### `Parameters` セクションの書き方

Copilot では、テンプレートに定義する必要があるパラメータがいくつかあります。

!!! info ""
    ```yaml
    Parameters:
        App:
            Type: String
        Env:
            Type: String
        Name:
            Type: String
    ```


#### `Parameters` セクションのカスタマイズ

Copilot が必要とするパラメータ以外にパラメータを定義したい場合は、`addons.parameters.yml` ファイルを使用して定義することができます。

```term
.
└── addons/
    ├── template.yml
    └── addons.parameters.yml # このファイルは addons/ ディレクトリの下に追加します。
```

1. テンプレートファイルの `Parameters` セクションに、追加のパラメータを追加します。
2. `addons.parameters.yml` にて、これらの追加パラメータの値を定義します。これらは、ワークロードスタックの値を参照することができます。

???- note "例: Addon パラメータのカスタマイズ"
    ```yaml
    # "webhook/addons/my-addon.yml" にて
    Parameters:
      # AWS Copilotで必要なパラメータ
      App:
        Type: String
      Env:
        Type: String
      Name:
        Type: String
      # addons.parameters.yml で定義された追加パラメータ
      ServiceName:
        Type: String
    ```
    ```yaml
    # "webhook/addons/addons.parameters.yml" にて
    Parameters:
        ServiceName: !GetAtt Service.Name
    ```

### `Conditions` と `Mappings` セクションの書き方

Addon リソースを特定の条件に応じて異なるように設定したい場合がよくあります。
例えば、DB リソースのキャパシティを、デプロイ先が本番環境かテスト環境かによって、条件付きで設定することができます。
これを行うには、`Conditions` セクションと `Mappings` セクションを使用します。

???- note "例: Addon を条件付きで設定"
    === "`Mappings` の利用"
        ```yaml
        Mappings:
            MyAuroraServerlessEnvScalingConfigurationMap:
                dev:
                    "DBMinCapacity": 0.5
                    "DBMaxCapacity": 8
                test:
                    "DBMinCapacity": 1
                    "DBMaxCapacity": 32
                prod:
                    "DBMinCapacity": 1
                    "DBMaxCapacity": 64
        Resources:
            MyCluster:
                Type: AWS::RDS::DBCluster
                Properties:
                    ScalingConfiguration:
                        MinCapacity: !FindInMap
                            - MyAuroraServerlessEnvScalingConfigurationMap
                            - !Ref Env
                            - DBMinCapacity
                        MaxCapacity: !FindInMap
                            - MyAuroraServerlessEnvScalingConfigurationMap
                            - !Ref Env
                            - DBMaxCapacity
        ```
    
    === "`Conditions` の利用"
        ```yaml
        Conditions:
          IsProd: !Equals [!Ref Env, "prod"]
        
        Resources:
          MyCluster:
            Type: AWS::RDS::DBCluster
            Properties:
              ScalingConfiguration:
                  MinCapacity: !If [IsProd, 1, 0.5]
                  MaxCapacity: !If [IsProd, 8, 64]
        ```


### [`Outputs`](https://docs.aws.amazon.com/ja_jp/AWSCloudFormation/latest/UserGuide/outputs-section-structure.html) セクションの書き方

`Outputs` セクションを使用して、他のリソース、例えば Service、CloudFormation スタックなどで使用できる任意の値を定義することができます。

#### ワークロード Addon: ワークロードに接続する

ECS タスクまたは App Runner インスタンスから Addon [`Resources`](https://docs.aws.amazon.com/ja_jp/AWSCloudFormation/latest/UserGuide/resources-section-structure.html) にアクセスする方法は、次のとおりです。

* ECS タスクロールや App Runner インスタンスロールに追加のポリシーを追加する必要がある場合、追加のパーミッションを保持する [IAM ManagedPolicy](https://docs.aws.amazon.com/ja_jp/AWSCloudFormation/latest/UserGuide/aws-resource-iam-managedpolicy.html) Addon リソースをテンプレートで定義し、それを[出力](https://docs.aws.amazon.com/ja_jp/AWSCloudFormation/latest/UserGuide/outputs-section-structure.html)することができます。このパーミッションは、タスクまたはインスタンスロールにインジェクトされます。
* ECS サービスにセキュリティグループを追加する必要がある場合、テンプレートで[Security Group](https://docs.aws.amazon.com/ja_jp/AWSCloudFormation/latest/UserGuide/aws-properties-ec2-security-group.html)を定義し、それを[出力](https://docs.aws.amazon.com/ja_jp/AWSCloudFormation/latest/UserGuide/outputs-section-structure.html)として追加することができます。セキュリティグループは、自動的に ECS サービスにアタッチされます。
* ECS タスクにシークレットを注入したい場合、テンプレートで [Secret](https://docs.aws.amazon.com/ja_jp/AWSCloudFormation/latest/UserGuide/aws-resource-secretsmanager-secret.html) を定義し、[出力](https://docs.aws.amazon.com/ja_jp/AWSCloudFormation/latest/UserGuide/outputs-section-structure.html)として追加することができます。Secret はコンテナにインジェクトされ、大文字の SNAKE_CASE で環境変数としてアクセスすることができるようになります。
* もし、任意のリソースの値を環境変数として注入したい場合は、ECS タスクに[出力](https://docs.aws.amazon.com/ja_jp/AWSCloudFormation/latest/UserGuide/outputs-section-structure.html)を作成することができます。これはコンテナにインジェクトされ、大文字の SNAKE_CASE で環境変数としてアクセスすることができるようになります。

## 例

### DynamoDB テーブルのワークロード Addon テンプレート

ワークロードレベルの DynamoDB テーブル Addon のテンプレート例です。
```yaml
# これらのパラメータのいずれかを使用して、テンプレートに条件やマッピングを作成することができます。
Parameters:
  App:
    Type: String
    Description: Your application's name.
  Env:
    Type: String
    Description: The environment name your service, job, or workflow is being deployed to.
  Name:
    Type: String
    Description: Your workload's name.

Resources:
  # AWS::DynamoDB::Table などのリソースをここで作成します。
  # MyTable:
  #   Type: AWS::DynamoDB::Table
  #   Properties:
  #     ...

  # 1. リソースに加えて、ECS タスクからリソースにアクセスする必要がある場合、リソースのパーミッション
  # を保持する AWS::IAM::ManagedPolicy を作成する必要があります。
  #
  # 以下は MyTable のポリシーの例です。
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
  # 1. IAM ManagedPolicy を出力して、Copilot が ECS タスクロールにマネージドポリシーとして追加できるようにする必要があります。
  MyTableAccessPolicyArn:
    Description: "The ARN of the ManagedPolicy to attach to the task role."
    Value: !Ref MyTableAccessPolicy

  # 2. もし、リソースのプロパティを環境変数として ECS タスクにインジェクトしたい場合は、その出力を定義する必要があります。
  #
  # 例として、出力された MyTableName は MY_TABLE_NAME という大文字のスネークケースでタスクにインジェクトされます。
  MyTableName:
    Description: "The name of this DynamoDB."
    Value: !Ref MyTable
```

