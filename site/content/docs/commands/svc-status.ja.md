# svc status
```console
$ copilot svc status
```

## コマンドの概要
`copilot svc status` はデプロイ済み Service のへルスステータスを表示します。タスクのステータス、Service の種類に応じて、サービス、タスク、関連するアラームや、ログ、S3 バケットのデータが含まれます。

## フラグ
```
  -a, --app string    Name of the application.
  -e, --env string    Name of the environment.
  -h, --help          help for status
      --json          Optional. Output in JSON format.
  -n, --name string   Name of the service.
```

## 出力例

![Running copilot svc status](https://raw.githubusercontent.com/kohidave/copilot-demos/master/svc-status.svg?sanitize=true)
