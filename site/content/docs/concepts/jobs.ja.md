Job はイベントによって起動される Amazon ECS タスクを表す概念です。Copilot は現時点では "Scheduled Jobs" (スケジュール実行される Job)のみをサポートしています。固定スケジュールで、あるいは間隔を指定して一定時間ごとに実行できます。

## Job の作成

Job を作成するもっとも簡単な方法は Dockerfile が置かれたディレクトリで `init` コマンドを実行することです。

```bash
$ copilot init
```

Job のどの Application に所属させるかを選択すると、Copilot は作成したい Job の __タイプ__ を尋ねます。現時点で指定可能なタイプは "Scheduled Job" のみです。

## Manifest と設定

`copilot init` コマンドの実行が完了すると、`manifest.yml` というファイルが作成されます。[Scheduled Job の Manifest](../manifest/scheduled-job.md) はシンプルな宣言的ファイルで、スケジュール実行されるタスクの一般的な共通設定が含まれています。例えば、いつ Job を実行したいのか、割り当てるリソースサイズ、処理のタイムアウト時間や失敗時に何回までリトライを試みるかといったことを設定できます。

## Job のデプロイ

要件を満たすように Manifest ファイルを編集したら、deploy コマンドを使ってそれらの変更をデプロイできます。

```bash
$ copilot deploy
```

このコマンドを実行すると、続けて以下のような作業が実施されます。

1. ローカル環境でのコンテナイメージのビルド  
2. Job 用の ECR リポジトリへのプッシュ
3. Manifest ファイルの CloudFormation テンプレートへの変換  
4. 追加インフラストラクチャがある場合、それらの CloudFormation テンプレートへのパッケージング  
5. デプロイ

### Job に含まれるものを確認したい

Copilot では内部的に CloudFormation を利用しているため、作成されるすべてのリソースは Copilot によってタグ付けされています。"Scheduled Jobs" には、Amazon ECS タスク定義、タスクロール、タスク実行ロール、タスクの処理が失敗した際にリトライを可能にする Step Functions ステートマシン、そしてステートマシンを実行する EventBridge ルールといったものが含まれます。
