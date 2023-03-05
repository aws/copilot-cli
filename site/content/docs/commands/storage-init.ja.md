# storage init
```console
$ copilot storage init
```
## コマンドの概要

`copilot storage init` は、アドオンとして新しいストレージリソースを作成します。

デフォルトでは、Copilotは「Database per Service」パターンに従っています。
`copilot storage init`で指定した Service または Job だけが、そのストレージリソースにアクセスできます。
ストレージは、サービスのコンテナ内から、ストレージリソースの名前またはリソースにアクセスするためのクレデンシャル情報を保持する環境変数を介してアクセスできます。

!!!note ""
    しかし、ユーザーにはそれぞれ固有の事情があります。もし、複数のサービスでデータストレージを共有する必要がある場合。
    の場合、Copilotが生成したCloudFormationテンプレートを変更することで、目的を達成することができます。

ストレージリソースは、[ワークロードアドオン](../developing/addons/workload.ja.md)として作成することができます。
これは、サービスやジョブの1つにアタッチされ、ワークロードと同時にデプロイされ削除されます。
例えば、`copilot svc deploy --name api`を実行すると、リソースは「api」とともにターゲット環境にデプロイされます。

また、ストレージリソースは [環境アドオン](../developing/addons/environment.ja.md) として作成することができます。
これは環境にアタッチされ、同時にデプロイされ削除されます。
例えば、`copilot env deploy --name test`を実行すると、test という環境と一緒にリソースがデプロイされます。

リソースの種類は *S3*、*DynamoDB*、*Aurora* のいずれかを指定できます。


このコマンドを実行すると、CLI は `copilot/service` ディレクトリ内に、`addons` サブディレクトリが存在しなければ作成します。`copilot svc deploy` を実行すると、新規に初期化されたストレージリソースが、デプロイ先の環境に作成されます。デフォルトでは、`storage init` で指定した Service のみが、そのストレージリソースにアクセスできます。

## フラグ
```
Required Flags
  -l, --lifecycle string      Whether the storage should be created and deleted
                              at the same time as an workload or an environment.
                              Must be one of: "workload" or "environment".
  -n, --name string           Name of the storage resource to create.
  -t, --storage-type string   Type of storage to add. Must be one of:
                              "DynamoDB", "S3", "Aurora".
  -w, --workload string       Name of the service/job that accesses the storage resource.

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
      --engine string               The database engine used in the cluster.
                                    Must be either "MySQL" or "PostgreSQL".
      --initial-db string           The initial database to create in the cluster.
      --parameter-group string      Optional. The name of the parameter group to associate with the cluster.
      --serverless-version string   Optional. Aurora Serverless version. Must be either "v1" or "v2". (default "v2")

Optional Flags
      --add-ingress-from string   The workload that needs access to an
                                  environment storage resource. Must be specified 
                                  with "--name" and "--storage-type".
                                  Can be specified with "--engine".
```

## 使用例
"frontend" Service に "my-bucket" という名前の S3 バケットを作成します。
```console
$ copilot storage init -n my-bucket -t S3 -w frontend -l workload
```

`copilot storage init`は、アドオンとして新しいストレージリソースを作成します。"api "サービスがフロントする "my-bucket"という名前の S3 バケット環境を作成します。

```console
$ copilot storage init \
  -t S3 -n my-bucket \
  -w api -l environment
```


"frontend" Service にアタッチされた "my-table" という名前の基本的な DynamoDB テーブルを、ソートキーを指定して作成します。
```console
$ copilot storage init -t DynamoDB -n my-table \
  -w frontend -l workload \
  --partition-key Email:S \
  --sort-key UserId:N \
  --no-lsi
```

複数の代替ソートキーを持つ DynamoDB テーブルを作成します。
```console
$ copilot storage init -t DynamoDB -n my-table \
+  -w frontend  \
  --partition-key Email:S \
  --sort-key UserId:N \
  --lsi Points:N \
  --lsi Goodness:N
```

データベースエンジンに PostgreSQL を使用して、RDS Aurora Serverless v2 クラスタを作成します。
```console
$ copilot storage init \
  -n my-cluster -t Aurora -w frontend --engine PostgreSQL
```

データベースエンジンに MySQL を使用し、初期データベース名を testdb として、RDS Aurora Serverless v1 クラスタを作成します。
```console
$ copilot storage init \
  -n my-cluster -t Aurora --serverless-version v1 -w frontend --engine MySQL --initial-db testdb
```

## コマンド内部での動作

Copilotは、S3バケット、DDBテーブル、またはAurora Serverlessクラスタを指定するCloudformationテンプレートを`addons`ディレクトリに書き込む。
`copilot [svc/job/env] deploy`を実行すると、CLIはこのテンプレートを addons ディレクトリの他のすべてのテンプレートとマージして、Service または環境に関連するネストされたスタックを作成します。
このネストされたスタックには、サービスまたは環境に関連付けられたすべての [追加リソース](../developing/addons/workload.ja.md) が記述され、Service または環境がデプロイされる場所に展開されます。

### シナリオの例
#### Service に接続されたS3ストレージ

```console
$ copilot storage init --storage-type S3 --name bucket \
--workload fe --lifecycle workload
```

Service に接続するS3バケット用のCloudFormationテンプレートが生成されます。

```console
$ copilot svc deploy --name fe --env test
$ copilot svc deploy --name fe --env prod
```

このコマンドを実行すると、test 環境と prod 環境の2つのバケットが展開され、それぞれの環境の fe Service からのみアクセスできるようになります。

#### 環境に接続されたS3ストレージ

これは、実行後に、
```console
$ copilot storage init --storage-type S3 --name bucket \
--workload fe --lifecycle environment
```
2 つのバケットがデプロイされます。1 つは "test" Environment、もう 1 つは "prod" Environment で、それぞれの "fe" Service からのみアクセスできます。
