# svc resume
```console
$ copilot svc resume [flags]
```

## コマンドの概要

!!! Note
  `svc resume` は "Request-Driven Web Service" タイプでのみサポートされています。

`copilot svc resume` は特定の Environment の Service に紐づけられた一時停止中の App Runner サービスを再開します。

## フラグ

```
  -a, --app string    Name of the application.
  -e, --env string    Name of the environment.
  -h, --help          help for resume
  -n, --name string   Name of the service.
```

## 実行例

一時停止中の App Runner サービス、"my-svc" を再開します。
```console
$ copilot svc resume -n my-svc
```
