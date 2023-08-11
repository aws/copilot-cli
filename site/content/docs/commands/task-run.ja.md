# task run
```console
$ copilot task run
```

## コマンドの概要
`copilot task run` はスタンドアロンタスクをデプロイ、実行します。

task run に関連する一般的な手順は次の通りです。

1. タスク用の ECR リポジトリとロググループの作成
2. コンテナイメージのビルドと ECR へのプッシュ
3. タスク定義の作成、または更新
4. タスクを実行し、開始されるのを待つ
5. タスクが 0 以外の終了コードで終了した場合、その終了コードを転送する

!!!info
    1. 同じグループ名のタスクは同じリソースセットを共有します。リソースセットには例えば CloudFormation スタック、ECR リポジトリ、CloudWatch ロググループ、タスク定義などが含まれます。
    2. `--env` オプションを利用してタスクを特定の Environment にデプロイする場合、そのタスクはデプロイ先 Environment のパブリックサブネットのみを利用します
    3. `--default` フラグの利用時に「デフォルトのクラスターが存在しない」旨のエラーが発生した場合、AWS CLI で `aws ecs create-cluster` コマンドを実行してから再度 `copilot run task` コマンドを実行してください。

## フラグ
```
Name Flags
  -n, --task-group-name string   Optional. The group name of the task. 
                                 Tasks with the same group name share the same set of resources. 
                                 (default directory name)

Build Flags
      --build-context string   Path to the Docker build context.
                               Cannot be specified with --image.
      --dockerfile string      Path to the Dockerfile.
                               Cannot be specified with --image. (default "Dockerfile")
  -i, --image string           The location of an existing Docker image.
                               Cannot be specified with --dockerfile or --build-context.
      --tag string             Optional. The container image tag in addition to "latest".

Placement Flags
      --app string                Optional. Name of the application.
                                  Cannot be specified with --default, --subnets or --security-groups.
      --cluster string            Optional. The short name or full ARN of the cluster to run the task in. 
                                  Cannot be specified with --app, --env or --default.
      --default                   Optional. Run tasks in default cluster and default subnets. 
                                  Cannot be specified with --app, --env or --subnets.
      --env string                Optional. Name of the environment.
                                  Cannot be specified with --default, --subnets or --security-groups.
      --security-groups strings   Optional. Additional security group IDs for the task to use. Can be specified multiple times.
      --subnets strings           Optional. The subnet IDs for the task to use. Can be specified multiple times.
                                  Cannot be specified with --app, --env or --default.

Task Configuration Flags
      --acknowledge-secrets-access     Optional. Skip the confirmation question and grant access to the secrets specified by --secrets flag. 
                                       This flag is useful only when 'secrets' flag is specified
      --command string                 Optional. The command that is passed to "docker run" to override the default command.
      --count int                      Optional. The number of tasks to set up. (default 1)
      --cpu int                        Optional. The number of CPU units to reserve for each task. (default 256)
      --entrypoint string              Optional. The entrypoint that is passed to "docker run" to override the default entrypoint.
      --env-file string                Optional. A path to an environment variable (.env) file with each line being of the form of VARIABLE=VALUE. Values specified with --env-vars take precedence over --env-file.
      --env-vars stringToString        Optional. Environment variables specified by key=value separated by commas. (default [])
      --execution-role string          Optional. The ARN of the role that grants the container agent permission to make AWS API calls.
      --memory int                     Optional. The amount of memory to reserve in MiB for each task. (default 512)
      --platform-arch string           Optional. Architecture of the task. Must be specified along with 'platform-os'.
      --platform-os string             Optional. Operating system of the task. Must be specified along with 'platform-arch'.
      --resource-tags stringToString   Optional. Labels with a key and value separated by commas.
                                       Allows you to categorize resources. (default [])
      --secrets stringToString         Optional. Secrets to inject into the container. Specified by key=value separated by commas. (default []).
                                       For secrets stored in AWS Parameter Store you can either specify names or ARNs.
                                       For the secrets stored in AWS Secrets Manager you need to specify ARNs.
      --task-role string               Optional. The ARN of the role for the task to use.

Utility Flags
      --follow                        Optional. Specifies if the logs should be streamed.
      --generate-cmd string           Optional. Generate a command with a pre-filled value for each flag.
                                      To use it for an ECS service, specify --generate-cmd <cluster name>/<service name>.
                                      Alternatively, if the service or job is created with Copilot, specify --generate-cmd <application>/<environment>/<service or job name>.
                                      Cannot be specified with any other flags.
      --acknowledge-secrets-access    Optional. Skip the confirmation question and grant access to the secrets specified by --secrets flag.
                                      This flag is useful only when '--secret' flag is specified
```

## 実行例
ローカルの Dockerfile を使用してタスクを実行し、タスクの実行後はログストリームを表示します。
コマンド実行後には質問が表示されますので、タスクを実行する Environment を指定します。
```console
$ copilot task run --follow
```

現在のワークスペース配下の "test" Environment で、"db-migrate" という名前のタスクを実行します。
```console
$ copilot task run -n db-migrate --env test --follow
```

2GB のメモリ、既存のイメージ、およびカスタムタスクロールを使用して 4 つのタスクを実行します。
```console
$ copilot task run --count 4 --memory 2048 --image=rds-migrate --task-role migrate-role --follow
```

環境変数を使用してタスクを実行します。
```console
$ copilot task run --env-vars name=myName,user=myUser
```

指定したサブネットとセキュリティグループを使用して、現在のワークスペース配下でタスクを実行します。
```console
$ copilot task run --subnets subnet-123,subnet-456 --security-groups sg-123,sg-456
```

コマンドを指定してタスクを実行します。
```console
$ copilot task run --command "python migrate-script.py"
```

Windows 2019 タスクを最小の CPU とメモリで実行します。 
```console
$ copilot task run --platform-os WINDOWS_SERVER_2019_CORE --platform-arch X86_64 --cpu 1024 --memory 2048
```

Windows 2022 タスクを最小の CPU とメモリで実行します。 
```console
$ copilot task run --platform-os WINDOWS_SERVER_2022_CORE --platform-arch X86_64 --cpu 1024 --memory 2048
```

AWS Secrets Manager のシークレットをコンテナに注入してタスクを実行します。
```console
$ copilot task run --secrets AuroraSecret=arn:aws:secretsmanager:us-east-1:535307839111:secret:AuroraSecret
```