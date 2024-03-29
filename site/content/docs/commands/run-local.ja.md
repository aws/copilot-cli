# run local
```console
$ copilot run local [flags]
```

## コマンドの概要
`copilot run local` はワークロードをローカルで実行します。

## フラグ
```
  -a, --app string                        Name of the application.
  -e, --env string                        Name of the environment.
      --env-var-override stringToString   Optional. Override environment variables passed to containers.
                                          Format: [container]:KEY=VALUE. Omit container name to apply to all containers. (default [])
  -h, --help                              help for run
  -n, --name string                       Name of the service or job.
      --port-override list                Optional. Override ports exposed by service. Format: <host port>:<service port>.
                                          Example: --port-override 5000:80 binds localhost:5000 to the service's port 80. (default [])
      --proxy                             Optional. Proxy outbound requests to your environment's VPC.
      --proxy-network ipNet               proxy-network (default 172.20.0.0/16)
      --watch                             Optional. Watch changes to local files and restart containers when updated.
```

## 実行例
Environment "test" の Service "mysvc" をローカルで実行します。
```console
$ copilot run local --name mysvc --env test
```