# svc package 
```bash
$ copilot svc package
```

## コマンドの概要

`copilot svc package` は任意の Environment に Service をデプロイする CloudFormation テンプレートを提供します。

## フラグ

```bash
  -e, --env string          Name of the environment.
  -h, --help                help for package
  -n, --name string         Name of the service.
      --output-dir string   Optional. Writes the stack template and template configuration to a directory.
      --tag string          Optional. The service's image tag.
```

## 実行例

CloudFormaiton スタックと設定を表示する代わりに、"infrastructure/" サブディレクトリへ書き込みます。

```bash
$ copilot svc package -n frontend -e test --output-dir ./infrastructure
$ ls ./infrastructure
frontend.stack.yml      frontend-test.config.yml
```

