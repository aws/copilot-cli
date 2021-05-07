`'Scheduled Job'` Manifest で利用できるプロパティの一覧です。

???+ note "レポートを作成する cron ジョブの Manifest のサンプル。

    ```yaml
    # Your job name will be used in naming your resources like log groups, ECS Tasks, etc.
    name: report-generator
    type: Scheduled Job
    
    on:
      schedule: @daily
    cpu: 256
    memory: 512
    retries: 3
    timeout: 1h
    
    image:
      # Path to your service's Dockerfile.
      build: ./Dockerfile
    
    variables:
      LOG_LEVEL: info
    secrets:
      GITHUB_TOKEN: GITHUB_TOKEN
    ```

<a id="name" href="#name" class="field">`name`</a> <span class="type">String</span>  
Job の名前。


<div class="separator"></div>

<a id="type" href="#type" class="field">`type`</a> <span class="type">String</span>  
Job のアーキテクチャタイプ。
現在、Copilot は定期的にもしくは固定したスケジュールでトリガされるタスクである "Scheduled Job" タイプのみをサポートしています。

<div class="separator"></div>

<a id="on" href="#on" class="field">`on`</a> <span class="type">Map</span>  
Job をトリガするイベントの設定。

<span class="parent-field">on.</span><a id="on-schedule" href="#on-schedule" class="field">`schedule`</a> <span class="type">String</span>  
定期的に Job をトリガする頻度を指定できます。
サポートする頻度は:

* `"@yearly"`
* `"@monthly"`
* `"@weekly"`
* `"@daily"`
* `"@hourly"`
* `"@every {duration}"` (例: "1m", "5m")
* `"rate({duration})"` CloudWatch の[rate 式](https://docs.aws.amazon.com/ja_jp/AmazonCloudWatch/latest/events/ScheduledEvents.html#RateExpressions) の形式

特定の時間に Job をトリガしたい場合、cron でスケジュールを指定できます。

* `"* * * * *"` 標準的な [cron フォーマット](https://en.wikipedia.org/wiki/Cron#Overview)を利用する
* `"cron({fields})"` 6 つフィールドからなる CloudWatch の[cron 式](https://docs.aws.amazon.com/ja_jp/AmazonCloudWatch/latest/events/ScheduledEvents.html#CronExpressions) を利用する
<div class="separator"></div>

<a id="image" href="#image" class="field">`image`</a> <span class="type">Map</span>  
image セクションは Docker の build に関するパラメータを持ちます。

<span class="parent-field">image.</span><a id="image-build" href="#image-build" class="field">`build`</a> <span class="type">String or Map</span>  
String 型を設定した場合、Copilot はそれを Dockerfile へのパスと解釈します。指定したディレクトリがビルドコンテキストとなります。下記の Manifest を指定した場合:
```yaml
image:
  build: path/to/dockerfile
```
このコマンドを実行した場合と同じ結果になります。  
`$ docker build --file path/to/dockerfile path/to` 

Map 型も指定できます:

```yaml
image:
  build:
    dockerfile: path/to/dockerfile
    context: context/dir
    target: build-stage
    cache_from:
      - image:tag
    args:
      key: value
```
この場合、Copilot は指定したコンテキストディレクトリを使用します。また、args で指定した key-value のペアで `--build-arg` を上書きします。これは下記の docker コマンドの実行と同等です。

`$ docker build --file path/to/dockerfile --target build-stage --cache-from image:tag --build-arg key=value context/dir`.

フィールドは省略できます。その場合、Copilot は可能な限り意図を汲み取ろうと試みます。例えば、`context` を指定しても、`dockerfile`を指定しなかった場合、Copilot はコンテキストディレクトリで Docker を実行し、”Dockerfile”という名前のファイルを Dockerfile とみなします。逆に、`dockerfile`を指定し、`context`を指定しなかった場合、Copilot は `dockerfile` が配置されたディレクトリで Docker を実行したいのだと推測します。

全てのパスはワークスペースをルートとした相対パスで記述できます。

<span class="parent-field">image.</span><a id="image-location" href="#image-location" class="field">`location`</a> <span class="type">String</span>  
Dockerfile からコンテナを構築する代わりに、既存のイメージの名前を指定できます。このパラメータは[`image.build`](#image-build) と同時に使用できません。
`location` フィールドは Amazon ECS のタスク定義の[`image` パラメータ](https://docs.aws.amazon.com/ja_jp/AmazonECS/latest/developerguide/task_definition_parameters.html#container_definition_image) と同じ定義になっています。

<div class="separator"></div>

<a id="entrypoint" href="#entrypoint" class="field">`entrypoint`</a> <span class="type">String or Array of Strings</span>  
イメージのエントリーポイントを上書きします:

```yaml
# String の場合。
entrypoint: "/bin/entrypoint --p1 --p2"
# String の配列の場合
entrypoint: ["/bin/entrypoint", "--p1", "--p2"]
```

<div class="separator"></div>

<a id="command" href="#command" class="field">`command`</a> <span class="type">String or Array of Strings</span>  
イメージのコマンドを上書きします:

```yaml
# String の場合。
command: ps au
# String の配列の場合
command: ["ps", "au"]
```

<div class="separator"></div>

<a id="cpu" href="#cpu" class="field">`cpu`</a> <span class="type">Integer</span>  
タスクに割り当てる CPU ユニットの数を指定します。有効な CPU の値については[Amazon ECS のドキュメント ](https://docs.aws.amazon.com/ja_jp/AmazonECS/latest/developerguide/task-cpu-memory-error.html)をご確認ください。

<div class="separator"></div>

<a id="memory" href="#memory" class="field">`memory`</a> <span class="type">Integer</span>  
タスクが利用するメモリ量を MiB で指定します。有効なメモリの値については
[Amazon ECS のドキュメント](https://docs.aws.amazon.com/ja_jp/AmazonECS/latest/developerguide/task-cpu-memory-error.html) をご覧ください。

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

!!! info inline end
    `'private'` サブネットでインターネット接続が必要なタスクを実行するためには、`copilot env init` を実行したときに、NAT Gateway が存在する VPC をインポートしている必要があります。Copilot が生成した VPC における NAT Gateway のサポートについては、[#1959](https://github.com/aws/copilot-cli/issues/1959) を見てください。


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
[ストレージ](../developing/storage.md) のページを確認してください。

<span class="parent-field">storage.</span><a id="volumes" href="#volumes" class="field">`volumes`</a> <span class="type">Map</span>  
アタッチする EFS ボリュームの名前と設定を指定します。`volumes` フィールドは次の形式の Map として指定されます:

```yaml
volumes:
  {{ volume name }}:
    path: "/etc/mountpath"
    efs:
      ...
```

<span class="parent-field">storage.volumes.</span><a id="volume" href="#volume" class="field">`volume`</a> <span class="type">Map</span>  
ボリュームの設定を指定します。

<span class="parent-field">volume.</span><a id="path" href="#path" class="field">`path`</a> <span class="type">String</span>  
必須項目。コンテナ内でボリュームをマウントしたい場所を指定します。利用できる文字は `a-zA-Z0-9.-_/` のみで、 242 文字未満である必要があります。

<span class="parent-field">volume.</span><a id="read_only" href="#read-only" class="field">`read_only`</a> <span class="type">Bool</span>  
任意項目。デフォルトでは `true` です。ボリュームが読み込み専用か否かを定義します。 false の場合、コンテナにファイルシステムへの `elasticfilesystem:ClientWrite` 権限が付与され、ボリュームは書き込み可能になります。

<span class="parent-field">volume.</span><a id="efs" href="#efs" class="field">`efs`</a> <span class="type">Map</span>  
より詳細な EFS の設定。

<span class="parent-field">volume.efs.</span><a id="id" href="#id" class="field">`id`</a> <span class="type">String</span>  
必須項目。マウントするファイルシステムの ID 。

<span class="parent-field">volume.efs.</span><a id="root_dir" href="#root-dir" class="field">`root_dir`</a> <span class="type">String</span>  
任意項目。デフォルトは `/` です。ボリュームのルートとして使用する EFS ファイルシステム内の場所を指定します。利用できる文字は `a-zA-Z0-9.-_/` のみで、 255 文字未満である必要があります。アクセスポイントを利用する場合、`root_dir` は空か `/` であり、`auth.iam` が `true` である必要があります。

<span class="parent-field">volume.efs.</span><a id="auth" href="#auth" class="field">`auth`</a> <span class="type">Map</span>  
EFS の高度な認可の設定を指定します。

<span class="parent-field">volume.efs.auth.</span><a id="iam" href="#iam" class="field">`iam`</a> <span class="type">Bool</span>  
任意項目。デフォルトは `true` です。volume の EFS への接続の可否の判定に IAM を利用するかしないかを設定します。

<span class="parent-field">volume.efs.auth.</span><a id="access_point_id" href="#access-point-id" class="field">`access_point_id`</a> <span class="type">String</span>  
任意項目。デフォルトでは `""` が設定されます。接続する EFS アクセスポイントの ID です。アクセスポイントを利用する場合、`root_dir` は空か `/` であり、`auth.iam` が `true` である必要があります。


<a id="environments" href="#environments" class="field">`environments`</a> <span class="type">Map</span>  
environments セクションは Environment の設定を Manifest で指定した値によって上書きできるようにします。
上記の例の Manifest では、 CPU のパラメータを上書きしているので production のコンテナはよりパフォーマンスが高くなります。