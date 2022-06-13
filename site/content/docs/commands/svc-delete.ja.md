# svc delete
```console
$ copilot svc delete [flags]
```

## コマンドの概要

`copilot svc delete` は、特定の Environment 内の Service に関連付けられたすべてのリソースを削除します。

## フラグ

```
  -e, --env string    Name of the environment.
  -h, --help          help for delete
  -n, --name string   Name of the service.
      --yes           Skips confirmation prompt.
```

## 実行例
"test" アプリケーションを全ての Environment から強制的に削除します。
```console
$ copilot svc delete --name test --yes
```
