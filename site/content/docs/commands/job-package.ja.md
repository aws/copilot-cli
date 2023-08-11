# job package 
```console
$ copilot job package
```

## コマンドの概要

`copilot job package` は Job を Environment にデプロイするための CloudFormation テンプレートを生成します。

## フラグ

```
      --allow-downgrade     Optional. Allow using an older version of Copilot to update Copilot components
                            updated by a newer version of Copilot.
  -a, --app string          Name of the application.
      --diff                Compares the generated CloudFormation template to the deployed stack.
  -e, --env string          Name of the environment.
  -h, --help                help for package
  -n, --name string         Name of the job.
      --output-dir string   Optional. Writes the stack template and template configuration to a directory.
      --tag string          Optional. The tag for the container images Copilot builds from Dockerfiles.
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


`--diff` を使用して、差分を出力し、終了します。
```console
$ copilot job deploy --diff
~ Resources:
    ~ TaskDefinition:
        ~ Properties:
            ~ ContainerDefinitions:
                ~ - (changed item)
                  ~ Environment:
                      (4 unchanged items)
                      + - Name: LOG_LEVEL
                      +   Value: "info"
```

!!! info "`copilot [noun] package --diff` を利用した場合の終了コード"
    0 = no diffs found  
    1 = diffs found  
    2 = error producing diffs
