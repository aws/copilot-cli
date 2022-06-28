# svc exec
```console
$ copilot svc exec
```

## コマンドの概要
`copilot svc exec` は、Service で実行中のコンテナに対してコマンドを実行します。

## フラグ
```
  -a, --app string         Name of the application.
  -c, --command string     Optional. The command that is passed to a running container. (default "/bin/bash")
      --container string   Optional. The specific container you want to exec in. By default the first essential container will be used.
  -e, --env string         Name of the environment.
  -h, --help               help for exec
  -n, --name string        Name of the service, job, or task group.
      --task-id string     Optional. ID of the task you want to exec in.
      --yes                Optional. Whether to update the Session Manager Plugin.
```

## 実行例

"frontend" Service のタスクにインタラクティブなセッションを開始します。

```console
$ copilot svc exec -a my-app -e test -n frontend
```

"backend" Service 内の ID "8c38184" から始まるタスクで 'ls' コマンドを実行します。

```console
$ copilot svc exec -a my-app -e test --name backend --task-id 8c38184 --command "ls"
```

## 出力例

<iframe width="560" height="315" src="https://www.youtube.com/embed/Evrl9Vux31k" frameborder="0" allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture" allowfullscreen></iframe>

!!! info
    1. Service デプロイ前に Manifest で `exec: true` が設定されていることを確認してください。
    2. これにより Service の Fargate Platform Version が 1.4.0 にアップデートされますのでご注意ください。プラットフォームバージョンをアップデートすると、[ECS サービスのリプレイス](https://docs.aws.amazon.com/ja_jp/AWSCloudFormation/latest/UserGuide/aws-resource-ecs-service.html#cfn-ecs-service-platformversion)となり、サービスのダウンタイムが発生します。
    3. `exec` は Windows コンテナではサポートされていません。
