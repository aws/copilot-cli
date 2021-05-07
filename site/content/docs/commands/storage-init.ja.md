# storage init
```bash
$ copilot storage init
```
## コマンドの概要
`copilot storage init` は、ワークロードの 1 つに接続された新しいストレージリソースを作成します。サービスコンテナ内からフレンドリーな環境変数を介してアクセスできます。リソースタイプには、*S3*、*DynamoDB*、*Aurora* のいずれかを指定できます。

このコマンドを実行すると、CLI は `copilot/service` ディレクトリ内に、`addons` サブディレクトリが存在しなければ作成します。`copilot svc deploy` を実行すると、新規に初期化されたストレージリソースが、デプロイ先の環境に作成されます。デフォルトでは、`storage init` で指定したサービスのみが、そのストレージリソースにアクセスできます。

## フラグ
```bash
Required Flags
  -n, --name string           Name of the storage resource to create.
  -t, --storage-type string   Type of storage to add. Must be one of:
                              "DynamoDB", "S3", "Aurora"
  -w, --workload string       Name of the service or job to associate with storage.

DynamoDB Flags
      --lsi stringArray        Optional. Attribute to use as an alternate sort key. May be specified up to 5 times.
                               Must be of the format '<keyName>:<dataType>'.
      --no-lsi                 Optional. Don't ask about configuring alternate sort keys.
      --no-sort                Optional. Skip configuring sort keys.
      --partition-key string   Partition key for the DDB table.
                               Must be of the format '<keyName>:<dataType>'.
      --sort-key string        Optional. Sort key for the DDB table.
                               Must be of the format '<keyName>:<dataType>'.
Aurora Serverless Flags
      --engine string           The database engine used in the cluster.
                                Must be either "MySQL" or "PostgreSQL".
      --parameter-group string  Optional. The name of the parameter group to associate with the cluster.
      --initial-db string       The initial database to create in the cluster.
```

## 使用例
"frontend" Service に "my-bucket" という名前の S3 バケットを作成します。
```
$ copilot storage init -n my-bucket -t S3 -w frontend
```

"frontend" Service にアタッチされた "my-table" という名前の基本的な DynamoDB テーブルを、ソートキーを指定して作成します。
```
$ copilot storage init -n my-table -t DynamoDB -w frontend --partition-key Email:S --sort-key UserId:N --no-lsi
```

複数の代替ソートキーを持つ DynamoDB テーブルを作成します。
```
$ copilot storage init \
  -n my-table -t DynamoDB -w frontend \
  --partition-key Email:S \
  --sort-key UserId:N \
  --lsi Points:N \
  --lsi Goodness:N
```

データベースエンジンに PostgreSQL を使用して、RDS Aurora Serverless クラスタを作成します。
```
$ copilot storage init \
  -n my-cluster -t Aurora -w frontend --engine PostgreSQL
```

## コマンド内部での動作
Copilotは、S3 バケットや DDB テーブルを指定した CloudFormation テンプレートを `addons` ディレクトリに格納します。`copilot svc deploy` を実行すると、CLI はこのテンプレートを addons ディレクトリ内の他のすべてのテンプレートとマージして、Service に関連付けられたネストされた (入れ子になった) スタックを作成します。このネストされたスタックには、その Service に関連付けられたすべての追加リソースが記述されており、その Service がデプロイできる場所ではどこにでもデプロイ可能です。

これは、実行後に、
```
$ copilot storage init -n bucket -t S3 -w fe
$ copilot svc deploy -n fe -e test
$ copilot svc deploy -n fe -e prod
```
2 つのバケットがデプロイされます。1 つは "test" Environment、もう 1 つは "prod" Environmentで、それぞれの "fe" Service からのみアクセスできます。
