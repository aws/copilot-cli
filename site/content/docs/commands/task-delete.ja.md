# task delete
```console
$ copilot task delete
```

## コマンドの概要
`copilot task delete` はタスクを停止し、関連するリソースを削除します。

!!!info
    v1.2.0 より前のバージョンの Copilot で作成されたタスクは `copilot task delete` で停止できません。v1.2.0 以前のバージョンで起動したタスクを使用しているお客様は、コマンド実行後に ECS コンソールを使用して手動でタスクを停止する必要があります。

## フラグ
```
  -a, --app string    Name of the application.
      --default       Optional. Delete a task which was launched in the default cluster and subnets.
                      Cannot be specified with 'app' or 'env'.
  -e, --env string    Name of the environment.
  -h, --help          help for delete
  -n, --name string   Name of the service.
      --yes           Optional. Skips confirmation prompt.
```
## 実行例
デフォルトのクラスターから、"test" タスクを削除します。
```console
$ copilot task delete --name test --default
```

prod Environment から、"db-migrate" タスクを削除します。
```console
$ copilot task delete --name db-migrate --env prod
```

確認のプロンプトを表示せずに、"test" タスクを削除します。
```console
$ copilot task delete --name test --yes
```
