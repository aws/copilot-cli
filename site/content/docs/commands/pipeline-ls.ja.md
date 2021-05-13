# pipeline ls
```bash
$ copilot pipeline ls [flags]
```

## コマンドの概要
`copilot pipeline ls` は、Application にデプロイされた全ての Pipeline を一覧表示します。

## フラグ
```bash
-a, --app string   Name of the application.
-h, --help         help for ls
    --json         Optional. Outputs in JSON format.
```

## 実行例
Application "phonetool" のすべての Pipeline を一覧表示します。

```bash
$ copilot pipeline ls -a phonetool
```
