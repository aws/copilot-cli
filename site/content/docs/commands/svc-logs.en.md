# svc logs
```console
$ copilot svc logs
```

## What does it do?

`copilot svc logs` displays the logs of a deployed service.  
(Logs are not available for Static Site services.)

## What are the flags?

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

## Examples 

Displays logs of the service "my-svc" in environment "test".

```console
$ copilot svc logs -n my-svc -e test
```

Displays logs in the last hour.

```console
$ copilot svc logs --since 1h
```

Displays logs from 2006-01-02T15:04:05 to 2006-01-02T15:05:05.

```console
$ copilot svc logs --start-time 2006-01-02T15:04:05+00:00 --end-time 2006-01-02T15:05:05+00:00
```
