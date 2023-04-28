# AWS CloudFormation による Environment リソース追加モデリング

CLI で "Addon" と呼ばれる追加の AWS リソースは、[Service または Environment の Manifest](../../manifest/overview.ja.md) がデフォルトで統合しない、任意の追加の AWS サービスです。
例えば Addon は、Service が読み取りまたは書き込みを必要とする DynamoDB テーブル、S3 バケット、または RDS Aurora Serverless クラスターとすることができます。

ワークロード ([Load Balanced Web Service](../../manifest/lb-web-service.ja.md) や [Scheduled Job](../../manifest/scheduled-job.ja.md) など) の追加リソースを定義することができます。
ワークロードの Addon のライフサイクルは、ワークロードによって管理され、ワークロードが削除されると削除されます。

または、Environment に対して追加の共有可能なリソースを定義することができます。
Environment Addon は、Environment が削除されない限り、削除されることはありません。

このページでは、Environment レベルの Addon を作成する方法を説明します。
ワークロードレベル Addon については、[AWS CloudFormation によるワークロードリソース追加モデリング](./workload.ja.md) を参照してください。

## どのように S3 バケット、DDB テーブル、Aurora Serverless クラスターを追加するのか？

Copilot には、特定の種類の Addon を作成するのに役立つ以下のコマンドが用意されています。

* [`storage init`](../../commands/storage-init.ja.md) は、DynamoDB テーブル、S3 バケット、または Aurora Serverless クラスター

ワークスペースから `copilot storage init` を実行すると、これらのリソースをセットアップするのに役立ついくつかの質問をガイドしてくれます。


## 他のリソースを追加するには？

他の種類の Addon については、独自のカスタム CloudFormation テンプレートを追加することができます。

1. カスタムテンプレートは、ワークスペースの `copilot/environments/addons` ディレクトリに格納することができます。
2. `copilot env deploy` を実行すると、カスタムアドオンのテンプレートが Environment スタックと一緒にデプロイされます。


???- note "Environment Addon によるワークスペースのレイアウト例"
    ```term
    .
    └── copilot
        └── environments
            ├── addons  # Environment Addon の格納
            │   └── mys3.yaml
            ├── dev
            └── prod
    ```

## Addon テンプレートとはどのようなものか？
Environment Addon テンプレートは、以下を満たす[有効な CloudFormation テンプレート](https://docs.aws.amazon.com/ja_jp/AWSCloudFormation/latest/UserGuide/template-anatomy.html)であれば、どのようなものでも使用可能です。

* 少なくとも 1 つの [`Resource`](https://docs.aws.amazon.com/ja_jp/AWSCloudFormation/latest/UserGuide/resources-section-structure.html) が含まれる。
* `Parameters` セクションに `App`、`Env` が含まれる。

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
2. `addons.parameters.yml` にて、これらの追加パラメータの値を定義します。これらは、Environment スタックの値を参照することができます。

???- note "例: Addon パラメータのカスタマイズ"
    ```yaml
    # "environments/addons/my-addon.yml" にて
    Parameters:
      # AWS Copilotで必要なパラメータ
      App:
        Type: String
      Env:
        Type: String
      # addons.parameters.yml で定義された追加パラメータ
      ClusterName:
        Type: String
    ```
    ```yaml
    # "environments/addons/addons.parameters.yml" にて
    Parameters:
        ClusterName: !Ref Cluster
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

#### Environment Addon: ワークロードに接続する

Environment Addon からの値は、ワークロード Addon またはワークロード Manifest から参照することができます。
これを行うには、まず `Outputs` セクションを使用して Environment Addon から値をエクスポートする必要があります。

???+ note "例: Environment Addon からの値のエクスポート"
    ```yaml
    Outputs:
        MyTableARN:
            Value: !GetAtt ServiceTable.Arn
            Export:
                Name: !Sub ${App}-${Env}-MyTableARN  # この値は、ワークロード Manifest またはワークロード Addon によって使用されることがあります。
        MyTableName:
            Value: !Ref ServiceTable
            Export:
                Name: !Sub ${App}-${Env}-MyTableName
    ```


`Export` ブロックを追加することが重要です。
そうしなければ、ワークロードスタックやワークロード Addon がその値にアクセスできなくなります。
ワークロードレベルのリソースから値を参照するには、`Export.Name` を使用します。

???- hint "検討事項: 名前空間 `Export.Name` の使用"
    Export.Name` には、好きな名前を指定することができます。
    つまり、`!Sub ${App}-${Env}` というプレフィックスを付ける必要はなく、単に `MyTableName` とすることもできます。

    しかし、AWS のリージョン内では、`Export.Name` は一意でなければなりません。
    つまり、`us-east-1` に `MyTableName` というエクスポート名を重複して持つことはできません。
    
    したがって、名前衝突の可能性を減らすために、`${App}` と `${Env}` で名前空間を指定してエクスポートすることをお勧めします。
    またこれにより、その値がどの Application と Environment の下で管理されているかが明確になります。
    
    名前空間では、例えば Application 名が `"my-app"` で、Environment `test` で Addon をデプロイしたとすると、最終的なエクスポート名は `my-app-test-MyTableName` となります。

##### ワークロード Addon からの参照

ワークロード Addon では、値がエクスポートされている限り、Environment Addon から値を参照することができます。
これを行うには、Environment Addon から値をインポートするために、その値のエクスポート名で [`Fn::ImportValue`](https://docs.aws.amazon.com/ja_jp/AWSCloudFormation/latest/UserGuide/intrinsic-function-reference-importvalue.html) 関数を使用します。

???- note "例: Environment レベルの DynamoDB テーブルにアクセスするための IAM ポリシー"
    ```yaml
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
      MyTableAccessPolicy:
        Type: AWS::IAM::ManagedPolicy
        Properties:
          Description: Grants CRUD access to the Dynamo DB table
          PolicyDocument:
            Version: '2012-10-17'
            Statement:
              - Sid: DDBActions
                Effect: Allow
                Action:
                  - dynamodb:* # NOTE: 実際の Application でパーミッションをスコープダウンしてください。これは、この例があまり長くならないようにするための記述です。
                Resource: 
                  Fn::ImportValue:                # <- Environment Addon からテーブル ARN をインポート
                    !Sub ${App}-${Env}-MyTableARN # <- 使用するエクスポート名
    ```



##### ワークロード Manifest からの参照

また、Environment Addon から [`variables`](../../manifest/lb-web-service.ja.md#variables-from-cfn)、[`secrets`](../../manifest/lb-web-service.ja.md#secrets-from-cfn)、[`security_groups`](../../manifest/lb-web-service.ja.md#network-vpc-security-groups-from-cfn) のワークロード Manifest で、その値がエクスポートされている限り、その値を参照することができます。
これを行うには、ワークロード Manifest でその値のエクスポート名を持つ `from_cfn` フィールドを使用します。


???- note "例: `from_cfn` の使用"
    === "シークレットのインジェクト"
        ```yaml
        name: db-front
        type: Backend Service
        
        secrets:
          MY_CLUSTER_CREDS:
            from_cfn: ${COPILOT_APPLICATION_NAME}-${COPILOT_ENVIRONMENT_NAME}-MyClusterSecret
        ```

    === "セキュリティグループのアタッチ"
        ```yaml
        name: db-front
        type: Backend Service
        
        security_groups:
            - from_cfn: ${COPILOT_APPLICATION_NAME}-${COPILOT_ENVIRONMENT_NAME}-MyClusterAllowedSecurityGroup
        ```


## 例

### Environment Addon のウォークスルー
[v1.25.0 のブログ記事](../../../blogs/release-v125.ja.md/#environment-addons)で詳しい解説をご覧ください。
