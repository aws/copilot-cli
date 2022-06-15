# svc pause
```console
$ copilot svc pause [flags]
```

## コマンドの概要

!!! Note
  `svc pause` は "Request-Driven Web Service" タイプでのみサポートされています。

`copilot svc pause` は特定の Environment の Service に紐づけられた App Runner サービスを一時停止します。

## フラグ

```
  -a, --app string    Name of the application.
  -e, --env string    Name of the environment.
  -h, --help          help for pause
  -n, --name string   Name of the service.
      --yes           Skips confirmation prompt.
```

## 実行例

実行中の App Runner サービス、"my-svc" を一時停止します。
```console
$ copilot svc pause -n my-svc
```
