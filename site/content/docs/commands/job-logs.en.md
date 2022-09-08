# job logs
```console
$ copilot job logs
```

## What does it do?

`copilot job logs` displays the logs of a deployed job.

## What are the flags?

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

## Examples 

Displays logs of the job "my-job" in environment "test".

```console
$ copilot job logs -n my-job -e test
```

Displays logs in the last hour.

```console
$ copilot job logs --since 1h
```

Displays logs from the last 4 executions of the job.

```console
$ copilot job logs --last 4
```

Displays logs from specific task IDs
```console
$ copilot job logs --tasks 709c7ea,1de57fd
```

Displays logs in real time.
```console
$ copilot job logs --follow
```

Displays container logs and state machine execution logs from the last execution.
```console
$ copilot job logs --include-state-machine --last 1
```
