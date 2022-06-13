# job delete
```console
$ copilot job delete [flags]
```

## コマンドの概要

`copilot job delete` は特定の Environment 内で Job に関連づけられたすべてのリソースを削除します。

## フラグ

```
  -a, --app string    Name of the application.
  -e, --env string    Name of the environment.
  -h, --help          help for delete
  -n, --name string   Name of the job.
      --yes           Skips confirmation prompt.
```

## 実行例

"report-generator" Job を "my-app" Application から削除します。このコマンドはワークスペースの外からでも実行できます。
```console
$ copilot job delete --name report-generator --app my-app
```

"report-generator" Job を "prod" Environment からのみ削除します。
```console
$ copilot job delete --name report-generator --env prod
```

確認をプロンプトに表示せず "report-generator" Job を削除します。
```console
$ copilot job delete --name report-generator --yes
```
