以下は `'Scheduled Job'` Manifest で利用できるすべてのプロパティのリストです。[Job の概念](../concepts/jobs.ja.md)説明のページも合わせてご覧ください。

???+ note "レポートを作成する cron ジョブのサンプル Manifest"

```yaml
# Your job name will be used in naming your resources like log groups, ECS Tasks, etc.
name: report-generator
type: Scheduled Job

on:
  schedule: "@daily"
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

# You can override any of the values defined above by environment.
environments:
  prod:
    cpu: 2048               # Larger CPU value for prod environment
    memory: 4096
```

<a id="name" href="#name" class="field">`name`</a> <span class="type">String</span>  
Job 名。

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
Dockerfile からコンテナイメージをビルドする代わりに、既存のコンテナイメージ名の指定も可能です。`image.location` と [`image.build`](#image-build) の同時利用はできません。
`location` フィールドの制約を含む指定方法は Amazon ECS タスク定義の [`image` パラメータ](https://docs.aws.amazon.com/ja_jp/AmazonECS/latest/developerguide/task_definition_parameters.html#container_definition_image)のそれに従います。

<span class="parent-field">image.</span><a id="image-labels" href="#image-labels" class="field">`labels`</a><span class="type">Map</span>  
コンテナに付与したい [Docker ラベル](https://docs.docker.com/config/labels-custom-metadata/)を key/value の Map で指定できます。これは任意設定項目です。

<span class="parent-field">image.</span><a id="image-depends-on" href="#image-depends-on" class="field">`depends_on`</a> <span class="type">Map</span>  
任意項目。コンテナに追加する [Container Dependencies](https://docs.aws.amazon.com/ja_jp/AmazonECS/latest/APIReference/API_ContainerDependency.html) の任意の key/value の Map。Map の key はコンテナ名で、value は依存関係を表す値 (依存条件) として `start`、`healthy`、`complete`、`success` のいずれかを指定できます。なお、必須コンテナに `complete` や `success` の依存条件を指定することはできません。

!!! note
    サイドカーのコンテナヘルスチェックは、現在 Copilot ではサポートされていません。つまり、`healthy` はサイドカーの有効な依存条件ではありません。

設定例:
```yaml
image:
  build: ./Dockerfile
  depends_on:
    nginx: start
    startup: success
```
上記の例では、タスクのメインコンテナは `nginx` サイドカーが起動し、`startup` コンテナが正常に完了してから起動します。

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
`docker build --platform` で渡すオペレーティングシステムとアーキテクチャ。（`[os]/[arch]` の形式で指定）

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

<div class="separator"></div>

<a id="logging" href="#logging" class="field">`logging`</a> <span class="type">Map</span>  
logging セクションには、コンテナの [FireLens](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/using_firelens.html) ログドライバ用のログ設定パラメータが含まれます。(設定例は[こちら](../developing/sidecars.ja.md#sidecar-patterns))

<span class="parent-field">logging.</span><a id="logging-image" href="#logging-image" class="field">`image`</a> <span class="type">Map</span>  
任意項目。使用する Fluent Bit のイメージ。デフォルト値は `amazon/aws-for-fluent-bit:latest`。

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
