# job run
```bash
$ copilot job run
```

## コマンドの概要

`copilot job run` は、スケジュールされた Job を実行します。

## フラグ
```console
  -a, --app string          Name of the application.
  -e, --env string          Name of the environment.
  -h, --help                help for package
  -n, --name string         Name of the job.
```

## 実行例

"report" という名前の Application で "report-gen" という名前の Job を "test" Environment で実行します。

```bash
$ copilot job run -a report -n report-gen -e test
```
