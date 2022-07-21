# pipeline ls
```console
$ copilot pipeline ls [flags]
```

## コマンドの概要
`copilot pipeline ls` は、Application にデプロイされた全ての Pipeline を一覧表示します。

## フラグ
```
-a, --app string   Name of the application.
-h, --help         help for ls
    --json         Optional. Output in JSON format.
    --local        Only show pipelines in the workspace.
```

## 実行例
Application "phonetool" のすべての Pipeline を一覧表示します。

```console
$ copilot pipeline ls -a phonetool
```
