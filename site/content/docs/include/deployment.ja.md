<div class="separator"></div>

<a id="deployment" href="#deployment" class="field">`deployment`</a> <span class="type">Map</span>  
deployment セクションには、デプロイ中に実行されるタスクの数や、タスクの停止と開始の順序を制御するためのパラメータが含まれています。

<span class="parent-field">deployment.</span><a id="deployment-rolling" href="#deployment-rolling" class="field">`rolling`</a> <span class="type">String</span>  
ローリングデプロイ戦略。有効な値は以下の通りです。

- `"default"`: 古いタスクを停止する前に、更新されたタスク定義で必要な数だけ新しいタスクを作成します。内部的には、[`minimumHealthyPercent`](https://docs.aws.amazon.com/ja_jp/AmazonECS/latest/developerguide/service_definition_parameters.html#minimumHealthyPercent) を 100 に、[`maximumPercent`](https://docs.aws.amazon.com/ja_jp/AmazonECS/latest/developerguide/service_definition_parameters.html#maximumPercent) を 200 に設定することになります。
- `"recreate"`: 実行中のタスクをすべて停止し、新しいタスクを起動します。内部的には、[`minimumHealthyPercent`](https://docs.aws.amazon.com/ja_jp/AmazonECS/latest/developerguide/service_definition_parameters.html#minimumHealthyPercent) を 0 に、[`maximumPercent`](https://docs.aws.amazon.com/ja_jp/AmazonECS/latest/developerguide/service_definition_parameters.html#maximumPercent) を 100 に設定します。

<span class="parent-field">deployment.</span><a id="deployment-rollback-alarms" href="#deployment-rollback-alarms" class="field">`rollback_alarms`</a> <span class="type">文字列またはマップの配列</span>
!!! info
    デプロイの開始時にアラームが「In alarm」状態にある場合、Amazon ECS はそのデプロイの間、アラームを監視しません。詳細については、[こちらのドキュメント](https://docs.aws.amazon.com/ja_jp/AmazonECS/latest/userguide/deployment-alarm-failure.html)をお読みください。

文字列のリストとして、[デプロイのロールバック](https://docs.aws.amazon.com/ja_jp/AmazonECS/latest/userguide/deployment-alarm-failure.html)を引き起こす可能性のある、Service に関連付ける既存の CloudWatch アラームの名前を指定します。

```yaml
deployment:
  rollback_alarms: ["MyAlarm-ELB-4xx", "MyAlarm-ELB-5xx"]
```
マップとして、Copilot が作成したアラームのアラーム メトリックとしきい値
利用可能なメトリクス:
