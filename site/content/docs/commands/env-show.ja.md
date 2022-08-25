# env show
```console
$ copilot env show [flags]
```

## コマンドの概要
`copilot env show` は、特定の Environment に関する以下のような情報を表示します。

* Environment があるリージョンとアカウント
* Environment に現在デプロイされている Service
* Environment に関連するタグ

オプションで `--resources` フラグを付けると Environment に関連する AWS リソースが表示されます。

## フラグ
```
-a, --app string    Name of the application.
-h, --help          help for show
    --json          Optional. Output in JSON format.
-n, --name string   Name of the environment.
    --resources     Optional. Show the resources in your environment.
```
結果をプログラムでパースしたい場合 `--json` フラグを利用することができます。

## 実行例
"test" Environment に関する情報を表示します。
```console
$ copilot env show -n test
```
