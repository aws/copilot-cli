# job package 
```console
$ copilot job package
```

## コマンドの概要

`copilot job package` は Job を Environment にデプロイするための CloudFormation テンプレートを生成します。

## フラグ

```
  -a, --app string          Name of the application.
      --diff                Compares the generated CloudFormation template to the deployed stack.
  -e, --env string          Name of the environment.
  -h, --help                help for package
  -n, --name string         Name of the job.
      --output-dir string   Optional. Writes the stack template and template configuration to a directory.
      --tag string          Optional. The container image tag.
      --upload-assets       Optional. Whether to upload assets (container images, Lambda functions, etc.).
                            Uploaded asset locations are filled in the template configuration.
```

## 実行例

"report-generator" Job を作成する CloudFormation テンプレートを、"test" Environment にデプロイする形で出力します。
 
```console
$ copilot job package -n report-generator -e test
```

CloudFormation テンプレートと設定を "infrastructure/" ディレクトリ以下に書き出します。
  
```console
$ copilot job package -n report-generator -e test --output-dir ./infrastructure
$ ls ./infrastructure
  report-generator-test.stack.yml      report-generator-test.params.yml
```
