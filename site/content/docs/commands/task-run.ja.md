# task run
```
$ copilot task run
```

## コマンドの概要
`copilot task run` はスタンドアロンタスクをデプロイ、実行します。

task run に関連する一般的な手順は次の通りです。

1. タスク用の ECR リポジトリとロググループの作成
2. コンテナイメージのビルドと ECR へのプッシュ
3. タスク定義の作成、または更新
4. タスクを実行し、開始されるのを待つ

!!!info
    1. 同じグループ名のタスクは同じリソースセットを共有します。リソースセットには例えば CloudFormation スタック、ECR リポジトリ、CloudWatch ロググループ、タスク定義などが含まれます。
    2. `--env` オプションを利用してタスクを特定の Environment にデプロイする場合、そのタスクはデプロイ先 Environment のパブリックサブネットのみを利用します
    3. `--default` フラグの利用時に「デフォルトのクラスターが存在しない」旨のエラーが発生した場合、AWS CLI で `aws ecs create-cluster` コマンドを実行してから再度 `copilot run task` コマンドを実行してください。

## フラグ
```
  --app string                     Optional. Name of the application.
                                   Cannot be specified with 'default', 'subnets' or 'security-groups'
  --command string                 Optional. The command that is passed to "docker run" to override the default command.
  --count int                      Optional. The number of tasks to set up. (default 1)
  --cpu int                        Optional. The number of CPU units to reserve for each task. (default 256)
  --default                        Optional. Run tasks in default cluster and default subnets.
                                   Cannot be specified with 'app', 'env' or 'subnets'.
  --dockerfile string              Path to the Dockerfile. (default "Dockerfile")
  --entrypoint string              Optional. The entrypoint that is passed to "docker run" to override the default entrypoint.
  --env string                     Optional. Name of the environment.
                                   Cannot be specified with 'default', 'subnets' or 'security-groups'
  --env-vars stringToString        Optional. Environment variables specified by key=value separated with commas. (default [])
  --execution-role string          Optional. The role that grants the container agent permission to make AWS API calls.
  --follow                         Optional. Specifies if the logs should be streamed.
-h, --help                         help for run
  --image string                   Optional. The image to run instead of building a Dockerfile.
  --memory int                     Optional. The amount of memory to reserve in MiB for each task. (default 512)
  --resource-tags stringToString   Optional. Labels with a key and value separated with commas.
                                   Allows you to categorize resources. (default [])
  --security-groups strings        Optional. The security group IDs for the task to use. Can be specified multiple times.
                                   Cannot be specified with 'app' or 'env'.
  --subnets strings                Optional. The subnet IDs for the task to use. Can be specified multiple times.
                                   Cannot be specified with 'app', 'env' or 'default'.
  --tag string                     Optional. The container image tag in addition to "latest".
-n, --task-group-name string       Optional. The group name of the task. Tasks with the same group name share the same set of resources.
  --task-role string               Optional. The role for the task to use.
```
## 実行例
ローカルの Dockerfile を使用してタスクを実行します。
タスクグループ名とタスクを実行する Environment を指定します。
```
$ copilot task run --follow
```

現在のワークスペース配下の "test" Environment で、"db-migrate" という名前のタスクを実行します。
```
$ copilot task run -n db-migrate --env test --follow
```

2GB のメモリ、既存のイメージ、およびカスタムタスクロールを使用して 4 つのタスクを実行します。
```
$ copilot task run --num 4 --memory 2048 --image=rds-migrate --task-role migrate-role --follow
```

環境変数を使用してタスクを実行します。
```
$ copilot task run --env-vars name=myName,user=myUser
```

指定したサブネットとセキュリティグループを使用して、現在のワークスペース配下でタスクを実行します。
```
$ copilot task run --subnets subnet-123,subnet-456 --security-groups sg-123,sg-456
```

コマンドを指定してタスクを実行します。
```
$ copilot task run --command "python migrate-script.py"
```
