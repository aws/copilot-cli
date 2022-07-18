# app show
```console
$ copilot app show [flags]
```

## コマンドの概要

`copilot app show` は Application の設定、Environment、Service を出力します。

## フラグ

```
-h, --help          help for show
    --json          Optional. Output in JSON format.
-n, --name string   Name of the application.
```

## 実行例
"my-app"という Application に関する情報を出力します。
```console
$ copilot app show -n my-app
```

## 出力例

![Running copilot app show](https://raw.githubusercontent.com/kohidave/copilot-demos/master/app-show.svg?sanitize=true)
