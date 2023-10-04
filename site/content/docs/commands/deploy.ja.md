# copilot deploy
```console
$ copilot deploy
```

## コマンドの概要 

このコマンドはローカル Manifest から Service 及び Environment をデプロイするために使用されます。このコマンドはデプロイされたインフラストラクチャとローカル Manifest をチェックし、Environment の初期化とデプロイ、およびワークロードのデプロイを支援します。

ワークロードが初期化されていない場合、`--init-wkld` フラグでワークロードをデプロイする前に初期化することができます。

必要な Environment が初期化されていない場合は、`--init-env` フラグで初期化することができます。

`--deploy-env` フラグを指定して Environment のデプロイの確認をスキップするか、false (`--deploy-env=false`) に設定して Environment のデプロイ自体をスキップできます。

`copilot deploy` に含まれる手順は次のとおりです。

1. Service が存在しない場合は、必要に応じて Service を初期化
2. ターゲット Environment が存在しない場合は、必要に応じてカスタム認証情報を使用して初期化
3. 必要に応じて Service をデプロイする前に Environment をデプロイ
4. Manifest に `image.build` が存在する場合
    1. ローカルの Dockerfile からコンテナイメージを作成
    2. `--tag` で指定された値、または最新の git sha を利用してタグ付け(git 管理されている場合)
    3. コンテナイメージを ECR に対してプッシュ
5. Manifest ファイルと Addon をまとめて CloudFormation テンプレートにパッケージ
6. ECS タスク定義を作成/更新し、Job や Service を作成/更新

## フラグ

```
      --allow-downgrade                Optional. Allow using an older version of Copilot to update Copilot components
                                       updated by a newer version of Copilot.
  -a, --app string                     Name of the application.
      --aws-access-key-id string       Optional. An AWS access key for the environment account.
      --aws-secret-access-key string   Optional. An AWS secret access key for the environment account.
      --aws-session-token string       Optional. An AWS session token for temporary credentials.
      --deploy-env bool                Deploy the target environment before deploying the workload.
      --detach bool                    Optional. Skip displaying CloudFormation deployment progress.
  -e, --env string                     Name of the environment.
      --force                          Optional. Force a new service deployment using the existing image.
                                       Not available with the "Static Site" service type.
  -h, --help                           help for deploy
      --init-env bool                  Confirm initializing the target environment if it does not exist.
      --init-wkld bool                 Optional. Initialize a workload before deploying it.
  -n, --name string                    Name of the service or job.
      --no-rollback bool               Optional. Disable automatic stack 
                                       rollback in case of deployment failure.
                                       We do not recommend using this flag for a
                                       production environment.
      --profile string                 Name of the profile for the environment account.
      --region string                  Optional. An AWS region where the environment will be created.
      --resource-tags stringToString   Optional. Labels with a key and value separated by commas.
                                       Allows you to categorize resources. (default [])
      --tag string                     Optional. The tag for the container images Copilot builds from Dockerfiles.

```

!!!info
`--no-rollback` フラグは、サービスのダウンタイムを招く可能性があるため、本番環境にデプロイする場合は ***お勧めしません*** 。
自動スタックロールバックが無効になっている場合に、デプロイに失敗すると、手動でスタックを開始する必要があります。次のデプロイの前に AWS コンソールまたは AWS CLI を利用してスタックのスタックロールバックを手動で開始する必要があります。

## 実行例

"frontend" という名前の Service を "test" Environment にデプロイします。
```console
 $ copilot deploy --name frontend --env test 
```

"mailer" という名前の Job を、追加のリソースタグを付加して、"prod" Environment にデプロイします。
```console
$ copilot deploy -n mailer -e prod --resource-tags source/revision=bb133e7,deployment/initiator=manual
```

us-west-2 リージョンの "test" という名前の Environment をローカル Manifest を使用して "default" プロファイルの下に初期化してデプロイし、"api" という名前の Service をデプロイします。
```console
$ copilot deploy --init-env --deploy-env --env test --name api --profile default --region us-west-2
```

"backend" という名前の Service を初期化し、"prod" Environment にデプロイします。
```console
$ copilot deploy --init-wkld --deploy-env=false --env prod --name backend
```
