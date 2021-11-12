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

<a id="platform" href="#platform" class="field">`platform`</a> <span class="type">String or Map</span>  
`docker build --platform` で渡す、デフォルト以外のオペレーティングシステムとアーキテクチャ。（`[os]/[arch]` の形式で指定）

自動生成された文字列を上書きして、異なる `osfamily` や `architecture` でビルドをします。例えば、Windows ユーザーはデフォルトで `WINDOWS_SERVER_2019_CORE` を利用する1つ目の例を、2つ目の例のように Map を使って変更するかもしれません。

```yaml
platform: windows/amd64
```

```yaml
platform:
  osfamily: windows_server_2019_full
  architecture: x86_64
```

<div class="separator"></div>

<a id="count" href="#count" class="field">`count`</a> <span class="type">Integer or Map</span>  
数値を指定する例。

```yaml
count: 5
```

Service が保つべきタスク数を５に指定します。

<span class="parent-field">count.</span><a id="count-spot" href="#count-spot" class="field">`spot`</a> <span class="type">Integer</span>

`spot` サブフィールドに数値を指定することで、Service の実行に Fargate Spot キャパシティを利用できます。
```yaml
count:
  spot: 5
```

<div class="separator"></div>

あるいは、Map を指定してオートスケーリングの設定も可能です。
```yaml
count:
  range: 1-10
  cpu_percentage: 70
  memory_percentage: 80
  requests: 10000
  response_time: 2s
```

<span class="parent-field">count.</span><a id="count-range" href="#count-range" class="field">`range`</a> <span class="type">String or Map</span>  
メトリクスに指定した値に基づいて、Service が保つべきタスク数の最小と最大を範囲指定できます。

```yaml
count:
  range: n-m
```

これにより Application Auto Scaling がセットアップされ、`MinCapacity` に `n` が、`MaxCapacity` に `m` が設定されます。

あるいは次の例に挙げるように `range` フィールド以下に `min` と `max` を指定し、加えて `spot_from` フィールドを利用することで、一定数以上のタスクを実行する場合に Fargate Spot キャパシティを利用する設定が可能です。

```yaml
count:
  range:
    min: 1
    max: 10
    spot_from: 3
```

上記の例では Application Auto Scaling は 1-10 の範囲で設定されますが、最初の２タスクはオンデマンド Fargate キャパシティに配置されます。Service が３つ以上のタスクを実行するようにスケールした場合、３つ目以降のタスクは Fargate Spot に配置されます。

<span class="parent-field">range.</span><a id="count-range-min" href="#count-range-min" class="field">`min`</a> <span class="type">Integer</span>  
Service がオートスケーリングを利用する場合の最小タスク数。
 
<span class="parent-field">range.</span><a id="count-range-max" href="#count-range-max" class="field">`max`</a> <span class="type">Integer</span>  
Service がオートスケーリングを利用する場合の最大タスク数。

<span class="parent-field">range.</span><a id="count-range-spot-from" href="#count-range-spot-from" class="field">`spot_from`</a> <span class="type">Integer</span>  
Service の何個目のタスクから Fargate Spot キャパシティを利用するか。

<span class="parent-field">count.</span><a id="count-cpu-percentage" href="#count-cpu-percentage" class="field">`cpu_percentage`</a> <span class="type">Integer</span>  
Service が保つべき平均 CPU 使用率を指定し、それによってスケールアップ・ダウンします。

<span class="parent-field">count.</span><a id="count-memory-percentage" href="#count-memory-percentage" class="field">`memory_percentage`</a> <span class="type">Integer</span>  
Service が保つべき平均メモリ使用率を指定し、それによってスケールアップ・ダウンします。

<span class="parent-field">count.</span><a id="requests" href="#count-requests" class="field">`requests`</a> <span class="type">Integer</span>  
タスク当たりのリクエスト数を指定し、それによってスケールアップ・ダウンします。

<span class="parent-field">count.</span><a id="response-time" href="#count-response-time" class="field">`response_time`</a> <span class="type">Duration</span>  
Service の平均レスポンスタイムを指定し、それによってスケールアップ・ダウンします。

<div class="separator"></div>

<a id="exec" href="#exec" class="field">`exec`</a> <span class="type">Boolean</span>   
コンテナ内部でのインタラクティブなコマンド実行機能を有効化します。デフォルト値は `false` です。`$ copilot svc exec` コマンドの利用には、この値に `true` を指定しておく必要があります。

<div class="separator"></div>

<a id="network" href="#network" class="field">`network`</a> <span class="type">Map</span>    
`network` セクションは VPC 内の AWS リソースに接続するための設定です。

<span class="parent-field">network.</span><a id="network-vpc" href="#network-vpc" class="field">`vpc`</a> <span class="type">Map</span>  
タスクを配置するサブネットとアタッチされるセキュリティグループの設定です。

<span class="parent-field">network.vpc.</span><a id="network-vpc-placement" href="#network-vpc-placement" class="field">`placement`</a> <span class="type">String</span>  
`public` あるいは `private` のどちらかを指定します。デフォルトではタスクはパブリックサブネットに配置されます。

<!-- textlint-disable ja-technical-writing/no-exclamation-question-mark -->
!!! info
    Copilot が生成した VPC を利用して `private` サブネットにタスクを配置する場合、Copilot は Environment に NAT ゲートウェイを作成します。既存の VPC を `copilot env init` コマンドでインポートする場合、タスクからのインターネット接続を確保できるよう、その VPC 内に NAT ゲートウェイが作成済みであることを確認してください。
<!-- textlint-enable ja-technical-writing/no-exclamation-question-mark -->

<span class="parent-field">network.vpc.</span><a id="network-vpc-security-groups" href="#network-vpc-security-groups" class="field">`security_groups`</a> <span class="type">Array of Strings</span>  
Copilot がタスクに対して自動で設定するセキュリティグループ以外に追加で設定したいセキュリティグループがある場合にそれらの ID を指定します。複数のセキュリティグループ ID を指定可能です。(Copilot が自動設定するセキュリティグループは、同一 Environment 内の Service 間通信を可能にする目的で設定されます。)

<div class="separator"></div>

<a id="variables" href="#variables" class="field">`variables`</a> <span class="type">Map</span>   
Copilot は Service 名などを常に環境変数としてタスクに対して渡します。本フィールドではそれら以外に追加で渡したい環境変数をキーバーリューのペアで指定します。

<div class="separator"></div>

<a id="secrets" href="#secrets" class="field">`secrets`</a> <span class="type">Map</span>   
[AWS Systems Manager (SSM) パラメータストア](https://docs.aws.amazon.com/ja_jp/systems-manager/latest/userguide/systems-manager-parameter-store.html)から取得する秘密情報を、キーに環境変数名、バリューに SSM パラメータ名をペアで指定します。秘密情報はタスク実行時に安全に取得され、環境変数として設定されます。

<div class="separator"></div>

<a id="storage" href="#storage" class="field">`storage`</a> <span class="type">Map</span>  
`storage` セクションでは、コンテナやサイドカーでマウントしたい EFS ボリュームを指定できます。これにより、リージョン内のアベイラビリティゾーンにまたがって永続化ストレージへのアクセスが必要となるデータ処理や CMS のようなワークロードの実行が可能となります。詳細は[ストレージ](../developing/storage.ja.md)ページもご覧ください。また、タスクレベルのエフェメラルストレージの拡張を設定もできます。

<span class="parent-field">storage.</span><a id="ephemeral" href="#ephemeral" class="field">`ephemeral`</a> <span class="type">Int</span>
タスクに割り当てたいエフェメラルストレージのサイズを GiB で指定します。デフォルトかつ最小値は 20 GiB で、最大値は 200 GiB です。20 GiB を超えるサイズを指定した場合、サイズに応じた追加の料金が発生します。

タスクのメインコンテナとサイドカーでファイルシステムを共有したい場合、例えば次のように空ボリュームを使う方法が検討できます。
```yaml
storage:
  ephemeral: 100
  volumes:
    scratch:
      path: /var/data
      read_only: false

sidecars:
  mySidecar:
    image: public.ecr.aws/my-image:latest
    mount_points:
      - source_volume: scratch
        path: /var/data
        read_only: false
```
この例ではサイドカーとメインコンテナで共有されるボリュームとして、100 GiB のストレージがプロビジョンされます。例えば大きなサイズのデータセットを利用したい場合、あるいはディスク I/O の要求が高いワークロードにおいてサイドカーを利用して EFS からデータをコピーするような場合に有効な方法と言えます。

<span class="parent-field">storage.</span><a id="volumes" href="#volumes" class="field">`volumes`</a> <span class="type">Map</span>  
マウントしたい EFS ボリュームの名前や設定を指定します。`volumes` フィールドでは次のように Map を利用して指定します。
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
必須設定項目です。ボリュームをマウントするコンテナ内のパスを指定します。指定する値は２４２文字未満かつ `a-zA-Z0-9.-_/` の文字種である必要があります。

<span class="parent-field">volume.</span><a id="read_only" href="#read-only" class="field">`read_only`</a> <span class="type">Boolean</span>  
任意設定項目で、デフォルト値は `true` です。ボリュームを読み取り専用とするかどうかを指定します。`false` に設定した場合、コンテナにファイルシステムへの `elasticfilesystem:ClientWrite` 権限が付与され、それによりボリュームへ書き込めるようになります。

<span class="parent-field">volume.</span><a id="efs" href="#efs" class="field">`efs`</a> <span class="type">Boolean or Map</span>  
詳細な EFS 設定を指定します。Boolean 値による指定、あるいは `uid` と `gid` サブフィールドのみを指定した場合に、EFS ファイルシステムと Service 専用の EFS アクセスポイントが作成されます。

```yaml
// Boolean 値を指定する場合
efs: true

// POSIX uid/gid を指定する場合
efs:
  uid: 10000
  gid: 110000
```

<span class="parent-field">volume.efs.</span><a id="id" href="#id" class="field">`id`</a> <span class="type">String</span>  
必須設定項目です。マウントする EFS ファイルシステムの ID を指定します。

<span class="parent-field">volume.efs.</span><a id="root_dir" href="#root-dir" class="field">`root_dir`</a> <span class="type">String</span>  
任意設定項目で、デフォルト値は `/` です。EFS ファイルシステム内のどのパスをマウントするボリュームのルートとするのかを指定します。指定する値は 255 文字未満かつ `a-zA-Z0-9.-_/` の文字種である必要があります。EFS アクセスポイントを利用する場合、本設定値に空もしくは `/` を指定し、かつ `auth.iam` の設定値が `true` となっている必要があります。

<span class="parent-field">volume.efs.</span><a id="uid" href="#uid" class="field">`uid`</a> <span class="type">Uint32</span>
任意設定項目で、`gid` とともに指定する必要があります。また、`root_dir`、`auth`、`id` とともに指定することはできません. Copilot 管理の EFS ファイルシステムに対する EFS アクセスポイントを作成する際の POSIX UID として利用されます。

<span class="parent-field">volume.efs.</span><a id="gid" href="#gid" class="field">`gid`</a> <span class="type">Uint32</span>
任意設定項目で、`uid` とともに指定する必要があります。また、`root_dir`、`auth`、`id` とともに指定することはできません. Copilot 管理の EFS ファイルシステムに対する EFS アクセスポイントを作成する際の POSIX GID として利用されます。

<span class="parent-field">volume.efs.</span><a id="auth" href="#auth" class="field">`auth`</a> <span class="type">Map</span>  
EFS に関連する認可設定を指定します。

<span class="parent-field">volume.efs.auth.</span><a id="iam" href="#iam" class="field">`iam`</a> <span class="type">Boolean</span>  
任意設定項目で、デフォルトは `true` です。EFS リソースへのアクセスに IAM による認可を利用するかどうかを指定します。

<span class="parent-field">volume.efs.auth.</span><a id="access_point_id" href="#access-point-id" class="field">`access_point_id`</a> <span class="type">String</span>  
任意設定項目で、デフォルトは `""` です。利用する EFS アクセスポイントの ID を指定します。EFS アクセスポイントを利用する場合、`root_dir` の設定値に空もしくは `/` を指定しており、かつ本設定値が `true` となっている必要があります。

<div class="separator"></div>

<a id="logging" href="#logging" class="field">`logging`</a> <span class="type">Map</span>  
logging セクションには、ログ設定を含みます。このセクションでは、コンテナの [FireLens](https://docs.aws.amazon.com/ja_jp/AmazonECS/latest/developerguide/using_firelens.html) ログドライバ用のパラメータを設定できます。(設定例は[こちら](../developing/sidecars.ja.md#sidecar-patterns))

<span class="parent-field">logging.</span><a id="retention" href="#logging-retention" class="field">`retention`</a> <span class="type">Integer</span>
任意項目。 ログイベントを保持する日数。設定可能な値については、[こちら](https://docs.aws.amazon.com/ja_jp/AWSCloudFormation/latest/UserGuide/aws-resource-logs-loggroup.html#cfn-logs-loggroup-retentionindays)を確認してください。省略した場合、デフォルトの 30 が設定されます。

<span class="parent-field">logging.</span><a id="logging-image" href="#logging-image" class="field">`image`</a> <span class="type">String</span>
任意項目。使用する Fluent Bit のイメージ。デフォルト値は `amazon/aws-for-fluent-bit:latest`。

<span class="parent-field">logging.</span><a id="logging-destination" href="#logging-destination" class="field">`destination`</a> <span class="type">Map</span>  
任意項目。FireLens ログドライバーにログを送信するときの設定。

<span class="parent-field">logging.</span><a id="logging-enableMetadata" href="#logging-enableMetadata" class="field">`enableMetadata`</a> <span class="type">Boolean</span>
任意項目。ログに ECS メタデータを含めるかどうか。デフォルトは `true`。

<span class="parent-field">logging.</span><a id="logging-secretOptions" href="#logging-secretOptions" class="field">`secretOptions`</a> <span class="type">Map</span>  
任意項目。ログの設定に渡す秘密情報です。

<span class="parent-field">logging.</span><a id="logging-configFilePath" href="#logging-configFilePath" class="field">`configFilePath`</a> <span class="type">String</span>
任意項目。カスタムの Fluent Bit イメージ内の設定ファイルのフルパス。

<div class="separator"></div>

<a id="taskdef_overrides" href="#taskdef_overrides" class="field">`taskdef_overrides`</a> <span class="type">Array of Rules</span>  
`taskdef_overrides` セクションでは、ECS のタスク定義のオーバーライドルールを適用できます (例は[こちら](../developing/taskdef-overrides.ja.md#examples))。

<span class="parent-field">taskdef_overrides.</span><a id="taskdef_overrides-path" href="#taskdef_overrides-path" class="field">`path`</a> <span class="type">String</span>
必須設定項目です。オーバーライドするタスク定義のフィールドのパス。

<span class="parent-field">taskdef_overrides.</span><a id="taskdef_overrides-value" href="#taskdef_overrides-value" class="field">`value`</a> <span class="type">Any</span>
必須設定項目です。オーバーライドするタスク定義のフィールドの値。

<div class="separator"></div>

<a id="environments" href="#environments" class="field">`environments`</a> <span class="type">Map</span>  
`environments` セクションでは、Manifest 内の任意の設定値を Environment ごとにオーバーライドできます。上部記載の Manifest 例では `count` パラメータをオーバーライドすることで 'prod' Environment で実行されるタスク数を ２ に設定し、'staging' Environment で実行される Fargate Spot capacity によるタスク数を ２ に設定します。
