# pipeline status
```bash
$ copilot pipeline status [flags]
```

## コマンドの概要
`copilot pipeline status` では、デプロイされた Pipeline のステージの状態を表示します。

## フラグ
```bash
-a, --app string    Name of the application.
-h, --help          help for status
    --json          Optional. Outputs in JSON format.
-n, --name string   Name of the pipeline.
```

## 実行例
Pipeline "pipeline-myapp-myrepo" の状態を表示します。
```bash
$ copilot pipeline status -n pipeline-myapp-myrepo
```

## 出力例

![Running copilot pipeline status](https://raw.githubusercontent.com/kohidave/copilot-demos/master/pipeline-status.svg?sanitize=true)
