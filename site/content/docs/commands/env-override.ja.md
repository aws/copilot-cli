# copilot env override
```console
$ copilot env override
```

## コマンドの概要
Envrionment の Infrastructure-as-Code (IaC) 拡張ファイルのスキャフォルドです。
生成されたファイルを利用して、 Copilot が生成した AWS CloudFormation テンプレートを拡張し、
オーバーライドできます。
このファイルを編集し、既存リソースのプロパティを変更できます。また、
Environment のテンプレートに削除や新規リソースの追加ができます。

### 詳細説明

オーバライドについて、より詳細について確認したい場合は[YAML パッチ](../developing/overrides/yamlpatch.ja.md) や、[AWS Cloud Development Kit](../developing/overrides/cdk.ja.md)を確認してください。

## フラグ

```console
  -a, --app string            Name of the application.
      --cdk-language string   Optional. The Cloud Development Kit language. (default "typescript")
  -h, --help                  Help for override
  -n, --name string           Optional. Name of the environment to use when retrieving resources in a template.
                              Defaults to a random environment.
      --skip-resources        Optional. Skip asking for which resources to override and generate empty IaC extension files.
      --tool string           Infrastructure as Code tool to override a template.
                              Must be one of: "cdk" or "yamlpatch".
```

## 実行例

Environment テンプレートをオーバライドするために、新しい Cloud Development Kit アプリケーションを作成します。

```console
$ copilot env override --tool cdk
```

## 出力例

![env-override](https://user-images.githubusercontent.com/879348/227585768-44d5d91f-11d5-4d4b-a5fa-12bb5239710f.gif)