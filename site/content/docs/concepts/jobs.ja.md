# Job

Job はイベントによって起動される Amazon ECS タスクを表す概念です。Copilot は現時点では "Scheduled Jobs" (スケジュール実行される Job)のみをサポートしています。固定スケジュールで、あるいは間隔を指定して一定時間ごとに実行できます。

## Job の作成

Job を作成するもっとも簡単な方法は Dockerfile が置かれたディレクトリで `init` コマンドを実行することです。

```console
$ copilot init
```

Job のどの Application に所属させるかを選択すると、Copilot は作成したい Job の __タイプ__ を尋ねます。現時点で指定可能なタイプは "Scheduled Job" のみです。

## Manifest と設定

`copilot init` コマンドの実行が完了すると、`manifest.yml` というファイルが `copilot/[job name]/` ディレクトリに作成されます。
[Scheduled Job の Manifest](../manifest/scheduled-job.ja.md) はシンプルな宣言的ファイルで、スケジュール実行されるタスクの一般的な共通設定が含まれています。例えば、いつ Job を実行したいのか、割り当てるリソースサイズ、処理のタイムアウト時間や失敗時に何回までリトライを試みるかといったことを設定できます。

## Job のデプロイ

要件を満たすように Manifest ファイルを編集したら、deploy コマンドを使ってそれらの変更をデプロイできます。

```console
$ copilot deploy
```

このコマンドを実行すると、続けて以下のような作業が実施されます。

1. ローカル環境でのコンテナイメージのビルド  
2. Job 用の ECR リポジトリへのプッシュ
3. Manifest ファイルの CloudFormation テンプレートへの変換  
4. 追加インフラストラクチャがある場合、それらの CloudFormation テンプレートへのパッケージング  
5. デプロイ

## Job に関連するその他のオプション

新しい Job をテストしたい、あるいは何らかの理由で実行したい場合があります。その場合、[`job run`](../commands/job-run.en.md) コマンドを実行します。
```console
$ copilot job run
```

Job を削除せず、一時的に Job を無効化したい場合、 [Manifest](../manifest/scheduled-job.ja.md)内でスケジュールを `none` に設定します。
```yaml
on:
  schedule: "none"
```

設定した Job の CloudFormation テンプレートを表示したい場合、[`job package`](../commands/job-package.ja.md)を実行します。
```console
$ copilot job package
```

### Job に含まれるものを確認したい

Copilot では内部的に CloudFormation を利用しているため、作成されるすべてのリソースは Copilot によってタグ付けされています。"Scheduled Jobs" には、Amazon ECS タスク定義、タスクロール、タスク実行ロール、タスクの処理が失敗した際にリトライを可能にする Step Functions ステートマシン、そしてステートマシンを実行する EventBridge ルールといったものが含まれます。

### Job のログを確認したい

Job のログの確認も簡単です。[`copilot job logs`](../commands/job-logs.ja.md) を実行すると、Job の最新のログが表示されます。`--follow` フラグを指定すると、コマンドを実行した後に新しく呼び出された Job のログを表示し、ログを追跡できます。

```console
$ copilot job logs
copilot/myjob/37236ed Doing some work
copilot/myjob/37236ed Did some work
copilot/myjob/37236ed Exited...
copilot/myjob/123e300 Doing some work
copilot/myjob/123e300 Did some work
copilot/myjob/123e300 Did some additional work
copilot/myjob/123e300 Exited
```
