# env package
```console
$ copilot env package [flags]
```

## コマンドの概要
`copilot env package` は、Envronment のデプロイに使用される CloudFormation スタックテンプレートと設定を出力します。

## フラグ
```console
  -a, --app string          Name of the application.
      --diff                Compares the generated CloudFormation template to the deployed stack.
      --force               Optional. Force update the environment stack template.
  -h, --help                help for package
  -n, --name string         Name of the environment.
      --output-dir string   Optional. Writes the stack template and template configuration to a directory.
      --upload-assets       Optional. Whether to upload assets (container images, Lambda functions, etc.).
                            Uploaded asset locations are filled in the template configuration.
```

## 実行例
"prod" Environment の CloudFormation テンプレートを出力し、カスタムリソースをアップロードします。
```console
$ copilot env package -n prod --upload-assets
```
CloudFormation のテンプレートと設定を stdout (標準出力) ではなく、"infrastructure/" サブディレクトリに書き出します。
```console
$ copilot env package -n test --output-dir ./infrastructure --upload-assets
$ ls ./infrastructure
test.env.yml      test.env.params.json
```
