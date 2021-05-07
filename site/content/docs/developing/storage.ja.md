# ストレージ

Copilot ワークロードに永続性を追加するには 2 つの方法があります。[`copilot storage init`](#データベースおよびアーティファクト) を使用してデータベースや S3 バケットを作成する方法、あるいは Manifest の [`storage` フィールド](#ファイルシステム) を使用して既存の EFS ファイルシステムをアタッチする方法です。

## データベースおよびアーティファクト

Job や Service に、データベースや S3 バケットを追加するには [`copilot storage init`](../commands/storage-init.md) を実行します。

```bash
# ウィザードに従い S3 バケットを作成する
$ copilot storage init -t S3

# "api" Service からアクセス可能な "my-bucket" という名前の S3 バケットを作成する
$ copilot storage init -n my-bucket -t S3 -w api
```

このコマンドにより、"api" Service の [addons](../developing/additional-aws-resources.md) ディレクトリに S3 バケットを定義した CloudFormation テンプレートが作成されます。続いて `copilot deploy -n api` を実行することで S3 バケットが作成されます。`api` タスクロールに S3 バケットへのアクセス権限が付与され、バケット名が `api` コンテナの環境変数に `MY_BUCKET_NAME` の形で設定されます。

!!!info
    すべての名前は、ハイフンやアンダースコアに基づいて SCREAMING_SNAKE_CASE のように変換されます。`copilot svc show` を実行することで、Service の環境変数を確認できます。

`copilot storage init` を実行して [DynamoDB テーブル](https://docs.aws.amazon.com/ja_jp/amazondynamodb/latest/developerguide/Introduction.html) の作成も可能です。例えば、ソートキーおよびローカルセカンダリインデックスを持つテーブルを定義した CloudFormation テンプレートは、次のコマンドで作成可能です。

```bash
# ウィザードに従い DynamoDB テーブルを作成する
$ copilot storage init -t DynamoDB

# もしくは、DynamoDB テーブルの作成に必要な情報をフラグで指定する
$ copilot storage init -n users -t DynamoDB -w api --partition-key id:N --sort-key email:S --lsi post-count:N
```

このコマンドにより `${app}-${env}-${svc}-users` という名前の DynamoDB テーブルが作成されます。パーティションキーは `id` となり、データ型は数値です。ソートキーは `email` となり、データ型は文字列です。また、[ローカルセカンダリインデックス](https://docs.aws.amazon.com/ja_jp/amazondynamodb/latest/developerguide/LSI.html) (代替のソートキー) として、データ型が数値である `post-count` が作成されます。

同様に、`copilot storage init` を実行して [RDS Aurora Serverless](https://docs.aws.amazon.com/ja_jp/AmazonRDS/latest/AuroraUserGuide/aurora-serverless.html) クラスターを作成できます。
```bash
# ウィザードに従い RDS Aurora Serverless クラスターを作成する
$ copilot storage init -t Aurora

# もしくは、RDS Aurora Serverless クラスターの作成に必要な情報をフラグで指定する
$ copilot storage init -n my-cluster -t Aurora -w api --engine PostgreSQL --initial-db my_db
```
このコマンドにより PostgreSQL エンジンを使用する `my_db` という名前のデータベースを持つ RDS Aurora Serverless クラスターが作成されます。JSON 文字列として `MYCLUSTER_SECRET` という名前の環境変数がワークロードに追加されます。この JSON 文字列は、`'host'`、`'port'`、`'dbname'`、`'username'`、`'password'`、`'dbClusterIdentifier'`、`'engine'` フィールドを含みます。

## ファイルシステム
Copilot タスクに EFS ボリュームをマウントするには次の 2 つを満たす必要があります。

1. Environment のリージョンに [EFS ファイルシステム](https://docs.aws.amazon.com/ja_jp/efs/latest/ug/whatisefs.html) を作成する
2. Environment の各サブネットに、Copilot Environment のセキュリティグループを使用した [EFS マウントターゲット](https://docs.aws.amazon.com/ja_jp/efs/latest/ug/accessing-fs.html) を作成する

これらの前提条件を満たしている場合、Manifest に設定を追加することで EFS ストレージが使用できます。設定には、ファイルシステム ID と、使用する場合は EFS ファイルシステムのアクセスポイント情報が必要です。

!!!info
    特定のファイルシステムは、一度に 1 つの Environment でのみ使用可能です。マウントターゲットは、アベイラビリティーゾーンごとで 1 つに制限されています。したがって、Copilot タスクにマウントする予定の EFS ボリュームを別の VPC で使用したことがある場合は、Copilot タスクで使用する前に既存のマウントターゲットを削除する必要があります。

### Manifest 構文
EFS ボリュームをもっともシンプルに指定できるのは次の構文です。

```yaml
storage:
  volumes:
    myEFSVolume: # このキーは任意の文字列を指定可能
      path: '/etc/mount1'
      efs:
        id: fs-1234567
```

この構文により、ファイルシステム `fs-1234567` を使用して Service または Job のコンテナに読み取り専用のボリュームが作成されます。Environment のサブネットにマウントターゲットが作成されていない場合、タスクの起動に失敗します。

ストレージの完全な構文は次の通りです。

```yaml
storage:
  volumes:
    {{ volume name }}:
      path: {{ mount path }}             # 必須。コンテナ内でのパスを指定
      read_only: {{ boolean }}           # デフォルト: true
      efs:
        id: {{ filesystem ID }}          # 必須
        root_dir: {{ filesystem root }}  # オプション。デフォルトは "/"。
                                         # アクセスポイントを使用する場合、このパラメータは指定しない
        auth:
          iam: {{ boolean }}             # オプション。
                                         # このファイルシステムをマウントする際に IAM 認証を使用するかどうか
          access_point_id: {{ access point ID}} # オプション.
                                                # このファイルシステムをマウントするときに使用する EFS アクセスポイントの ID
```

### マウントターゲットの作成
既存の EFS ファイルシステムのマウントターゲットは、[AWS CLI](#aws-cli) や [CloudFormation](#cloudformation) などを使用して作成できます。

#### AWS CLI
既存のファイルシステムのマウントターゲットを作成する場合、以下の項目が必要です。

1. ファイルシステム ID
2. ファイルシステムと同じアカウントおよびリージョンにデプロイされた Copilot Environment

次の AWS CLI のコマンドでファイルシステム ID を取得できます。
```bash
$ EFS_FILESYSTEMS=$(aws efs describe-file-systems | \
  jq '.FileSystems[] | {ID: .FileSystemId, CreationTime: .CreationTime, Size: .SizeInBytes.Value}')
```

この変数を `echo` することで、必要なファイルシステムの情報が確認できます。ファイルシステム ID を `$EFS_ID` に設定し、手順を続けます。

同様に、Copilot Environment のパブリックサブネットとセキュリティグループが必要です。次の jq コマンドは、describe-stacks の結果から必要な情報を抽出しています。

!!!info
    使用するファイルシステムは Copilot Environment と同じリージョンに作成されている必要があります。

```bash
$ SUBNETS=$(aws cloudformation describe-stacks --stack-name ${YOUR_APP}-${YOUR_ENV} \
  | jq '.Stacks[] | .Outputs[] | select(.OutputKey == "PublicSubnets") | .OutputValue')
$ SUBNET1=$(echo $SUBNETS | jq -r 'split(",") | .[0]')
$ SUBNET2=$(echo $SUBNETS | jq -r 'split(",") | .[1]')
$ ENV_SG=$(aws cloudformation describe-stacks --stack-name ${YOUR_APP}-${YOUR_ENV} \
  | jq -r '.Stacks[] | .Outputs[] | select(.OutputKey == "EnvironmentSecurityGroup") | .OutputValue')
```

これらの情報を取得後、マウントターゲットを作成します。

```bash
$ MOUNT_TARGET_1_ID=$(aws efs create-mount-target \
    --subnet-id $SUBNET_1 \
    --security-groups $ENV_SG \
    --file-system-id $EFS_ID | jq -r .MountTargetID)
$ MOUNT_TARGET_2_ID=$(aws efs create-mount-target \
    --subnet-id $SUBNET_2 \
    --security-groups $ENV_SG \
    --file-system-id $EFS_ID | jq -r .MountTargetID)
```

コマンド実行後、先ほど示した Manifest のように `storage` を設定できます。

##### リソースのクリーンアップ

AWS CLI を使用してマウントターゲットを削除します。

```bash
$ aws efs delete-mount-target --mount-target-id $MOUNT_TARGET_1
$ aws efs delete-mount-target --mount-target-id $MOUNT_TARGET_2
```

#### CloudFormation
CloudFormation スタックを使用して、外部ファイルシステムに適切な EFS インフラストラクチャを作成する例を示します。

Environment の作成後、Environment と同じアカウントおよびリージョンに次の CloudFormation テンプレートをデプロイします。

次の CloudFormation テンプレートを `efs.yml` というファイル名で保存します。

```yaml
Parameters:
  App:
    Type: String
    Description: Your application's name.
  Env:
    Type: String
    Description: The environment name your service, job, or workflow is being deployed to.

Resources:
  EFSFileSystem:
    Metadata:
      'aws:copilot:description': 'An EFS File System for persistent backing storage for tasks and services'
    Type: AWS::EFS::FileSystem
    Properties:
      PerformanceMode: generalPurpose
      ThroughputMode: bursting
      Encrypted: true

  MountTargetPublicSubnet1:
    Type: AWS::EFS::MountTarget
    Properties:
      FileSystemId: !Ref EFSFileSystem
      SecurityGroups:
        - Fn::ImportValue:
            !Sub "${App}-${Env}-EnvironmentSecurityGroup"
      SubnetId: !Select
        - 0
        - !Split
            - ","
            - Fn::ImportValue:
                !Sub "${App}-${Env}-PublicSubnets"

  MountTargetPublicSubnet2:
    Type: AWS::EFS::MountTarget
    Properties:
      FileSystemId: !Ref EFSFileSystem
      SecurityGroups:
        - Fn::ImportValue:
            !Sub "${App}-${Env}-EnvironmentSecurityGroup"
      SubnetId: !Select
        - 1
        - !Split
            - ","
            - Fn::ImportValue:
                !Sub "${App}-${Env}-PublicSubnets"
Outputs:
  EFSVolumeID:
    Value: !Ref EFSFileSystem
    Export:
      Name: !Sub ${App}-${Env}-FilesystemID
```

次のコマンドを実行します。
```bash
$ aws cloudformation deploy
    --stack-name efs-cfn \
    --template-file ecs.yml
    --parameter-overrides App=${YOUR_APP} Env=${YOUR_ENV}
```

これにより EFS ファイルシステムと、Copilot Environment スタックからの出力を使用して、タスクに必要なマウントターゲットが作成されます。

ファイルシステム ID を取得するには、`describe-stacks` を実行します。

```bash
$ aws cloudformation describe-stacks --stack-name efs-cfn | \
    jq -r '.Stacks[] | .Outputs[] | .OutputValue'
```

次に、EFS ファイルシステムにアクセスさせたい Service の Manifest に以下の設定を追加します。

```yaml
storage:
  volumes:
    copilotVolume: # このキーは任意の文字列を指定可能
      path: '/etc/mount1'
      read_only: true # Service に書き込み処理が必要な場合は false を指定
      efs:
        id: {{ your filesystem ID }}
```

最後に、`/etc/mount1` にファイルシステムをマウントするため、`copilot svc deploy` を実行して Service の設定を変更します。

##### リソースのクリーンアップ
クリーンアップをするには、Manifest から `storage` 設定を削除して Service を再デプロイします。
```bash
$ copilot svc deploy
```

次に、スタックを削除します。

```bash
$ aws cloudformation delete-stack --stack-name efs-cfn
```
