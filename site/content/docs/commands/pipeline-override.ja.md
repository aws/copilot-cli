# copilot pipeline override
```console
$ copilot pipeline override
```

## コマンドの概要
Pipeline の Infrastructure-as-Code (IaC) 拡張ファイルのスキャフォルドです。
生成されたファイルを利用して、 Copilot が生成した AWS CloudFormation テンプレートを拡張し、オーバーライドできます。
このファイルを編集し、既存リソースのプロパティを変更できます。また、
Environment のテンプレートの削除や新規リソースの追加ができます。

### 詳細説明

オーバライドについて、より詳細について確認したい場合は[YAML パッチ](../developing/overrides/yamlpatch.ja.md) や、
[AWS Cloud Development Kit](../developing/overrides/cdk.ja.md)を確認してください。

## フラグ

```console
  -a, --app string            Name of the application.
      --cdk-language string   Optional. The Cloud Development Kit language. (default "typescript")
  -h, --help                  Help for override
  -n, --name string           Name of the pipeline.
      --skip-resources        Optional. Skip asking for which resources to override and generate empty IaC extension files.
      --tool string           Infrastructure as Code tool to override a template.
                              Must be one of: "cdk" or "yamlpatch".
```

## 実行例

"myrepo-main" Pipeline テンプレートをオーバライドするために、新しい Cloud Development Kit アプリケーションを作成します。

```console
$ copilot pipeline override -n myrepo-main --tool cdk
```

## 出力例

![pipeline-override](https://github.com/aws/copilot-cli/assets/10566468/21ecf58b-fc7e-4e20-a5b7-6b8e2049fda4)