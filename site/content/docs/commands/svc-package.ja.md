# svc package 
```console
$ copilot svc package
```

## コマンドの概要

`copilot svc package` は任意の Environment に Service をデプロイする CloudFormation テンプレートを提供します。

## フラグ

```
  -a, --app string          Name of the application.
  -e, --env string          Name of the environment.
  -h, --help                help for package
  -n, --name string         Name of the service.
      --output-dir string   Optional. Writes the stack template and template configuration to a directory.
      --tag string          Optional. The service's image tag.
      --upload-assets       Optional. Whether to upload assets (container images, Lambda functions, etc.).
                            Uploaded asset locations are filled in the template configuration.
```

## 実行例

CloudFormaiton スタックと設定を表示する代わりに、"infrastructure/" サブディレクトリへ書き込みます。

```console
$ copilot svc package -n frontend -e test --output-dir ./infrastructure
$ ls ./infrastructure
frontend.stack.yml      frontend-test.config.yml
```
