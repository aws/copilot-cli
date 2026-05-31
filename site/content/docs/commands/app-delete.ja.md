# app delete
```console
$ copilot app delete [flags]
```

## コマンドの概要

`copilot app delete` Application に関連付けられた全てのリソースを削除します。

## フラグ

```
-h, --help          help for delete
-n, --name string   Name of the application.
    --yes           Skips confirmation prompt.
```

## 実行例
!!!warning "Application 名には `--name` を使用してください"
    `copilot app delete` は位置引数の Application 名を受け付けません。特定の Application を削除するには、`--name` で名前を渡します。

"phonetool" という名前の Application を強制的に削除します。
```console
$ copilot app delete --name phonetool --yes
```
