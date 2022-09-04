# job logs
```console
$ copilot job logs
```

## コマンドの概要

`copilot job logs` は、デプロイされた Job のログを表示します。

## フラッグ

```  
  -a, --app string              Name of the application.
      --end-time string         Optional. Only return logs before a specific date (RFC3339).
                                Defaults to all logs. Only one of end-time / follow may be used.
  -e, --env string              Name of the environment.
      --follow                  Optional. Specifies if the logs should be streamed.
  -h, --help                    help for logs
      --include-state-machine   Optional. Include logs from the state machine executions.
      --json                    Optional. Output in JSON format.
      --last int                Optional. The number of executions of the scheduled job for which
                                logs should be shown. (default 1)
      --limit int               Optional. The maximum number of log events returned. Default is 10
                                unless any time filtering flags are set.
  -n, --name string             Name of the job.
      --since duration          Optional. Only return logs newer than a relative duration like 5s, 2m, or 3h.
                                Defaults to all logs. Only one of start-time / since may be used.
      --start-time string       Optional. Only return logs after a specific date (RFC3339).
                                Defaults to all logs. Only one of start-time / since may be used.
      --tasks strings           Optional. Only return logs from specific task IDs.

```

## 実行例 

"test" Environment の "my-job" Job のログを表示します。

```console
$ copilot job logs -n my-job -e test
```

過去 1 時間のログを表示します。

```console
$ copilot job logs --since 1h
```

過去 4 回分の Job の実行ログを表示します。

```console
$ copilot job logs --last 4
```

特定のタスク ID のログを表示します。
```console
$ copilot job logs --tasks 709c7ea,1de57fd
```

ログをリアルタイムで表示します。
```console
$ copilot job logs --follow
```

前回実行時のコンテナログとステートマシンの実行ログを表示します。
```console
$ copilot job logs --include-state-machine --last 1
```
