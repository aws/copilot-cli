# svc override
```console
$ copilot svc override
```

## 概要

Service の Infrastructure-as-Code (IaC) 拡張ファイルのスキャフォルドです。
生成されたファイルを利用して、 Copilot が生成した AWS CloudFormation テンプレートを拡張し、オーバーライドできます。
このファイルを編集し、既存リソースのプロパティを変更できます。また、
Service のテンプレートを削除や新規リソースの追加ができます。

### 詳細説明

オーバライドについて、より詳細について確認したい場合は[YAML パッチ](../developing/overrides/yamlpatch.ja.md) や、
[AWS Cloud Development Kit](../developing/overrides/cdk.ja.md)を確認してください。

## フラグ

```console
  -a, --app string            Name of the application.
      --cdk-language string   Optional. The Cloud Development Kit language. (default "typescript")
  -e, --env string            Optional. Name of the environment to use when retrieving resources in a template.
                              Defaults to a random environment.
  -h, --help                  Help for override
  -n, --name string           Name of the service.
      --skip-resources        Optional. Skip asking for which resources to override and generate empty IaC extension files.
      --tool string           Infrastructure as Code tool to override a template.
                              Must be one of: "cdk" or "yamlpatch".
```

## 実行例

"frontend" Service テンプレートをオーバライドするために、新しい Cloud Development Kit アプリケーションを作成します。

```console
$ copilot svc override -n frontend --tool cdk
```

## 出力例

![svc-override](https://user-images.githubusercontent.com/879348/227581322-7ef52595-4d92-47ff-860a-329c29ae1e04.gif)
