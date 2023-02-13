---
title: 'AWS Copilot v1.25: Environment Addon と静的コンテンツ配信'
twitter_title: 'AWS Copilot v1.25'
image: ''
image_alt: ''
image_width: '1051'
image_height: '747'
---

# AWS Copilot v1.25: Environment Addon と静的コンテンツ配信

投稿日: 2023 年 1 月 17 日

AWS Copilot コアチームは Copilot v1.25 リリースを発表します。
私たちのパブリックな[コミュニティチャット](https://gitter.im/aws/copilot-cli)は成長しており、オンラインでは 400 人以上、[GitHub](http://github.com/aws/copilot-cli/) では 2.6k 以上のスターを獲得しています。
AWS Copilot へご支援、ご支持いただいている皆様お一人お一人に感謝をいたします。

Copilot v1.25 では、いくつかの新機能と改良が施されています。

- **Environment Addon**: [詳細はこちらを確認してください。](#environment-addons).
- **CloudFront による静的コンテンツ配信**: [詳細はこちらを確認してください。](#static-content-delivery-with-cloudfront).


???+ note "AWS Copilot とは?"

    AWS Copilot CLI は AWS 上でプロダクションレディなコンテナ化されたアプリケーションのビルド、リリース、そして運用のためのツールです。
    開発のスタートからステージング環境へのプッシュ、本番環境へのリリースまで、Copilot はアプリケーション開発ライフサイクル全体の管理を容易にします。
    Copilot の基礎となるのは、 AWS CloudFormation です。CloudFormation により、インフラストラクチャを 1 回の操作でコードとしてプロビジョニングできます。
    Copilot は、さまざまなタイプのマイクロサービスの作成と運用の為に、事前定義された CloudFormation テンプレートと、ユーザーフレンドリーなワークフローを提供します。
    デプロイメントスクリプトを記述する代わりに、アプリケーションの開発に集中できます。

    より詳細な AWS Copilot の紹介については、[Overview](../docs/concepts/overview.ja.md) を確認してください。

<a id="environment-addons"></a>

## Environment Addon

Environment に関する Addon を配置できるようになりました。

Addon とは、デフォルトでは Copilot に統合されていない追加の AWS リソース - 例えば、DynamoDB、RDS などであり、Environment Addon は、Environment レベルで管理される追加のリソースです。Environment Addon を作成または更新するためには `copilot env deploy` を実行し、Environment に対して `copilot env delete` を実行すると、Copilot はその Addon も削除しようとします。

すでに Workload Addon に慣れている方には朗報で、Environment Addon の管理は、ほとんど同じような感覚で行えます。

#### はじめに
##### Step 1: CloudFormation で追加の AWS リソースをモデル化する
現在、Addon は CloudFormation を使ったリソースの定義のみサポートしています。Environment Addon の場合は以下が必須です。

1. `Parameters` に `App` と `Env` を持たせる。
2. 少なくとも 1 つの `Resource` を含ませる。

???- note "サンプル CloudFormation テンプレート"
    ここでは、実際にお試し頂ける CloudFormation のテンプレートの例をご紹介します。

    ```yaml
    AWSTemplateFormatVersion: 2010-09-09
    Parameters:
      App:
        Type: String
        Description: Your application's name.
      Env:
        Type: String
        Description: The name of the environment being deployed.
    Resources:
      MyTable:
        Type: 'AWS::DynamoDB::Table'
        Properties:
          TableName: MyEnvAddonsGettingStartedTable
          AttributeDefinitions:
            - AttributeName: key
              AttributeType: S
          KeySchema:
            - AttributeName: key
              KeyType: HASH
          ProvisionedThroughput:
            ReadCapacityUnits: 5
            WriteCapacityUnits: 2
    Outputs:
      MyTableARN:
        Value: !GetAtt MyTable.Arn
        Export:
          Name: !Sub ${App}-${Env}-MyTableARN
      MyTableName:
        Value: !Ref MyTable
        Export:
          Name: !Sub ${App}-${Env}-MyTableName
    ```

##### Step 2: CFN テンプレートを `copilot/environments/addons` に格納

`copilot env init` を実行すると、ワークスペースに `copilot/environments` フォルダが作成されているはずです。もし実行していない場合は、後ほど必要になるので直ちに実行しておきます。

実行後、ワークスペースは以下のような構成になります。
```
copilot/
├── environments/
│   ├── addons/  
│   │     ├── appmesh.yml         
│   │     └── ddb.yml      # <- 複数の Addon を持つことができます
│   ├── test/
│   │     └─── manifest.yml
│   └── dev/
│         └── manifest.yml
└── web
    ├── addons/
    │     └── s3.yml       # <- ワークロードの Addon テンプレートです
    └─── manifest.yml
```

##### Step 3: `copilot env deploy` の実行

`copilot env deploy` を実行すると、Copilot は `addons` フォルダをスキャンして Addon テンプレートを探します。見つかった場合は、Environment と一緒にテンプレートも配備されます。

##### (オプション) Step 4: デプロイの確認

デプロイの確認は、利用しているリージョンの [AWS CloudFormation コンソール](https://ap-northeast-1.console.aws.amazon.com/cloudformation/home?region=ap-northeast-1#/stacks)にアクセスすることで行えます。`[app]-[env]-AddonsStack-[random string]` というスタックを見つけることができるはずです。これは、`[app]-[env]` という名前の Environment スタックの下に作成された、ネストされたスタックです。

### ワークロード Addon との機能比較
Environment Addon は、ワークロード Addon で利用可能なすべての既存機能を含めて提供されます。これは、次のことを意味します。

1. 必須の `App` や `Env` に加えて、カスタマイズされた `Parameter` を参照することができます。
2. テンプレートにおいて、ローカルパスを参照することができます。Copilot はこれらのローカルファイルをアップロードし、関連するリソースプロパティをアップロードされた S3 ロケーションに置換します。

詳しくは[こちら](../docs/developing/addons/workload.ja.md)をご覧ください。

### その他の考慮事項
すべての Environment (上記の例では、"test" と "dev" の両方) は、同じ Addon テンプレートを共有します。今日のワークロードレベルの Addon と同様に、Environment 固有の設定は、Addon テンプレートの `Conditions` および `Mappings` セクションで指定する必要があります。これは、[CFN におけるテンプレート再利用のベストプラクティス](https://docs.aws.amazon.com/ja_jp/AWSCloudFormation/latest/UserGuide/best-practices.html#reuse) に則ったものです。

```yaml
AWSTemplateFormatVersion: 2010-09-09
Parameters:
  App:
    Type: String
    Description: Your application name.
  Env:
    Type: String
    Description: The name of the environment being deployed.

Conditions:
  IsTestEnv: !Equals [ !Ref Env, "test" ]  # <- "test" 固有の設定を行うには、`Conditions` セクションを使用します

Mappings:
  ScalingConfigurationMapByEnv:
    test:
      "DBMinCapacity": 0.5
    prod:
      "DBMinCapacity": 1
```

### ワークロードとの統合
ワークロードレベルのリソースで、Environment Addon からの値を参照することができます。

#### ワークロード Addon における Environment Addon の値の参照

##### Step 1: Environment Addon から値をエクスポート
Environment Addon テンプレートで、`Outputs` セクションを追加し、ワークロードリソースに参照させたい `Output` を定義する必要があります。CloudFormation の `Outputs` 構文については、[こちらのドキュメント](https://docs.aws.amazon.com/ja_jp/AWSCloudFormation/latest/UserGuide/outputs-section-structure.html)を参照してください。

テンプレート例では、以下のように `Outputs` セクションを追加しています。
```yaml
Outputs:
  MyTableARN:
    Value: !GetAtt MyTable.Arn
    Export:
      Name: !Sub ${App}-${Env}-MyTableARN
  MyTableName:
    Value: !Ref MyTable
    Export:
      Name: !Sub ${App}-${Env}-MyTableName
```

`Export.Name` には好きな名前を指定することができますが、AWS リージョン内でユニークな名前である必要があります。そのため、名前の衝突の可能性を減らすために、`${App}` と`${Env}` で名前空間を設定することをお勧めします。名前空間では、例えばアプリケーション名が `"my-app"` で、Environment `test` で Addon をデプロイしたとすると、最終的なエクスポート名は `my-app-test-MyTableName` となります。

コードを変更した後、`copilot env deploy` を実行して変更を反映させます。


##### Step 2: ワークロード Addon から値をインポート

ワークロード Addon にて、[`Fn::ImportValue`](https://docs.aws.amazon.com/ja_jp/AWSCloudFormation/latest/UserGuide/intrinsic-function-reference-importvalue.html) 関数を使用して Environment Addon からエクスポートした値をインポートします。

上記の例の続きで、`db-front` Service が `MyTable` にアクセスするようにしたいとします。`db-front` Service にアクセスするための IAM ポリシーを持つワークロード Addon を作成します。

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
    Description: The name of the service, job, or workflow being deployed.
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
              - dynamodb:* # NOTE: 実際の Application でパーミッションをスコープダウンしてください。これはあくまでも本ブログの記事が長くなりすぎないようにするための記述です。
            Resource:
              Fn::ImportValue:                # <- Environment Addon からテーブル ARN をインポート
                !Sub ${App}-${Env}-MyTableARN # <- 使用したエクスポート名
```

別の例として、`Export.Name` に名前空間を付けず、代わりに以下のような名前を付けてエクスポートしたとします。
```yaml
Outputs:
  MyTableARN:
    Value: !GetAtt MyTable.Arn
    Export:
      Name: !Sub MyTableARN
```

インポートするには、以下のような値にします。
```yaml
Fn::ImportValue:       
  !Sub MyTableARN
```

ワークロードの Addon と Environment Addon は、このように関連付けます。

#### ワークロード Manifest における Environment Addon の値の参照

Environment Addon から何らかの値を参照する必要がある - 例えば、Environment Addon で作成した Secret を Service に追加する - 場合、[ワークロード Manifest の `from_cfn`](#import-values-from-cloudformation-stacks-in-workload-manifests) 機能を使用して行うことができます。

##### Step 1: Environment Addon から値をエクスポート
ワークロード Addon で作業するときと同じように、Environment Addon から値をエクスポートする必要があります。

```yaml
Outputs:
  MyTableName:
    Value: !Ref MyTable
    Export:
      Name: !Sub ${App}-${Env}-MyTableName
```

##### Step 2: ワークロード Manifest で `from_cfn`  を使用して値を参照
`db-front` Service でテーブル名を環境変数として注入したい場合、`db-front` Service は次のような Manifest を持つ必要があります。

```yaml
name: db-front
type: Backend Service

// その他の設定値...

variables:
  MY_TABLE_NAME:
    from_cfn: ${COPILOT_APPLICATION_NAME}-${COPILOT_ENVIRONMENT_NAME}-MyTableName
```

同様に、名前空間を除いたテーブル名をエクスポートした場合は、以下のようにします。
```yaml
Outputs:
  MyTableName:
    Value: !Ref MyTable
    Export:
      Name: MyTableName
```

その場合、Manifest では以下のような値にします。
```yaml
variables:
  MY_TABLE_NAME:
    from_cfn: MyTableName
```


<a id="import-values-from-cloudformation-stacks-in-workload-manifests"></a>

### ワークロード Manifest における CloudFormation スタックからの値のインポート

`from_cfn` を使用して、Environment Addon の CloudFormation スタックまたはワークロード Manifest の他のスタックから値をインポートできるようになりました。他の CloudFormation スタックから値を参照するには、ユーザーは最初にソーススタックから出力値をエクスポートする必要があります。

以下は、他のスタックから値をエクスポートしたり、相互スタック参照を作成する際に、CloudFormation テンプレートの `Outputs` セクションがどのように見えるかの一例です。

```yaml
Outputs:
  WebBucketURL:
    Description: URL for the website bucket
    Value: !GetAtt WebBucket.WebsiteURL
    Export:
      Name: stack-WebsiteUrl # <- リージョン内で一意なエクスポート名
```

詳しくは[こちらのページ](https://docs.aws.amazon.com/ja_jp/AWSCloudFormation/latest/UserGuide/outputs-section-structure.html)をご覧ください。

今のところ、`from_cfn` は以下のワークロード Manifest フィールドにのみ追加されます。

```yaml
variables:
  LOG_LEVEL: info
  WebsiteUrl:
    from_cfn: stack-WebsiteUrl
```

```yaml
secrets:
  GIT_USERNAME:
    from_cfn: stack-SSMGHUserName
```

```yaml
logging:
  secretOptions:
    GIT_USERNAME:
      from_cfn: stack-SSMGHUserName
```

```yaml
sidecars:
  secrets:
    GIT_USERNAME:
      from_cfn: stack-SSMGHUserName
```

```yaml
network:
  vpc:
    security_groups:
      - sg-1234
      - from_cfn: UserDBAccessSecurityGroup
```

<a id="static-content-delivery-with-cloudfront"></a>

## CloudFront による静的コンテンツ配信
独自の S3 バケットを利用して CloudFront と連携し、より高速な静的コンテンツ配信を実現できるようになりました。バケット管理のネイティブサポート (バケット作成、アセットのアップロードなど) は、今後のリリースでより充実させる予定です。

### (オプション) S3 バケットの作成
既存の S3 バケットがない場合、S3 コンソールや AWS CLI/SDK を使用して S3 バケットを作成します。なお、セキュリティの観点から、デフォルトでパブリックアクセスをブロックするプライベート S3 バケットを作成することを強く推奨します。

### env Manifest での CloudFront の設定
以下のように Environment Manifest を設定することで、S3 バケットをオリジンとした CloudFront を利用することができます。

```yaml
cdn:
  static_assets:
    location: cf-s3-ecs-demo-bucket.s3.us-west-2.amazonaws.com
    alias: example.com
    path: static/*
```

具体的には、`location` は [S3 バケットの DNS ドメイン名](https://docs.aws.amazon.com/ja_jp/AmazonCloudFront/latest/DeveloperGuide/distribution-web-values-specify.html#DownloadDistValuesDomainName) で、静的アセットは `example.com/static/*` にアクセスすることになります。

### (オプション) バケットポリシーの更新
CloudFront に使用しているバケットが**プライベート**の場合、CloudFront に読み取りアクセスを許可するようにバケットポリシーを更新する必要があります。上記の例を用いる場合、`cf-s3-ecs-demo-bucket` のバケットポリシーを更新して、以下のようにする必要があります。

```json
{
    "Version": "2012-10-17",
    "Statement": {
        "Sid": "AllowCloudFrontServicePrincipalReadOnly",
        "Effect": "Allow",
        "Principal": {
            "Service": "cloudfront.amazonaws.com"
        },
        "Action": "s3:GetObject",
        "Resource": "arn:aws:s3:::cf-s3-ecs-demo-bucket/*",
        "Condition": {
            "StringEquals": {
                "AWS:SourceArn": "arn:aws:cloudfront::111122223333:distribution/EDFDVBD6EXAMPLE"
            }
        }
    }
}
```

CloudFront のディストリビューション ID は、`copilot env show --resources` を実行することで確認することができます。

## 次は？

以下のリンクより、新しい Copilot CLI バージョンをダウンロードし、[GitHub](https://github.com/aws/copilot-cli/) や [コミュニティチャット](https://gitter.im/aws/copilot-cli)にフィードバックを残してください。

- [最新 CLI バージョン](../docs/getting-started/install.ja.md)のダウンロード
- [スタートガイド](../docs/getting-started/first-app-tutorial.ja.md)を試す
- [GitHub](https://github.com/aws/copilot-cli/releases/tag/v1.25.0) でリリースノートの全文を読む

今回のリリースの翻訳はソリューションアーキテクトの杉本が担当しました。

