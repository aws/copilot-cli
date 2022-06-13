# pipeline deploy
```console
$ copilot pipeline deploy [flags]
```

## コマンドの概要
`copilot pipeline deploy` は、ワークスペース内の全ての Service に対する Pipeline をデプロイします。Pipeline 用 Manifest にて Application と紐付けられた Environment 群を利用します。

## フラグ
```
-a, --app string    Name of the application.
-h, --help          help for deploy
-n, --name string   Name of the pipeline.
    --yes           Skips confirmation prompt.
```

## 実行例
ワークスペース内の Service 群と Job 群に対する Pipeline をデプロイします。
```console
$ copilot pipeline deploy
```
