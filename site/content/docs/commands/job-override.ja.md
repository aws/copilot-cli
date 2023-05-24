# copilot job override
```console
$ copilot job override
```

## 概要
Job の Infrastructure-as-Code (IaC) 拡張ファイルのスキャフォルドです。
生成されたファイルを利用して、 Copilot が生成した AWS CloudFormation テンプレートを拡張し、オーバーライドできます。
このファイルを編集し、既存リソースのプロパティを変更できます。また、
Job のテンプレートの削除や新規リソースの追加ができます。

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
  -n, --name string           Name of the job.
      --skip-resources        Optional. Skip asking for which resources to override and generate empty IaC extension files.
      --tool string           Infrastructure as Code tool to override a template.
                              Must be one of: "cdk" or "yamlpatch".
```

## 実行例

"report" Job テンプレートをオーバライドするために、新しい Cloud Development Kit アプリケーションを作成します。

```console
$ copilot job override -n report --tool cdk
```

## 出力例

![job-override](https://user-images.githubusercontent.com/879348/227583979-cc112657-b0a8-4b7a-9e33-1db5489506fd.gif)