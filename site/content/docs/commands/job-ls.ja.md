# job ls
```console
$ copilot job ls
```

## コマンドの概要

`copilot job ls` は、特定の Application に含まれる全ての Copilot の Job を一覧表示します。

## フラグ

```
  -a, --app string   Name of the application.
  -h, --help         help for ls
      --json         Optional. Output in JSON format.
      --local        Only show jobs in the workspace.
```

## 実行例

"myapp" Application に含まれる全ての Job を一覧表示します。
```console
$ copilot job ls --app myapp
```
