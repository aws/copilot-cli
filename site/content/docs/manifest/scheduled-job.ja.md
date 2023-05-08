以下は `'Scheduled Job'` Manifest で利用できるすべてのプロパティのリストです。[Job の概念](../concepts/jobs.ja.md)説明のページも合わせてご覧ください。

???+ note "スケジュールされた Job のサンプル Manifest"

    ```yaml
        name: report-generator
        type: Scheduled Job
    
        on:
          schedule: "@daily"
        cpu: 256
        memory: 512
        retries: 3
        timeout: 1h
    
        image:
          build: ./Dockerfile
    
        variables:
          LOG_LEVEL: info
        env_file: log.env
        secrets:
          GITHUB_TOKEN: GITHUB_TOKEN

        # 上記すべての値は Environment ごとにオーバーライド可能です。
        environments:
          prod:
            cpu: 2048
            memory: 4096
    ```

<a id="name" href="#name" class="field">`name`</a> <span class="type">String</span>  
Job 名。

<div class="separator"></div>

<a id="type" href="#type" class="field">`type`</a> <span class="type">String</span>  
Job のアーキテクチャタイプ。
現在、Copilot は定期的にもしくは固定したスケジュールでトリガーされるタスクである "Scheduled Job" タイプのみをサポートしています。

<div class="separator"></div>

<a id="on" href="#on" class="field">`on`</a> <span class="type">Map</span>  
Job をトリガーするイベントの設定。

<span class="parent-field">on.</span><a id="on-schedule" href="#on-schedule" class="field">`schedule`</a> <span class="type">String</span>  
定期的に Job をトリガーする頻度を指定できます。
サポートする頻度は:


| 頻度         | 以下と同一              | `UTC` を用いた可読表記による実行タイミング             |
| ------------ | --------------------- | --------------------------------------------- |
| `"@yearly"`  | `"cron(0 * * * ? *)"` | 1 月 1 日の午前 0 時                            |
| `"@monthly"` | `"cron(0 0 1 * ? *)"` | 毎月 1 日の午前 0 時                            |
| `"@weekly"`  | `"cron(0 0 ? * 1 *)"` | 毎週日曜日の午前 0 時                            |
| `"@daily"`   | `"cron(0 0 * * ? *)"` | 毎日午前 0 時                                   |
| `"@hourly"`  | `"cron(0 * * * ? *)"` | 毎時 0 分                                      |

* `"@every {duration}"` (例: "1m", "5m")
* `"rate({duration})"` CloudWatch の[rate 式](https://docs.aws.amazon.com/ja_jp/AmazonCloudWatch/latest/events/ScheduledEvents.html#RateExpressions) の形式

特定の時間に Job をトリガーしたい場合、cron でスケジュールを指定できます。

* `"* * * * *"` 標準的な [cron フォーマット](https://en.wikipedia.org/wiki/Cron#Overview)を利用する
* `"cron({fields})"` 6 つフィールドからなる CloudWatch の[cron 式](https://docs.aws.amazon.com/ja_jp/AmazonCloudWatch/latest/events/ScheduledEvents.html#CronExpressions) を利用する

最後に、`schedule` フィールドを `none` に設定することで、Job がトリガーされないようにすることができます。
```yaml
on:
  schedule: "none"
```

<div class="separator"></div>

{% include 'image.ja.md' %}

{% include 'image-config.ja.md' %}

<div class="separator"></div>

<a id="entrypoint" href="#entrypoint" class="field">`entrypoint`</a> <span class="type">String or Array of Strings</span>  
コンテナイメージのデフォルトエントリポイントをオーバーライドします。

```yaml
# 文字列による指定。
entrypoint: "/bin/entrypoint --p1 --p2"
# あるいは文字列配列による指定も可能。
entrypoint: ["/bin/entrypoint", "--p1", "--p2"]
```

<div class="separator"></div>

<a id="command" href="#command" class="field">`command`</a> <span class="type">String or Array of Strings</span>  
コンテナイメージのデフォルトコマンドをオーバーライドします。

```yaml
# 文字列による指定。
command: ps au
# あるいは文字列配列による指定も可能。
command: ["ps", "au"]
```

<div class="separator"></div>

<a id="cpu" href="#cpu" class="field">`cpu`</a> <span class="type">Integer</span>  
タスクに割り当てる CPU ユニット数。指定可能な値については [Amazon ECS ドキュメント](https://docs.aws.amazon.com/ja_jp/AmazonECS/latest/developerguide/task-cpu-memory-error.html)をご覧ください。

<div class="separator"></div>

<a id="memory" href="#memory" class="field">`memory`</a> <span class="type">Integer</span>  
タスクに割り当てるメモリ量（MiB）。指定可能な値については [Amazon ECS ドキュメント](https://docs.aws.amazon.com/ja_jp/AmazonECS/latest/developerguide/task-cpu-memory-error.html)をご覧ください。

<div class="separator"></div>

<a id="platform" href="#platform" class="field">`platform`</a> <span class="type">String</span>  
`docker build --platform` で渡すオペレーティングシステムとアーキテクチャ。（`[os]/[arch]` の形式で指定） 例えば、`linux/arm64` や `windows/x86_64` といった値です。デフォルトは `linux/x86_64` です。

生成された文字列を上書きして、有効な異なる `osfamily` や `architecture` を明示的に指定してビルドすることができます。例えば Windows ユーザーの場合は、
```yaml
platform: windows/x86_64
```
とするとデフォルトは `WINDOWS_SERVER_2019_CORE` が利用されますが、 Map を使って以下のように指定できます：
```yaml
platform:
  osfamily: windows_server_2019_full
  architecture: x86_64
```
```yaml
platform:
  osfamily: windows_server_2019_core
  architecture: x86_64
```
```yaml
platform:
  osfamily: windows_server_2022_core
  architecture: x86_64
```
```yaml
platform:
  osfamily: windows_server_2022_full
  architecture: x86_64
```

<div class="separator"></div>

<a id="retries" href="#retries" class="field">`retries`</a> <span class="type">Integer</span>  
Job が失敗するまでにリトライする回数。

<div class="separator"></div>

<a id="timeout" href="#timeout" class="field">`timeout`</a> <span class="type">Duration</span>  
Job の実行時間。この時間を超えた場合、Job は停止されて失敗となります。単位には `h`, `m`, `s`が利用できます。

<div class="separator"></div>

<a id="network" href="#network" class="field">`network`</a> <span class="type">Map</span>    
`network` セクションは VPC 内の AWS リソースに接続するためのパラメータを持ちます。

<span class="parent-field">network.</span><a id="network-vpc" href="#network-vpc" class="field">`vpc`</a> <span class="type">Map</span>  
タスクにアタッチするサブネットとセキュリティグループ。

<span class="parent-field">network.vpc.</span><a id="network-vpc-placement" href="#network-vpc-placement" class="field">`placement`</a> <span class="type">String</span>    
`'public'` か `'private'`のいずれかである必要があります。デフォルトではタスクはパブリックサブネットで起動します。

!!! info
    Copilot が作成した VPC の `'private'` サブネットを利用してタスクを実行する場合、Copilot は Environment に NAT ゲートウェイを追加します。あるいは Copilot 外で作成した VPC を `copilot env init` コマンドにてインポートしている場合は、その VPC に NAT ゲートウェイがあり、プライベートサブネットからインターネットへの接続性があることを確認してください。

<span class="parent-field">network.vpc.</span><a id="network-vpc-security-groups" href="#network-vpc-security-groups" class="field">`security_groups`</a> <span class="type">Array of Strings</span>  
タスクに関連づける追加のセキュリティグループのリスト。Copilot は常にセキュリティグループを含んでおり、環境内のコンテナは互いに通信できるようになっています。

<div class="separator"></div>

<a id="variables" href="#variables" class="field">`variables`</a> <span class="type">Map</span>   
Job に環境変数として渡される key-value ペア。Copilot ではデフォルトでいくつかの環境変数が含まれています。

<div class="separator"></div>

<a id="secrets" href="#secrets" class="field">`secrets`</a> <span class="type">Map</span>   
[AWS Systems Manager パラメータストア](https://docs.aws.amazon.com/ja_jp/systems-manager/latest/userguide/systems-manager-parameter-store.html) から環境変数として Job に安全に渡される key-value ペア。

<div class="separator"></div>

<a id="storage" href="#storage" class="field">`storage`</a> <span class="type">Map</span>  
Storage セクションではコンテナとサイドカーからマウントする外部の EFS ボリュームを指定します。これにより、データ処理や CMS のワークロードのために、リージョン内で永続ストレージへアクセスできるようになります。より詳しくは
[ストレージ](../developing/storage.ja.md) のページを確認してください。

<span class="parent-field">storage.</span><a id="volumes" href="#volumes" class="field">`volumes`</a> <span class="type">Map</span>  
アタッチする EFS ボリュームの名前と設定を指定します。`volumes` フィールドは次の形式の Map として指定されます:

```yaml
volumes:
  <volume name>:
    path: "/etc/mountpath"
    efs:
      ...
```

<span class="parent-field">storage.volumes.</span><a id="volume" href="#volume" class="field">`<volume>`</a> <span class="type">Map</span>  
ボリュームの設定を指定します。

<span class="parent-field">storage.volumes.`<volume>`.</span><a id="path" href="#path" class="field">`path`</a> <span class="type">String</span>  
必須項目。コンテナ内でボリュームをマウントしたい場所を指定します。利用できる文字は `a-zA-Z0-9.-_/` のみで、 242 文字未満である必要があります。

<span class="parent-field">storage.volumes.`<volume>`.</span><a id="read_only" href="#read-only" class="field">`read_only`</a> <span class="type">Bool</span>  
任意項目。デフォルトでは `true` です。ボリュームが読み込み専用か否かを定義します。 false の場合、コンテナにファイルシステムへの `elasticfilesystem:ClientWrite` 権限が付与され、ボリュームは書き込み可能になります。

<span class="parent-field">storage.volumes.`<volume>`.</span><a id="efs" href="#efs" class="field">`efs`</a> <span class="type">Map</span>  
より詳細な EFS の設定。

<span class="parent-field">storage.volumes.`<volume>`.efs.</span><a id="id" href="#id" class="field">`id`</a> <span class="type">String</span>  
必須項目。マウントするファイルシステムの ID 。

<span class="parent-field">storage.volumes.`<volume>`.efs.</span><a id="root_dir" href="#root-dir" class="field">`root_dir`</a> <span class="type">String</span>  
任意項目。デフォルトは `/` です。ボリュームのルートとして使用する EFS ファイルシステム内の場所を指定します。利用できる文字は `a-zA-Z0-9.-_/` のみで、 255 文字未満である必要があります。アクセスポイントを利用する場合、`root_dir` は空か `/` であり、`auth.iam` が `true` である必要があります。

<span class="parent-field">storage.volumes.`<volume>`.efs.</span><a id="auth" href="#auth" class="field">`auth`</a> <span class="type">Map</span>  
EFS の高度な認可の設定を指定します。

<span class="parent-field">storage.volumes.`<volume>`.efs.auth.</span><a id="iam" href="#iam" class="field">`iam`</a> <span class="type">Bool</span>  
任意項目。デフォルトは `true` です。volume の EFS への接続の可否の判定に IAM を利用するかしないかを設定します。

<span class="parent-field">storage.volumes.`<volume>`.efs.auth.</span><a id="access_point_id" href="#access-point-id" class="field">`access_point_id`</a> <span class="type">String</span>  
任意項目。デフォルトでは `""` が設定されます。接続する EFS アクセスポイントの ID です。アクセスポイントを利用する場合、`root_dir` は空か `/` であり、`auth.iam` が `true` である必要があります。

<div class="separator"></div>

<a id="logging" href="#logging" class="field">`logging`</a> <span class="type">Map</span>  
logging セクションには、コンテナの [FireLens](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/using_firelens.html) ログドライバ用のログ設定パラメータが含まれます。(設定例は[こちら](../developing/sidecars.ja.md#sidecar-patterns))

<span class="parent-field">logging.</span><a id="logging-image" href="#logging-image" class="field">`image`</a> <span class="type">Map</span>  
任意項目。使用する Fluent Bit のイメージ。デフォルト値は `public.ecr.aws/aws-observability/aws-for-fluent-bit:stable`。

<span class="parent-field">logging.</span><a id="logging-destination" href="#logging-destination" class="field">`destination`</a> <span class="type">Map</span>  
任意項目。Firelens ログドライバーにログを送信するときの設定。

<span class="parent-field">logging.</span><a id="logging-enableMetadata" href="#logging-enableMetadata" class="field">`enableMetadata`</a> <span class="type">Map</span>  
任意項目。ログに ECS メタデータを含むかどうか。デフォルトは `true`。

<span class="parent-field">logging.</span><a id="logging-secretOptions" href="#logging-secretOptions" class="field">`secretOptions`</a> <span class="type">Map</span>  
任意項目。ログの設定に渡す秘密情報です。

<span class="parent-field">logging.</span><a id="logging-configFilePath" href="#logging-configFilePath" class="field">`configFilePath`</a> <span class="type">Map</span>  
任意項目。カスタムの Fluent Bit イメージ内の設定ファイルのフルパス。

{% include 'publish.ja.md' %}

<div class="separator"></div>

<a id="environments" href="#environments" class="field">`environments`</a> <span class="type">Map</span>  
environments セクションは Environment の設定を Manifest で指定した値によって上書きできるようにします。
上記の例の Manifest では、 CPU のパラメータを上書きしているので production のコンテナはよりパフォーマンスが高くなります。
