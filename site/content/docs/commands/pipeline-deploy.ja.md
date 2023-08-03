# pipeline deploy
```console
$ copilot pipeline deploy [flags]
```

## コマンドの概要
`copilot pipeline deploy` は、ワークスペース内の全ての Service に対する Pipeline をデプロイします。Pipeline 用 Manifest にて Application と紐付けられた Environment 群を利用します。

## フラグ
```
      --allow-downgrade   Optional. Allow using an older version of Copilot to update Copilot components
                          updated by a newer version of Copilot.
  -a, --app string        Name of the application.
      --diff              Compares the generated CloudFormation template to the deployed stack.
  -h, --help              help for deploy
  -n, --name string       Name of the pipeline.
      --yes               Skips confirmation prompt.
```

## 実行例
ワークスペース内の Service 群と Job 群に対する Pipeline をデプロイします。
```console
$ copilot pipeline deploy
```
