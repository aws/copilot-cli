# env delete
```console
$ copilot env delete [flags]
```

## コマンドの概要
`copilot env delete`は、Applicationから Environment を削除します。 Environment 内に実行中のアプリケーションがある場合は、はじめに [`copilot svc delete`](../commands/svc-delete.ja.md) を実行する必要があります。

質問に答えた後、Environment 用の AWS CloudFormation スタックが削除されたことを確認してください。

## フラグ
```
-h, --help             help for delete
-n, --name string      Name of the environment.
    --yes              Skips confirmation prompt.
-a, --app string       Name of the application.
```

## 実行例
"test" Environment を削除します。
```console
$ copilot env delete --name test 
```
"test" Environment をプロンプトなしで削除します。
```console
$ copilot env delete --name test --yes
```
