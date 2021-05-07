# svc status
```
$ copilot svc status
```

## コマンドの概要
`copilot svc status` はデプロイ済み Service のステータス、タスクのステータス、関連する CloudWatch アラームなどのヘルスステータスを表示します。

## What are the flags?
```
  -a, --app string    Name of the application.
  -e, --env string    Name of the environment.
  -h, --help          help for status
      --json          Optional. Outputs in JSON format.
  -n, --name string   Name of the service.
```

## 出力例

![Running copilot svc status](https://raw.githubusercontent.com/kohidave/copilot-demos/master/svc-status.svg?sanitize=true)