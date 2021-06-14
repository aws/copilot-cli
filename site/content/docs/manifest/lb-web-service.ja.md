以下は `'Load Balanced Web Service'` Manifest で利用できるすべてのプロパティのリストです。[Copilot Service の概念](../concepts/services.ja.md)説明のページも合わせてご覧ください。

???+ note "frontend Service のサンプル Manifest"

```yaml
# Service 名はロググループや ECS サービスなどのリソースの命名に利用されます。
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

# 上記すべての値は Environment ごとにオーバーライド可能です。
environments:
  test:
    count:
      range:
        min: 1
        max: 10
        spot_from: 2
  staging:
    count:
      spot: 2
  production:
    count: 2
```

<a id="name" href="#name" class="field">`name`</a> <span class="type">String</span>  
Service の名前。

<div class="separator"></div>

<a id="type" href="#type" class="field">`type`</a> <span class="type">String</span>  
Service のアーキテクチャタイプ。 [Load Balanced Web Service](../concepts/services.ja.md#load-balanced-web-service) は、ロードバランサー及び AWS Fargate 上の Amazon ECS によって構成される、インターネットに公開するための Service です。

{% include 'http-config.ja.md' %}

{% include 'image-config.ja.md' %}

{% include 'image-healthcheck.ja.md' %}

{% include 'common-svc-fields.ja.md' %}
