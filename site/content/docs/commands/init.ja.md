# init
```console
$ copilot init
```

## コマンドの概要
`copilot init` は、コンテナアプリを AWS App Runner や Amazon ECS on AWS Fargate 上にデプロイしたい場合の出発点となります。Dockerfile を含むディレクトリ内で実行すると、あとは `init` の質問に答えていくだけですぐに Application を作成し、実行できます。

すべての質問に答えると、`copilot init` は ECR リポジトリをセットアップし、デプロイするかどうかを尋ねます。デプロイを選択すると、ネットワークスタックとロールを備えた新しい `test` Environemnt を作成します。そして、Dockerfile をビルドして Amazon ECR に Push し、Service や Job をデプロイします。

既存の Application への Service と Job の追加も `copilot init` で行えます。この場合は Service や Job を追加する Application の選択を求められます。

## フラグ

Copilot CLI の全てのコマンドと同様に、必要なフラグを指定しない場合は、アプリの実行に必要な情報をすべて入力するように求められます。フラグを介して情報を提供することで、プロンプトをスキップできます。

```
  -a, --app string          Name of the application.
      --deploy              Deploy your service or job to a "test" environment.
  -d, --dockerfile string   Path to the Dockerfile.
                            Mutually exclusive with -i, --image.
  -h, --help                help for init
  -i, --image string        The location of an existing Docker image.
                            Mutually exclusive with -d, --dockerfile.
  -n, --name string         Name of the service or job.
      --port uint16         Optional. The port on which your service listens.
      --retries int         Optional. The number of times to try restarting the job on a failure.
      --schedule string     The schedule on which to run this job. 
                            Accepts cron expressions of the format (M H DoM M DoW) and schedule definition strings. 
                            For example: "0 * * * *", "@daily", "@weekly", "@every 1h30m".
                            AWS Schedule Expressions of the form "rate(10 minutes)" or "cron(0 12 L * ? 2021)"
                            are also accepted.
      --tag string          Optional. The tag for the container images Copilot builds from Dockerfiles.
      --timeout string      Optional. The total execution time for the task, including retries.
                            Accepts valid Go duration strings. For example: "2h", "1h30m", "900s".
  -t, --type string         Type of service to create. Must be one of:
                            "Request-Driven Web Service", "Load Balanced Web Service", "Backend Service", "Scheduled Job".
```
