# pipeline show
```bash
$ copilot pipeline show [flags]
```

## コマンドの概要
`copilot pipeline show` は、Application にデプロイされた Pipeline の構成情報 (アカウント、リージョン、ステージなど) を表示します。

## フラグ
```bash
-a, --app string    Name of the application.
-h, --help          help for show
    --json          Optional. Outputs in JSON format.
-n, --name string   Name of the pipeline.
    --resources     Optional. Show the resources in your pipeline.
```

## 実行例
Application "myapp" の Pipeline に関する情報を表示します。
```bash
$ copilot pipeline show --app myapp --resources
```

## 出力例

![Running copilot pipeline show](https://raw.githubusercontent.com/kohidave/copilot-demos/master/pipeline-show.svg?sanitize=true)
