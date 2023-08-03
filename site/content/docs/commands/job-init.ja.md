# job init
```console
$ copilot job init
```

## コマンドの概要

`copilot job init` は新しく [Job](../concepts/jobs.ja.md) を作成します。

このコマンドを実行すると、 Copilot CLI は [Manifest ファイル](../manifest/overview.ja.md) を格納するための Application 名がついたディレクトリを `copilot` ディレクトリ配下に作成します。 Job のデフォルト設定を変更したい場合は Manifest ファイルを更新してください。 CLI はさらに全ての [Environment](../concepts/environments.ja.md) からプルできるポリシーをアタッチした ECR リポジトリをセットアップします。最後に Job は AWS Systems Manager パラメータストアに登録され CLI が Job をトラックできるようになります。

その後すでにセットアップされた Environment があるなら、 `copilot job deploy` コマンドを実行してその Environment に Job をデプロイできます。

## フラグ

```
      --allow-downgrade     Optional. Allow using an older version of Copilot to update Copilot components
                            updated by a newer version of Copilot.
  -a, --app string          Name of the application.
  -d, --dockerfile string   Path to the Dockerfile.
                            Mutually exclusive with -i, --image.
  -h, --help                help for init
  -i, --image string        The location of an existing Docker image.
                            Mutually exclusive with -d, --dockerfile.
  -t, --job-type string     Type of job to create. Must be one of:
                            "Scheduled Job".
  -n, --name string         Name of the job.
      --retries int         Optional. The number of times to try restarting the job on a failure.
  -s, --schedule string     The schedule on which to run this job. 
                            Accepts cron expressions of the format (M H DoM M DoW) and schedule definition strings. 
                            For example: "0 * * * *", "@daily", "@weekly", "@every 1h30m".
                            AWS Schedule Expressions of the form "rate(10 minutes)" or "cron(0 12 L * ? 2021)"
                            are also accepted.
      --timeout string      Optional. The total execution time for the task, including retries.
                            Accepts valid Go duration strings. For example: "2h", "1h30m", "900s".
```

## 実行例

1 日 1 回実行される "reaper" という名前のスケジュールされたタスクを作成します。
```console
$ copilot job init --name reaper --dockerfile ./frontend/Dockerfile --schedule "@daily"
```
リトライ回数を指定した "report-generator" という名前のスケジュールされたタスクを作成します。
```console
$ copilot job init --name report-generator --schedule "@monthly" --retries 3 --timeout 900s
```
