# pipeline show
```console
$ copilot pipeline show [flags]
```

## コマンドの概要
`copilot pipeline show` は、Application にデプロイされた Pipeline の構成情報 (アカウント、リージョン、ステージなど) を表示します。

## フラグ
```
-a, --app string    Name of the application.
-h, --help          help for show
    --json          Optional. Output in JSON format.
-n, --name string   Name of the pipeline.
    --resources     Optional. Show the resources in your pipeline.
```

## 実行例
Pipeline "myapp-mybranch" に関する情報をリソース情報を含めて表示します。
```console
$ copilot pipeline show --name myrepo-mybranch --resources
```

## 出力例

![Running copilot pipeline show](https://raw.githubusercontent.com/kohidave/copilot-demos/master/pipeline-show.svg?sanitize=true)
