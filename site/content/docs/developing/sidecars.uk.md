---
title: Sidecars
---

# Sidecars

Sidecar — це додаткові контейнери, які працюють поруч з основним контейнером. Зазвичай вони використовуються для виконання периферійних завдань, таких як логування, конфігурація або проксі-запити.

!!! Attention Увага
    Sidecar не підтримуються для вебсервісів, керованих запитами.

!!! Attention Увага
    Якщо ваш основний контейнер використовує Windows-образ, [FireLens](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/using_firelens.html), [AWS X-Ray](https://aws.amazon.com/xray/) та [AWS App Mesh](https://aws.amazon.com/app-mesh/) не підтримуються. Будь ласка, перевірте, чи ваш sidecar-контейнер підтримує Windows.

AWS також надає деякі опції втулків, які можна легко інтегрувати з вашим ECS сервісом, включаючи, але не обмежуючись [FireLens](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/using_firelens.html), [AWS X-Ray](https://aws.amazon.com/xray/) та [AWS App Mesh](https://aws.amazon.com/app-mesh/).

Якщо ви визначили том EFS для вашого основного контейнера через [поле `storage`](../../developing/storage/) в маніфесті, ви також можете монтувати цей том у будь-яких визначених вами sidecar-контейнерах.

## Як додати sidecar з Copilot?

Існує два способи додавання sidecar за допомогою маніфесту Copilot: шляхом визначення [загальних sidecar](#general-sidecars) або використання [шаблонів sidecar](#sidecar-patterns).

### Загальні sidecar <a id="general-sidecars"></a>

Вам потрібно буде надати URL-адресу для образу sidecar. За бажанням, ви можете вказати порт, який хочете експонувати, та параметр облікових даних для [приватного реєстру](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/private-auth.html).

{% include 'sidecar-config.uk.md' %}

<div class="separator"></div>

#### Приклад

##### sidecar з перевизначенням середовища

Подібно до інших полів маніфесту сервісу/завдання, конфігурації sidecar можуть бути перевизначені для кожного середовища через поле [`environments`](../../manifest/lb-web-service/#environments). Нижче наведено приклад, який налаштовує значення для змінної середовища `DD_APM_ENABLED` sidecar `datadog`, залежно від того, чи це середовище `dev`:

```yaml
name: api
type: Load Balanced Web Service

sidecars:
  datadog:
    port: 80
    image:
      build: src/reverseproxy/Dockerfile
    variables:
      DD_APM_ENABLED: true

environments:
  dev:
    sidecars:
      datadog:
        variables:
          DD_APM_ENABLED: false
```

##### Sidecar контейнер [nginx](https://www.nginx.com/)

Нижче наведено приклад визначення sidecar-контейнера [nginx](https://www.nginx.com/) у маніфесті вебсервісу з балансуванням навантаження.

``` yaml
name: api
type: Load Balanced Web Service

image:
  build: api/Dockerfile
  port: 3000

http:
  path: 'api'
  healthcheck: '/api/health-check'
  # Цільовий контейнер для балансувальника навантаження - наш sidecar 'nginx', а не контейнер сервісу.
  target_container: 'nginx'

cpu: 256
memory: 512
count: 1

sidecars:
  nginx:
    port: 80
    image:
      build: src/reverseproxy/Dockerfile
    variables:
      NGINX_PORT: 80
```

##### Том EFS у контейнері сервісу так і в sidecar контейнері

```yaml
storage:
  volumes:
    myEFSVolume:
      path: '/etc/mount1'
      read_only: false
      efs:
        id: fs-1234567

sidecars:
  nginx:
    port: 80
    image: 1234567890.dkr.ecr.us-west-2.amazonaws.com/reverse-proxy:revision_1
    variables:
      NGINX_PORT: 80
    mount_points:
      - source_volume: myEFSVolume
        path: '/etc/mount1'
```

##### Sidecar [AWS Distro for OpenTelemetry](https://aws-otel.github.io/)

Нижче наведено приклад запуску sidecar [AWS Distro for OpenTelemetry](https://aws-otel.github.io/) з власною конфігурацією. Приклад власної конфігурації не тільки збирає дані трасування X-Ray, але й відправляє метрики ECS до третьої сторони. Приклад вимагатиме секрету SSM та додаткових IAM дозволів.

Щоб використовувати sidecar OpenTelemetry, спочатку створіть дійсний [файл конфігурації](https://opentelemetry.io/docs/collector/configuration/). Далі перевірте розмір файлу конфігурації. Стандартний параметр [обмежений до 4KB](https://docs.aws.amazon.com/systems-manager/latest/APIReference/API_PutParameter.html#systemsmanager-PutParameter-request-Value).
Якщо файл конфігурації перевищує 4K, необхідно використовувати розширений параметр SSM.

Якщо потрібен розширений параметр, його потрібно створити та позначити вручну. Якщо конфігурація вміщується в стандартний параметр, створіть секрет SSM за допомогою [`secret init`](../../commands/secret-init/). Нижче наведений документ YAML можна використовувати як є з New Relic після оновлення ключа API "YOUR-API-KEY-HERE".

У прикладі YAML включення порожніх ключів є навмисним. Sidecar використовуватиме стандартні значення для цих ключів.

```yaml
receivers:
  awsxray:
    transport: udp
  awsecscontainermetrics:

processors:
  batch:

exporters:
  awsxray:
    region: us-west-2
  otlp:
    endpoint: otlp.nr-data.net:4317
    headers: 
      api-key: YOUR-API-KEY-HERE

service:
  pipelines:
    traces:
      receivers: [awsxray]
      processors: [batch]
      exporters: [awsxray]
    metrics:
      receivers: [awsecscontainermetrics]
      exporters: [otlp]
```

Запис трейсів X-Ray потребує додаткових IAM дозволів, як показано нижче. Включіть це в застосунки відповідно до [опублікованої документації](../addons/workload/)

``` yaml
Resources:
  XrayWritePolicy:
    Type: AWS::IAM::ManagedPolicy
    Properties:
      PolicyDocument:
        Version: '2012-10-17'
        Statement:
          - Sid: CopyOfAWSXRayDaemonWriteAccess
            Effect: Allow
            Action:
              - xray:PutTraceSegments
              - xray:PutTelemetryRecords
              - xray:GetSamplingRules
              - xray:GetSamplingTargets
              - xray:GetSamplingStatisticSummaries
            Resource: "*"

Outputs:
  XrayAccessPolicyArn:
    Description: "ARN ManagedPolicy для приєднання до ролі завдання."
    Value: !Ref XrayWritePolicy
```

Конфігурація для колектора OpenTelemetry буде передана в sidecar як змінна середовища.

```yaml
sidecars:
  otel_sidecar:
    image: 'public.ecr.aws/aws-observability/aws-otel-collector:latest'
    secrets:
      AOT_CONFIG_CONTENT: /copilot/${COPILOT_APPLICATION_NAME}/${COPILOT_ENVIRONMENT_NAME}/secrets/otel_config
```

### Шаблони sidecar <a id="sidecar-patterns"></a>

Шаблони sidecar — це попередньо визначені конфігурації sidecar Copilot. Наразі підтримується лише шаблон FireLens, але в майбутньому ми додамо більше!

``` yaml
# У маніфесті.
logging:
  # Образ Fluent Bit. (Необовʼязково, стандартно ми використовуємо "public.ecr.aws/aws-observability/aws-for-fluent-bit:stable")
  image: <image URL>
  # Параметри конфігурації для надсилання до драйвера логування FireLens. (Необовʼязково)
  destination:
    <config key>: <config value>
  # Чи включати метадані ECS у логи. (Необовʼязково, стандартно true)
  enableMetadata: <true|false>
  # Секрет для передачі до конфігурації логування. (Необовʼязково)
  secretOptions:
    <key>: <value>
  # Повний шлях до файлу конфігурації у вашому користувацькому образі Fluent Bit. (Необовʼязково)
  configFilePath: <config file path>
  # Змінні середовища для sidecar контейнера. (Необовʼязково)
  variables:
    <key>: <value>
  # Секрети для доступу до sidecar контейнера. (Необовʼязково)
  secrets:
    <key>: <value>
```

Наприклад:

``` yaml
logging:
  destination:
    Name: cloudwatch
    region: us-west-2
    log_group_name: /copilot/sidecar-test-hello
    log_stream_prefix: copilot/
```

Можливо, вам потрібно буде додати необхідні дозволи до ролі завдання, щоб FireLens міг пересилати ваші дані. Ви можете додати дозволи, вказавши їх у своїх [надбудовах](../addons/workload/). Наприклад:

``` yaml
Resources:
  FireLensPolicy:
    Type: AWS::IAM::ManagedPolicy
    Properties:
      PolicyDocument:
        Version: '2012-10-17'
        Statement:
        - Effect: Allow
          Action:
          - logs:CreateLogStream
          - logs:CreateLogGroup
          - logs:DescribeLogStreams
          - logs:PutLogEvents
          Resource: "<resource ARN>"
Outputs:
  FireLensPolicyArn:
    Description: ManagedPolicy застосунка використовується роллю завдання ECS
    Value: !Ref FireLensPolicy
```

!!!info "Інформація"
    Оскільки драйвер логування FireLens може направляти логи вашого основного контейнера до різних пунктів призначення, команда [`svc logs`](../../commands/svc-logs/) може відстежувати їх лише тоді, коли вони надсилаються до групи логів, яку ми створюємо для вашого сервісу Copilot у CloudWatch.
