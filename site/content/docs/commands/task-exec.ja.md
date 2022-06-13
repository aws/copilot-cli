# task exec
```console
$ copilot task exec
```

## コマンドの概要
`copilot task exec` は実行中のタスク内のコンテナでコマンドを実行します。

## フラグ
```
  -a, --app string       Name of the application.
  -c, --command string   Optional. The command that is passed to a running container. (default "/bin/bash")
      --default          Optional. Execute commands in running tasks in default cluster and default subnets.
                         Cannot be specified with 'app' or 'env'.
  -e, --env string       Name of the environment.
  -h, --help             help for exec
  -n, --name string      Name of the service, job, or task group.
      --task-id string   Optional. ID of the task you want to exec in.
```

## 実行例

現在のワークスペース配下の "test" Environment で、タスクグループ "db-migrate" 内のタスクへ対話型の bash セッションを開始します。

```console
$ copilot task exec -e test -n db-migrate
```

タスクグループ "db-migrate" 内の、ID "1848c38" のプレフィックスを持つタスクで 'cat progress.csv' コマンドを実行します。

```console
$ copilot task exec --name db-migrate --task-id 1848c38 --command "cat progress.csv"
```

デフォルトクラスター内で動作する、ID "38c3818" のプレフィックスを持つタスクへ対話型の bash セッションを開始します。

```console
$ copilot task exec --default --task-id 38c3818
```

!!! info
    `copilot task exec` は特定のタスクロールの権限がないと実行できません。既存のタスクロールを使用してタスクを実行する場合、 `copilot task exec` を動作させるために必要な以下の権限が付与されていることを確認してください。

```json
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Action": [
                "ssmmessages:CreateControlChannel",
                "ssmmessages:OpenControlChannel",
                "ssmmessages:CreateDataChannel",
                "ssmmessages:OpenDataChannel"
            ],
            "Resource": "*",
            "Effect": "Allow"
        },
        {
            "Action": [
                "logs:CreateLogStream",
                "logs:DescribeLogGroups",
                "logs:DescribeLogStreams",
                "logs:PutLogEvents"
            ],
            "Resource": "*",
            "Effect": "Allow"
        }
    ]
}
```
