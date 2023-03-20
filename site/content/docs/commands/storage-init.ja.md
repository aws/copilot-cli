# storage init
```console
$ copilot storage init
```
## コマンドの概要

`copilot storage init` は、Addon として新しいストレージリソースを作成します。

デフォルトでは、Copilot は「Database-per-service パターン」に従っています。
`copilot storage init`で指定した Service または Job だけが、そのストレージリソースにアクセスできます。
ストレージは、Service のコンテナ内から、ストレージリソースの名前またはリソースにアクセスするためのクレデンシャル情報を保持する環境変数を介してアクセスできます。

!!!note ""
    各ユーザーには独自の状況があります。データストレージを複数の Service 間で共有する必要がある場合は、Copilot で生成された CloudFormation テンプレートを変更して、目的を達成することができます。

ストレージリソースは、[ワークロード Addon](../developing/addons/workload.ja.md)として作成することができます。
これは、Service や Job の1つにアタッチされ、ワークロードと同時にデプロイされ削除されます。
例えば、`copilot svc deploy --name api`を実行すると、リソースは「api」とともにターゲット Environment にデプロイされます。

また、ストレージリソースは [Environment Addon](../developing/addons/environment.ja.md) として作成することができます。
Environment に関連づけられ、Environment と同時にデプロイされ削除されます。
例えば、`copilot env deploy --name test`を実行すると、test という Environment と一緒にリソースがデプロイされます。

リソースの種類は *S3*、*DynamoDB*、*Aurora* のいずれかを指定できます。


このコマンドを実行すると、CLI は `copilot/service` ディレクトリ内に、`addons` サブディレクトリが存在しなければ作成します。`copilot svc deploy` を実行すると、新規に初期化されたストレージリソースが、デプロイ先の　Environment に作成されます。デフォルトでは、`storage init` で指定した Service のみが、そのストレージリソースにアクセスできます。

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

`copilot storage init`は、Addon として新しいストレージリソースを作成します。"api" Service がフロントする "my-bucket" という名前の S3 バケットを Environment 単位で作成します。

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

Copilot は S3 バケット、DynamoDB テーブル、または Aurora Serverless クラスターを指定する CloudFormation テンプレートを `addons` ディレクトリに書き出します
`copilot [svc/job/env] deploy`を実行すると、CLI はこのテンプレートを `addons` ディレクトリの他のすべてのテンプレートとマージして、Service または Environment に関連するネストされたスタックを作成します。
このネストされたスタックには、Service または Environment に関連付けられたすべての [Addon リソース](../developing/addons/workload.ja.md) が記述され、Service または Environment がデプロイされる場所に展開されます。

### シナリオの例
#### Service に関連づけられた S3 ストレージ

```console
$ copilot storage init --storage-type S3 --name bucket \
--workload fe --lifecycle workload
```

Service に関連づけられた S3 バケット用の CloudFormation テンプレートが生成されます。

```console
$ copilot svc deploy --name fe --env test
$ copilot svc deploy --name fe --env prod
```

このコマンドを実行すると、test Environment と prod Environment の2つのバケットが展開され、それぞれの Environment の fe Service からのみアクセスできるようになります。

#### Environment に関連づけられた S3 ストレージ

これは、実行後に、
```console
$ copilot storage init --storage-type S3 --name bucket \
--workload fe --lifecycle environment
```
2 つのバケットがデプロイされます。1 つは "test" Environment、もう 1 つは "prod" Environment で、それぞれの "fe" Service からのみアクセスできます。
