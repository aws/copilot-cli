`'Backend Service'` の Manifest で利用できるプロパティの一覧です。

<!-- textlint-disable ja-technical-writing/no-exclamation-question-mark, ja-technical-writing/ja-no-mixed-period -->
???+ note "\"api service\" の Manifest サンプル"
<!-- textlint-enable ja-technical-writing/no-exclamation-question-mark, ja-technical-writing/ja-no-mixed-period -->

    ```yaml
    # Service 名はロググループや ECS サービスなどのリソースの命名に利用されます。
    name: api
    type: Backend Service

    # この 'Backend Service' は "http://api.${COPILOT_SERVICE_DISCOVERY_ENDPOINT}:8080" でアクセスできますが、パブリックには公開されません。

    # コンテナと Service 用の設定
    image:
      build: ./api/Dockerfile
      port: 8080
      healthcheck:
        command: ["CMD-SHELL", "curl -f http://localhost:8080 || exit 1"]
        interval: 10s
        retries: 2
        timeout: 5s
        start_period: 0s

    cpu: 256
    memory: 512
    count: 1
    exec: true

    storage:
      volumes:
        myEFSVolume:
          path: '/etc/mount1'
          read_only: true
          efs:
            id: fs-12345678
            root_dir: '/'
            auth:
              iam: true
              access_point_id: fsap-12345678

    network:
      vpc:
        placement: 'private'
        security_groups: ['sg-05d7cd12cceeb9a6e']

    variables:
      LOG_LEVEL: info
    secrets:
      GITHUB_TOKEN: GITHUB_TOKEN

    # 上記すべての値は Environment ごとにオーバーライド可能です。
    environments:
      production:
        count: 2
    ```

<a id="name" href="#name" class="field">`name`</a> <span class="type">String</span>  
Service 名。

<div class="separator"></div>

<a id="type" href="#type" class="field">`type`</a> <span class="type">String</span>  
Service のアーキテクチャ。[Backend Services](../concepts/services.md#backend-service) はインターネット側からはアクセスできませんが、[サービス検出](../developing/service-discovery.md) の利用により他の Service からはアクセスできます。

<div class="separator"></div>

<a id="image" href="#image" class="field">`image`</a> <span class="type">Map</span>  
image セクションには、Docker ビルドに関する設定や公開するポートについてのパラメータが含まれます。

<span class="parent-field">image.</span><a id="image-build" href="#image-build" class="field">`build`</a> <span class="type">String or Map</span>  
このフィールドに String（文字列）を指定した場合、Copilot はそれを Dockerfile の場所を示すパスと解釈します。その際、指定したパスのディレクトリ部が Docker のビルドコンテキストであると仮定します。以下は build フィールドに文字列を指定する例です。
```yaml
image:
  build: path/to/dockerfile
```
これにより、イメージビルドの際に次のようなコマンドが実行されることになります: `$ docker build --file path/to/dockerfile path/to`

build フィールドには Map も利用できます。
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
この例は、Copilot は Docker ビルドコンテキストに context フィールドの値が示すディレクトリを利用し、args 以下のキーバリューのペアをイメージビルド時の --build-args 引数として渡します。上記例と同等の docker build コマンドは次のようになります:  
`$ docker build --file path/to/dockerfile --target build-stage --cache-from image:tag --build-arg key=value context/dir`.

Copilot はあなたの意図を理解するために最善を尽くしますので、記述する情報の省略も可能です。例えば、`context` は指定しているが `dockerfile` は未指定の場合、Copilot は Dockerfile が "Dockerfile" という名前で存在すると仮定しつつ、docker コマンドを `context` ディレクトリ以下で実行します。逆に `dockerfile` は指定しているが `context` が未指定の場合は、Copilot はあなたが `dockerfile` で指定されたディレクトリをビルドコンテキストディレクトリとして利用したいのだと仮定します。

すべてのパスはワークスペースのルートディレクトリからの相対パスと解釈されます。

<span class="parent-field">image.</span><a id="image-location" href="#image-location" class="field">`location`</a> <span class="type">String</span>  
Dockerfile からコンテナイメージをビルドする代わりに、既存のコンテナイメージ名の指定も可能です。`image.location` と [`image.build`](#image-build) の同時利用はできません。
`location` フィールドの制約を含む指定方法は Amazon ECS タスク定義の [`image` パラメータ](https://docs.aws.amazon.com/ja_jp/AmazonECS/latest/developerguide/task_definition_parameters.html#container_definition_image)のそれに従います。

<span class="parent-field">image.</span><a id="image-port" href="#image-port" class="field">`port`</a> <span class="type">Integer</span>  
公開するポート番号。Dockerfile 内に `EXPOSE` インストラクションが記述されている場合、Copilot はそれをパースした値をここに挿入します。  
作成する Backend Service が他の Service からのリクエストを受け付ける必要がない場合は、このフィールドは省略できます。

<span class="parent-field">image.</span><a id="image-healthcheck" href="#image-healthcheck" class="field">`healthcheck`</a> <span class="type">Map</span>  
コンテナヘルスチェックの設定。この設定はオプションです。

<span class="parent-field">image.healthcheck.</span><a id="image-healthcheck-cmd" href="#image-healthcheck-cmd" class="field">`command`</a> <span class="type">Array of Strings</span>  
コンテナが healthy であると判断するためのコマンド。  
このフィールドに設定する文字列配列の最初のアイテムには、コマンド引数を直接実行するための `CMD`、あるいはコンテナのデフォルトシェルでコマンドを実行する `CMD-SHELL` が利用できます。

<span class="parent-field">image.healthcheck.</span><a id="image-healthcheck-interval" href="#image-healthcheck-interval" class="field">`interval`</a> <span class="type">Duration</span>  
各ヘルスチェックの実行間の秒単位の間隔です。デフォルト値は１０秒です。

<span class="parent-field">image.healthcheck.</span><a id="image-healthcheck-retries" href="#image-healthcheck-retries" class="field">`retries`</a> <span class="type">Integer</span>  
コンテナが unhealthy と見なされるまでに、失敗したヘルスチェックを再試行する回数です。1〜10 回を指定できます。デフォルト値は２です。

<span class="parent-field">image.healthcheck.</span><a id="image-healthcheck-timeout" href="#image-healthcheck-timeout" class="field">`timeout`</a> <span class="type">Duration</span>  
ヘルスチェックの実行開始から失敗とみなすまでに待機する秒単位の期間です。デフォルト値は５秒です。

<span class="parent-field">image.healthcheck.</span><a id="image-healthcheck-start-period" href="#image-healthcheck-start-period" class="field">`start_period`</a> <span class="type">Duration</span>  
ヘルスチェックの実行と失敗がリトライ回数としてカウントされ始める前に、コンテナに対して起動処理を済ませる猶予期間を与えるための設定です。秒単位で指定し、デフォルト値は０秒です。

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

<a id="count" href="#count" class="field">`count`</a> <span class="type">Integer or Map</span>  
数値を指定する例。
```yaml
count: 5
```
Service が保つべきタスク数を５に指定します。

あるいは、Map を指定してオートスケーリングの設定も可能です。
```yaml
count:
  range: 1-10
  cpu_percentage: 70
  memory_percentage: 80
```

<span class="parent-field">count.</span><a id="count-range" href="#count-range" class="field">`range`</a> <span class="type">String</span>  
Service が保つべきタスク数の最小値と最大値の範囲を指定します。

<span class="parent-field">count.</span><a id="count-cpu-percentage" href="#count-cpu-percentage" class="field">`cpu_percentage`</a> <span class="type">Integer</span>  
Serviec が保つべき平均 CPU 使用率を指定し、それによってスケールアップ・ダウンします。

<span class="parent-field">count.</span><a id="count-memory-percentage" href="#count-memory-percentage" class="field">`memory_percentage`</a> <span class="type">Integer</span>  
Serviec が保つべき平均メモリ使用率を指定し、それによってスケールアップ・ダウンします。

<div class="separator"></div>

<a id="exec" href="#exec" class="field">`exec`</a> <span class="type">Boolean</span>   
コンテナ内部でのインタラクティブなコマンド実行機能を有効化します。デフォルト値は `false` です。`$ copilot svc exec` コマンドの利用には、この値に `true` を指定しておく必要があります。本機能を有効化すると ECS サービスの Fargate プラットフォームのバージョンが 1.4.0 へと更新される点にご留意ください。

<a id="network" href="#network" class="field">`network`</a> <span class="type">Map</span>    
`network` セクションは VPC 内の AWS リソースに接続するための設定です。

<span class="parent-field">network.</span><a id="network-vpc" href="#network-vpc" class="field">`vpc`</a> <span class="type">Map</span>  
タスクを配置するサブネットとアタッチされるセキュリティグループの設定です。

<span class="parent-field">network.vpc.</span><a id="network-vpc-placement" href="#network-vpc-placement" class="field">`placement`</a> <span class="type">String</span>  
`public` あるいは `private` のどちらかを指定します。デフォルトではタスクはパブリックサブネットに配置されます。

<!-- textlint-disable ja-technical-writing/no-exclamation-question-mark -->
!!! info inline end
    Copilot が生成した VPC を利用して `private` サブネットにタスクを配置する場合、Copilot は Environment に NAT ゲートウェイを作成します。既存の VPC を `copilot env init` コマンドでインポートする場合、タスクからのインターネット接続を確保できるよう、その VPC 内に NAT ゲートウェイが作成済みであることを確認してください。
<!-- textlint-enable ja-technical-writing/no-exclamation-question-mark -->

<span class="parent-field">network.vpc.</span><a id="network-vpc-security-groups" href="#network-vpc-security-groups" class="field">`security_groups`</a> <span class="type">Array of Strings</span>  
Copilot は、Service が Environment 内の他の Service と通信できるようにするためのセキュリティグループを常にタスクに対して設定します。本フィールドでは、同セキュリティグループ以外に追加で紐づけたい１つ以上のセキュリティグループ ID を指定します。

<div class="separator"></div>

<a id="variables" href="#variables" class="field">`variables`</a> <span class="type">Map</span>   
Copilot は Service 名などを常に環境変数としてタスクに対して渡します。本フィールドではそれら以外に追加で渡したい環境変数をキーバーリューのペアで指定します。

<div class="separator"></div>

<a id="secrets" href="#secrets" class="field">`secrets`</a> <span class="type">Map</span>   
[AWS Systems Manager (SSM) パラメータストア](https://docs.aws.amazon.com/ja_jp/systems-manager/latest/userguide/systems-manager-parameter-store.html)から取得する秘密情報を、キーに環境変数名、バリューに SSM パラメータ名をペアで指定します。秘密情報はタスク実行時に安全に取得され、環境変数として設定されます。

<div class="separator"></div>

<a id="storage" href="#storage" class="field">`storage`</a> <span class="type">Map</span>  
`storage` セクションでは、コンテナやサイドカーでマウントしたい EFS ボリュームを指定できます。これにより、リージョン間にまたがって永続化ストレージへのアクセスが必要となるデータ処理や CMS のようなワークロードの実行が可能となります。詳細は[ストレージ](../developing/storage.md)ページもご覧ください。

<span class="parent-field">storage.</span><a id="volumes" href="#volumes" class="field">`volumes`</a> <span class="type">Map</span>  
マウントしたい EFS ボリュームの名前や設定を指定します。`volumes` フィールドでは次のように Map を利用して指定します。
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
必須設定項目です。ボリュームをマウントするコンテナ内のパスを指定します。指定する値は２４２文字未満かつ `a-zA-Z0-9.-_/` の文字種である必要があります。

<span class="parent-field">volume.</span><a id="read_only" href="#read-only" class="field">`read_only`</a> <span class="type">Bool</span>  
任意設定項目で、デフォルト値は `true` です。ボリュームを読み取り専用とするかどうかを指定します。`false` に設定した場合、コンテナにファイルシステムへの `elasticfilesystem:ClientWrite` 権限が付与され、それによりボリュームへ書き込めるようになります。

<span class="parent-field">volume.</span><a id="efs" href="#efs" class="field">`efs`</a> <span class="type">Map</span>  
詳細な EFS 設定を指定します。

<span class="parent-field">volume.efs.</span><a id="id" href="#id" class="field">`id`</a> <span class="type">String</span>  
必須設定項目です。マウントする EFS ファイルシステムの ID を指定します。

<span class="parent-field">volume.efs.</span><a id="root_dir" href="#root-dir" class="field">`root_dir`</a> <span class="type">String</span>  
任意設定項目で、デフォルト値は `/` です。EFS ファイルシステム内のどのパスをマウントするボリュームのルートとするのかを指定します。指定する値は<!-- textlint-disable ja-technical-writing/ja-no-successive-word -->２５５<!-- textlint-enable ja-technical-writing/ja-no-successive-word -->文字未満かつ `a-zA-Z0-9.-_/` の文字種である必要があります。EFS アクセスポイントを利用する場合、本設定値に空もしくは `/` を指定し、かつ `auth.iam` の設定値が `true` となっている必要があります。

<span class="parent-field">volume.efs.</span><a id="auth" href="#auth" class="field">`auth`</a> <span class="type">Map</span>  
EFS に関連する認可設定を指定します。

<span class="parent-field">volume.efs.auth.</span><a id="iam" href="#iam" class="field">`iam`</a> <span class="type">Bool</span>  
任意設定項目で、デフォルトは `true` です。EFS リソースへのアクセスに IAM による認可を利用するかどうかを指定します。

<span class="parent-field">volume.efs.auth.</span><a id="access_point_id" href="#access-point-id" class="field">`access_point_id`</a> <span class="type">String</span>  
任意設定項目で、デフォルトは `""` です。利用する EFS アクセスポイントの ID を指定します。EFS アクセスポイントを利用する場合、`root_dir` の設定値に空もしくは `/` を指定しており、かつ本設定値が `true` となっている必要があります。

<div class="separator"></div>

<a id="environments" href="#environments" class="field">`environments`</a> <span class="type">Map</span>  
`environment` セクションでは、Manifest 内の任意の設定値を Environment ごとにオーバーライドできます。上部記載の Manifest 例では `count` パラメータをオーバーライドすることで prod Environment で実行されるタスク数を２に設定しています。
