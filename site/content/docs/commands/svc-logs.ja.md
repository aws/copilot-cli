# svc logs
```console
$ copilot svc logs
```

## コマンドの概要

`copilot svc logs` はデプロイ済みの Service のログを表示します。
(ログは Static Site Service では利用できません。)

## フラグ

```
  -a, --app string          Name of the application. (default "testing-buildspec")
      --container string    Optional. Return only logs from a specific container.
      --end-time string     Optional. Only return logs before a specific date (RFC3339).
                            Defaults to all logs. Only one of end-time / follow may be used.
  -e, --env string          Name of the environment.
      --follow              Optional. Specifies if the logs should be streamed.
  -h, --help                help for logs
      --json                Optional. Output in JSON format.
      --limit int           Optional. The maximum number of log events returned. Default is 10
                            unless any time filtering flags are set.
      --log-group string    Optional. Only return logs from specific log group.
  -n, --name string         Name of the service.
  -p, --previous            Optional. Print logs for the last stopped task if exists.
      --since duration      Optional. Only return logs newer than a relative duration like 5s, 2m, or 3h.
                            Defaults to all logs. Only one of start-time / since may be used.
      --start-time string   Optional. Only return logs after a specific date (RFC3339).
                            Defaults to all logs. Only one of start-time / since may be used.
      --tasks strings       Optional. Only return logs from specific task IDs.
```

## 実行例

"test" Environment の "my-svc" Service のログを表示します。

```console
$ copilot svc logs -n my-svc -e test
```

過去 1 時間のログを表示します。

```console
$ copilot svc logs --since 1h
```

2006-01-02T15:04:05 から 2006-01-02T15:05:05 までのログを表示します。

```console
$ copilot svc logs --start-time 2006-01-02T15:04:05+00:00 --end-time 2006-01-02T15:05:05+00:00
```
