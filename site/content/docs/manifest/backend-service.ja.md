以下は `'Backend Service'` Manifest で利用できるすべてのプロパティのリストです。

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

{% include 'image-config.ja.md' %}
{% include 'image-healthcheck.ja.md' %}
{% include 'common-svc-fields.ja.md' %}
