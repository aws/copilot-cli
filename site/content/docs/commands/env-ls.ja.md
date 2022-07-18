# env ls
```console
$ copilot env ls [flags]
```

## コマンドの概要
`copilot env ls` は、Application 内の全ての Environment を一覧表示します。

## フラグ
```
-h, --help          help for ls
    --json          Optional. Output in JSON format.
-a, --app string    Name of the application.
```
結果をプログラムでパースしたい場合 `--json` フラグを利用することができます。

## 実行例
frontend Application の全ての Environment を一覧表示します。
```console
$ copilot env ls -a frontend
```

## 出力例

![Running copilot env ls](https://raw.githubusercontent.com/kohidave/copilot-demos/master/env-ls.svg?sanitize=true)
