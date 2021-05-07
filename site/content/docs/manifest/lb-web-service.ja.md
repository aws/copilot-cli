以下は、 `'Load Balanced Web Service'` Manifest で使用可能なすべてのプロパティのリストです。

???+ note "frontend Service のサンプル Manifest"

    ```yaml
    # Service 名( name ) は、ロググループ、ECS サービスなどのリソースの命名に利用されます。
    name: frontend
    type: Load Balanced Web Service

    # Serviceのトラフィックを分散します。
    http:
      path: '/'
      healthcheck:
        path: '/_healthcheck'
        healthy_threshold: 3
        unhealthy_threshold: 2
        interval: 15s
        timeout: 10s
      stickiness: false
      allowed_source_ips: ["10.24.34.0/23"]

    # コンテナと Service の構成
    image:
      build:
        dockerfile: ./frontend/Dockerfile
        context: ./frontend
      port: 80

    cpu: 256
    memory: 512
    count:
      range: 1-10
      cpu_percentage: 70
      memory_percentage: 80
      requests: 10000
      response_time: 2s
    exec: true

    variables:
      LOG_LEVEL: info
    secrets:
      GITHUB_TOKEN: GITHUB_TOKEN

    # Enviroment 毎の定義によって、上記で定義された値のいずれかを上書きできます。
    environments:
      production:
        count: 2
    ```

<a id="name" href="#name" class="field">`name`</a> <span class="type">String</span>  
Service の名前。

<div class="separator"></div>

<a id="type" href="#type" class="field">`type`</a> <span class="type">String</span>  
Service のアーキテクチャタイプ。 [Load Balanced Web Service](../concepts/services.md#load-balanced-web-service) は、ロードバランサー及び AWS Fargate 上の Amazon ECS によって構成される、インターネットに公開するための Service です。

<div class="separator"></div>

<a id="http" href="#http" class="field">`http`</a> <span class="type">Map</span>   
http セクションには、Service と Application Load Balancer の統合に関連するパラメーターが含まれています。

<span class="parent-field">http.</span><a id="http-path" href="#http-path" class="field">`path`</a> <span class="type">String</span>  
このパスに対するリクエストは Service に転送されます。各 [Load Balanced Web Service](../concepts/services.md#load-balanced-web-service) は、一意の path でリッスンする必要があります。

<span class="parent-field">http.</span><a id="http-healthcheck" href="#http-healthcheck" class="field">`healthcheck`</a> <span class="type">String or Map</span>  
文字列が指定された場合、Copilot は、ターゲットグループのヘルスチェックリクエストを処理するためにコンテナが公開しているパスとして解釈します。デフォルトは "/" です。
```yaml
http:
  healthcheck: '/'
```
以下のようにヘルスチェックの詳細を指定できます。:
```yaml
http:
  healthcheck:
    path: '/'
    healthy_threshold: 3
    unhealthy_threshold: 2
    interval: 15s
    timeout: 10s

<span class="parent-field">http.healthcheck.</span><a id="http-healthcheck-healthy-threshold" href="#http-healthcheck-healthy-threshold" class="field">`healthy_threshold`</a> <span class="type">Integer</span>  
Unhealthy なターゲットを healthy とみなすために必要な、連続したヘルスチェックの成功回数。Copilot のデフォルトは 2 です。設定可能な範囲は、2〜10です。

<span class="parent-field">http.healthcheck.</span><a id="http-healthcheck-unhealthy-threshold" href="#http-healthcheck-unhealthy-threshold" class="field">`unhealthy_threshold`</a> <span class="type">Integer</span>  
ターゲットが Unhealthy であると判断するまでに必要な、連続したヘルスチェックの失敗回数。Copilot のデフォルトは 2 です。設定可能な範囲は、2〜10です。

<span class="parent-field">http.healthcheck.</span><a id="http-healthcheck-interval" href="#http-healthcheck-interval" class="field">`interval`</a> <span class="type">Duration</span>  
個々のターゲットのヘルスチェックを行う際の、おおよその時間を秒単位で指定します。Copilot のデフォルトは 10秒 です。設定可能な範囲は、5秒〜300秒です。

<span class="parent-field">http.healthcheck.</span><a id="http-healthcheck-timeout" href="#http-healthcheck-timeout" class="field">`timeout`</a> <span class="type">Duration</span>  
ターゲットからの応答がない場合、ヘルスチェックが失敗したことを意味する時間を秒単位で指定します。Copilot のデフォルトは 5秒 です。設定可能な範囲は、5秒〜300秒です。

<span class="parent-field">http.</span><a id="http-target-container" href="#http-target-container" class="field">`target_container`</a> <span class="type">String</span>  
Service コンテナの代わりにロードバランサのターゲットに指定する Sidecar コンテナ。

<span class="parent-field">http.</span><a id="http-stickiness" href="#http-stickiness" class="field">`stickiness`</a> <span class="type">Boolean</span>  
sticky sessions の有効/無効。

<span class="parent-field">http.</span><a id="http-allowed-source-ips" href="#http-allowed-source-ips" class="field">`allowed_source_ips`</a> <span class="type">Array of Strings</span>  
Service へのアクセスが許可されているCIDR IP アドレスのリスト。
```yaml
http:
  allowed_source_ips: ["192.0.2.0/24", "198.51.100.10/32"]
```

<div class="separator"></div>

<a id="image" href="#image" class="field">`image`</a> <span class="type">Map</span>  
image セクションには、Docker のビルド構成や公開するポートに関するパラメータが含まれています。

<span class="parent-field">image.</span><a id="image-build" href="#image-build" class="field">`build`</a> <span class="type">String or Map</span>  
文字列を指定した場合、Copilot は Dockerfile のパスと解釈します。指定された文字列のディレクトリ名がビルド時のコンテキストになると仮定します。
Manifest はこのようになります。
```yaml
image:
  build: path/to/dockerfile
```

実行すると、次のように docker build コマンドが呼び出されます。: `$ docker build --file path/to/dockerfile path/to`

以下のように build の詳細を指定できます。:
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
この場合、Copilot は指定されたコンテキストディレクトリを使用し、 `args` 以下のキーと値のペアを用いて、 `--build-arg` 引数を上書きするように変換します。これに相当する `docker build` コマンドは次のようになります。 
`$ docker build --file path/to/dockerfile --target build-stage --cache-from image:tag --build-arg key=value context/dir`.

フィールドを省略した場合でも、 Copilot はその意図をそれを最大限補完して解釈をします。例えば、 `context` を指定して `dockerfile` を指定しない場合、Copilotはコンテキストディレクトリで Docker を実行し、 Dockerfile の名前は、"Dockerfile" であると仮定します。  `dockerfile` を指定して、 `context` を指定しない場合、 Copilot は `dockerfile` を含むディレクトリで Docker の実行を試みます。

すべてのパスは、作業ディレクトリ からの相対パスです。

<span class="parent-field">image.</span><a id="image-location" href="#image-location" class="field">`location`</a> <span class="type">String</span>  
Dockerfile からコンテナをビルドする代わりに、既存のイメージ名の指定が可能です。これは、[`image.build`](#image-build)とは相互排他的な項目です。
`location` フィールドは、Amazon ECS タスク定義の [`image` parameter](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/task_definition_parameters.html#container_definition_image) と同じ定義に従います。

<span class="parent-field">image.</span><a id="image-port" href="#image-port" class="field">`port`</a> <span class="type">Integer</span>  
Dockerfile で公開されるポートです。 Copilot は、 `EXPOSE` 命令からこの値を解析します。

<div class="separator"></div>

<a id="entrypoint" href="#entrypoint" class="field">`entrypoint`</a> <span class="type">String or Array of Strings</span>  
イメージのデフォルトのエントリポイントを上書きします。
```yaml
# String version.
entrypoint: "/bin/entrypoint --p1 --p2"
# Alteratively, as an array of strings.
entrypoint: ["/bin/entrypoint", "--p1", "--p2"]
```

<div class="separator"></div>

<a id="command" href="#command" class="field">`command`</a> <span class="type">String or Array of Strings</span>  
イメージのデフォルトコマンドを上書きします。

```yaml
# String version.
command: ps au
# Alteratively, as an array of strings.
command: ["ps", "au"]
```

<div class="separator"></div>

<a id="cpu" href="#cpu" class="field">`cpu`</a> <span class="type">Integer</span>  
タスクの CPU ユニットの数を指定します。有効な CPU ユニット数の値については [Amazon ECS docs](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/task-cpu-memory-error.html) を参照してください。

<div class="separator"></div>

<a id="memory" href="#memory" class="field">`memory`</a> <span class="type">Integer</span>  
タスクが使用する MiB 単位のメモリ量を指定します。有効なメモリの値については、[Amazon ECS docs](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/task-cpu-memory-error.html)を参照してください。

<div class="separator"></div>

<a id="count" href="#count" class="field">`count`</a> <span class="type">Integer or Map</span>  
数字を指定する場合、以下のように指定します。
```yaml
count: 5
```
ECS サービスは、希望するカウントの値を 5 に設定し、Service 内の 5 つのタスクを維持するようにします。

以下のようにオートスケーリングの詳細を指定できます。

```yaml
count:
  range: 1-10
  cpu_percentage: 70
  memory_percentage: 80
  requests: 10000
  response_time: 2s
```

<span class="parent-field">count.</span><a id="count-range" href="#count-range" class="field">`range`</a> <span class="type">String</span>  
ECS サービスが維持すべきタスクの数の最小値と最大値を指定します。

<span class="parent-field">count.</span><a id="count-cpu-percentage" href="#count-cpu-percentage" class="field">`cpu_percentage`</a> <span class="type">Integer</span>  
ECS サービスが維持すべき平均 CPU に基づいて、スケールアップまたはスケールダウンします。

<span class="parent-field">count.</span><a id="count-memory-percentage" href="#count-memory-percentage" class="field">`memory_percentage`</a> <span class="type">Integer</span>  
ECS サービスが維持すべき平均的なメモリに基づいて、スケールアップまたはスケールダウンします。

<span class="parent-field">count.</span><a id="requests" href="#count-requests" class="field">`requests`</a> <span class="type">Integer</span>  
ECS タスクごとに処理するリクエスト数に基づいて、スケールアップまたスケールダウンします。

<span class="parent-field">count.</span><a id="response-time" href="#count-response-time" class="field">`response_time`</a> <span class="type">Duration</span>  
ECS サービスの平均レスポンス時間に基づいて、スケールアップまたはスケールダウンします。

<div class="separator"></div>

<a id="exec" href="#exec" class="field">`exec`</a> <span class="type">Boolean</span>   
コンテナ内でのコマンド実行を有効にします。デフォルトは  `false` です。 `$ copilot svc exec` に必要です。これにより、サービスの Fargate Platform Version が 1.4.0 に更新されることに注意してください。

<a id="network" href="#network" class="field">`network`</a> <span class="type">Map</span>    
` `network` セクションには、VPC 内の AWS リソースに接続するためのパラメータが含まれています。

<span class="parent-field">network.</span><a id="network-vpc" href="#network-vpc" class="field">`vpc`</a> <span class="type">Map</span>  
タスクにアタッチされたサブネットとセキュリティグループ。

<span class="parent-field">network.vpc.</span><a id="network-vpc-placement" href="#network-vpc-placement" class="field">`placement`</a> <span class="type">String</span>  
 `'public'` または  `'private'` のいずれかを指定する必要があります。デフォルトでは、パブリックサブネットでタスクを起動します。

!!! info inline end
    `private` サブネットでタスクを起動し、Copilot で生成された VPC を使用する場合、Copilot は Enviroment に NAT Gateway を追加します。また、インターネット接続のために、`copilot env init`を実行する際に、NAT Gateway を含む VPC をインポートできます。

<span class="parent-field">network.vpc.</span><a id="network-vpc-security-groups" href="#network-vpc-security-groups" class="field">`security_groups`</a> <span class="type">Array of Strings</span>  
タスクに関連付けられた追加のセキュリティグループ ID です。 Copilot は、常にセキュリティグループを利用しているため、Enviroment 内のコンテナは相互に通信できます。

<div class="separator"></div>

<a id="variables" href="#variables" class="field">`variables`</a> <span class="type">Map</span>   
サービスに渡される環境変数を表すキーと値のペアです。Copilot は、デフォルトでいくつかの環境変数を含んでいます。

<div class="separator"></div>

<a id="secrets" href="#secrets" class="field">`secrets`</a> <span class="type">Map</span>   
環境変数として [AWS Systems Manager Parameter Store](https://docs.aws.amazon.com/systems-manager/latest/userguide/systems-manager-parameter-store.html) から Service に対して渡される秘密情報を表すキーと値のペアです。

<div class="separator"></div>

<a id="storage" href="#storage" class="field">`storage`</a> <span class="type">Map</span>  
Storage セクションでは、コンテナやサイドカーがマウントする外部 EFS ボリュームの指定が可能です。これにより、データ処理や CMS のワークロードのために、リージョンをまたいで永続的なストレージにアクセスが可能です。詳細については、[storage](../developing/storage.md)ページを参照してください。

<span class="parent-field">storage.</span><a id="volumes" href="#volumes" class="field">`volumes`</a> <span class="type">Map</span>  
アタッチしたい EFS ボリュームの名前と構成を指定します。`volumes` フィールドは、マップとして指定します。

```yaml
volumes:
  {{ volume name }}:
    path: "/etc/mountpath"
    efs:
      ...
```

<span class="parent-field">storage.volumes.</span><a id="volume" href="#volume" class="field">`volume`</a> <span class="type">Map</span>  
ボリュームの構成を指定します。

<span class="parent-field">volume.</span><a id="path" href="#path" class="field">`path`</a> <span class="type">String</span>  
必須項目です。コンテナ内でボリュームをマウントする場所を指定します。242 文字以下で、文字 ``a-zA-Z0-9.-_/` のみで構成されている必要があります。

<span class="parent-field">volume.</span><a id="read_only" href="#read-only" class="field">`read_only`</a> <span class="type">Bool</span>  
オプション項目です。デフォルトは  `true` です。ボリュームを読み取り専用にするかどうかを指定します。false の場合、コンテナにはファイルシステムに対する `elasticfilesystem:ClientWrite` パーミッションが付与され、ボリュームは書き込み可能になります。

<span class="parent-field">volume.</span><a id="efs" href="#efs" class="field">`efs`</a> <span class="type">Map</span>  
より詳細な EFS の設定を指定します。

<span class="parent-field">volume.efs.</span><a id="id" href="#id" class="field">`id`</a> <span class="type">String</span>  
必須項目です。マウントしたいファイルシステムの ID を指定します。

<span class="parent-field">volume.efs.</span><a id="root_dir" href="#root-dir" class="field">`root_dir`</a> <span class="type">String</span>  
オプション項目です。デフォルトは `/` です。EFS ファイルシステムの中で、ボリュームのルートとして使用する場所を指定します。255 文字以下で、 `a-zA-Z0-9.-_/` の文字のみで構成されている必要があります。アクセスポイントを使用する場合は、`root_dir` には空または `/` を、`auth.iam` には `true` を指定してください。

<span class="parent-field">volume.efs.</span><a id="auth" href="#auth" class="field">`auth`</a> <span class="type">Map</span>  
EFS の高度な認証設定を指定します。

<span class="parent-field">volume.efs.auth.</span><a id="iam" href="#iam" class="field">`iam`</a> <span class="type">Bool</span>  
オプション項目です。デフォルトは `true` です。ボリュームが EFS への接続を許可されているかどうかを判断するのに、IAM 認証を使用するかどうかを指定します。

<span class="parent-field">volume.efs.auth.</span><a id="access_point_id" href="#access-point-id" class="field">`access_point_id`</a> <span class="type">String</span>  
オプション項目です。デフォルトは `""` です。接続する EFS アクセスポイントの ID です。アクセスポイントを使用する場合、 `root_dir` には空または  `/` を、 `auth.iam` には  `true` を指定します。

<div class="separator"></div>

<a id="environments" href="#environments" class="field">`environments`</a> <span class="type">Map</span>  
Enviroments セクションでは、使用している Enviroments に基づいて Manifest 内の任意の値を上書き可能です。上記の Manifest の例では、count パラメータをオーバーライドして、prod Enviroments で Service のコピーを 2 つ実行できるようにしています。