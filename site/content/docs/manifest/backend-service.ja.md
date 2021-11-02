以下は `'Backend Service'` Manifest で利用できるすべてのプロパティのリストです。[Copilot Service の概念](../concepts/services.ja.md)説明のページも合わせてご覧ください。

<!-- textlint-disable ja-technical-writing/no-exclamation-question-mark, ja-technical-writing/ja-no-mixed-period -->
???+ note "api service のサンプル Manifest"
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
  test:
    count:
      spot: 2
  production:
    count: 2
```

<a id="name" href="#name" class="field">`name`</a> <span class="type">String</span>  
Service 名。

<div class="separator"></div>

<a id="type" href="#type" class="field">`type`</a> <span class="type">String</span>  
Service のアーキテクチャ。[Backend Services](../concepts/services.ja.md#backend-service) はインターネット側からはアクセスできませんが、[サービス検出](../developing/service-discovery.ja.md)の利用により他の Service からはアクセスできます。

{% include 'image-config-with-port.ja.md' %}

{% include 'image-healthcheck.ja.md' %}

{% include 'task-size.ja.md' %}

{% include 'platform.ja.md' %}

<div class="separator"></div>

<a id="count" href="#count" class="field">`count`</a> <span class="type">Integer or Map</span>
次の様に指定すると、
```yaml
count: 5
```
Service は、希望するタスク数を 5 に設定し、Service 内に 5 つのタスクが起動している様に保ちます。

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

上記の例では Application Auto Scaling は 1-10 の範囲で設定されますが、最初の２タスクはオンデマンド Fargate キャパシティに配置されます。Service が３つ以上のタスクを実行するようにスケールした場合、３つ目以降のタスクは最大タスク数に達するまで Fargate Spot に配置されます。

<span class="parent-field">range.</span><a id="count-range-min" href="#count-range-min" class="field">`min`</a> <span class="type">Integer</span>  
Service がオートスケーリングを利用する場合の最小タスク数。

<span class="parent-field">range.</span><a id="count-range-max" href="#count-range-max" class="field">`max`</a> <span class="type">Integer</span>  
Service がオートスケーリングを利用する場合の最大タスク数。

<span class="parent-field">range.</span><a id="count-range-spot-from" href="#count-range-spot-from" class="field">`spot_from`</a> <span class="type">Integer</span>  
Service の何個目のタスクから Fargate Spot キャパシティプロバイダーを利用するか。

<span class="parent-field">count.</span><a id="count-cpu-percentage" href="#count-cpu-percentage" class="field">`cpu_percentage`</a> <span class="type">Integer</span>  
Service が保つべき平均 CPU 使用率を指定し、それによってスケールアップ・ダウンします。

<span class="parent-field">count.</span><a id="count-memory-percentage" href="#count-memory-percentage" class="field">`memory_percentage`</a> <span class="type">Integer</span>  
Service が保つべき平均メモリ使用率を指定し、それによってスケールアップ・ダウンします。

{% include 'exec.ja.md' %}

{% include 'entrypoint.ja.md' %}

{% include 'command.ja.md' %}

{% include 'network.ja.md' %}

{% include 'envvars.ja.md' %}

{% include 'secrets.ja.md' %}

{% include 'storage.ja.md' %}

{% include 'publish.ja.md' %}

{% include 'logging.ja.md' %}

{% include 'taskdef-overrides.ja.md' %}

{% include 'environments.ja.md' %}
