---
title: "Backend Service"
---

# Backend Service

Список всіх доступних властивостей для маніфесту `'Backend Service'`. Щоб дізнатися про сервіси Copilot, дивіться сторінку концепції [Сервіси](../../concepts/services/).

???+ note "Приклади маніфестів сервісу бекенду"

    === "Обслуговування внутрішнього трафіку"

        ```yaml
            name: api
            type: Backend Service

            image:
              build: ./api/Dockerfile
              port: 8080
              healthcheck:
                command: ["CMD-SHELL", "curl -f http://localhost:8080 || exit 1"]
                interval: 10s
                retries: 2
                timeout: 5s
                start_period: 0s

            network:
              connect: true

            cpu: 256
            memory: 512
            count: 2
            exec: true

            env_file: ./api/.env
            environments:
              test:
                deployment:
                  rolling: "recreate"
                count: 1
        ```

    === "Внутрішній балансувальник навантаження застосунків"

        ```yaml
        # Ваш сервіс доступний за адресою:
        # http://api.${COPILOT_ENVIRONMENT_NAME}.${COPILOT_APPLICATION_NAME}.internal
        # за внутрішнім балансувальником навантаження лише у вашій VPC.
        name: api
        type: Backend Service

        image:
          build: ./api/Dockerfile
          port: 8080

        http:
          path: '/'
          healthcheck:
            path: '/_healthcheck'
            success_codes: '200,301'
            healthy_threshold: 3
            interval: 15s
            timeout: 10s
            grace_period: 30s
          deregistration_delay: 50s

        network:
          vpc:
            placement: 'private'

        count:
          range: 1-10
          cpu_percentage: 70
          requests: 10
          response_time: 2s

        secrets:
          GITHUB_WEBHOOK_SECRET: GH_WEBHOOK_SECRET
          DB_PASSWORD:
            secretsmanager: 'mysql:password::'
        ```

    === "З доменом"

        ```yaml
        # Припускаючи, що у вашому середовищі імпортовані приватні сертифікати, ви можете призначити
        # HTTPS кінцеву точку для вашого сервісу.
        # Дивіться https://aws.github.io/copilot-cli/docs/manifest/environment/#http-private-certificates
        name: api
        type: Backend Service

        image:
          build: ./api/Dockerfile
          port: 8080

        http:
          path: '/'
          alias: 'v1.api.example.com'
          hosted_zone: AN0THE9H05TED20NEID # Вставте запис для v1.api.example.com в зону хостингу.

        count: 1
        ```

    === "Подієво-орієнтований"

        ```yaml
        # Дивіться https://aws.github.io/copilot-cli/docs/developing/publish-subscribe/
        name: warehouse
        type: Backend Service

        image:
          build: ./warehouse/Dockerfile
          port: 80

        publish:
          topics:
            - name: 'inventory'
            - name: 'orders'
              fifo: true

        variables:
          DDB_TABLE_NAME: 'inventory'

        count:
          range: 3-5
          cpu_percentage: 70
          memory_percentage: 80
        ```

    === "Спільна файлова система"

        ```yaml
        # Дивіться http://localhost:8000/copilot-cli/docs/developing/storage/#file-systems
        name: sync
        type: Backend Service

        image:
          build: Dockerfile

        variables:
          S3_BUCKET_NAME: my-userdata-bucket

        storage:
          volumes:
            userdata:
              path: /etc/mount1
              efs:
                id: fs-1234567
        ```

    === "Відкриття кількох портів"

        ```yaml
        name: 'backend'
        type: 'Backend Service'

        image:
          build: './backend/Dockerfile'
          port: 8080

        http:
          path: '/'
          target_port: 8083           # Трафік на "/" перенаправляється до головного контейнера, на порт 8083.
          additional_rules:
            - path: 'customerdb'
              target_port: 8081       # Трафік на "/customerdb" перенаправляється до головного контейнера, на порт 8081.
            - path: 'admin'
              target_port: 8082       # Трафік на "/admin" перенаправляється до побічного контейнера "envoy", на порт 8082.
              target_container: envoy

        sidecars:
          envoy:
            port: 80
            image: aws_account_id.dkr.ecr.us-west-2.amazonaws.com/envoy-proxy-with-selfsigned-certs:v1
        ```

<a id="name" href="#name" class="field">`name`</a> <span class="type">String</span>  
Назва вашого сервісу.

<div class="separator"></div>

<a id="type" href="#type" class="field">`type`</a> <span class="type">String</span>  
Тип архітектури вашого сервісу. [Сервіси бекенду](../../concepts/services/#backend-service) недоступні з інтернету, але можуть бути досягнуті за допомогою [виявлення сервісу](../../developing/svc-to-svc-communication/#service-discovery) з ваших інших сервісів.

<div class="separator"></div>

<a id="http" href="#http" class="field">`http`</a> <span class="type">Map</span>  
Розділ http містить параметри, повʼязані з інтеграцією вашого сервісу з внутрішнім балансувальником навантаження застосунків.

<span class="parent-field">http.</span><a id="http-path" href="#http-path" class="field">`path`</a> <span class="type">String</span>  
Запити до цього шляху будуть перенаправлені до вашого сервісу. Кожен сервіс бекенду повинен слухати на унікальному шляху.

<span class="parent-field">http.</span><a id="http-alb" href="#http-alb" class="field">`alb`</a> <span class="type">String</span> <span class="version">Додано у [v1.33.0](../../../blogs/release-v133/#imported-albs)</span>  
ARN або назва наявного внутрішнього ALB для імпорту. Правила слухача будуть додані до ваших слухачів. Copilot не буде керувати ресурсами, повʼязаними з DNS, такими як сертифікати.

{% include 'http-healthcheck.uk.md' %}

<span class="parent-field">http.</span><a id="http-deregistration-delay" href="#http-deregistration-delay" class="field">`deregistration_delay`</a> <span class="type">Duration</span>  
Час очікування для зняття реєстрації цілей під час зняття реєстрації. Стандартно 60s. Встановлення цього значення на більше значення дає цілям більше часу для належного завершення зʼєднань, але збільшує час, необхідний для нових розгортань. Діапазон 0s-3600s.

<span class="parent-field">http.</span><a id="http-target-container" href="#http-target-container" class="field">`target_container`</a> <span class="type">String</span>  
Sidecar контейнер, до якого перенаправляються запити замість головного контейнера сервісу. Якщо порт цільового контейнера встановлено на `443`, тоді протокол встановлюється як `HTTPS`, щоб балансувальник навантаження встановлював TLS зʼєднання з завданнями Fargate, використовуючи сертифікати, які ви встановлюєте на цільовий контейнер.

<span class="parent-field">http.</span><a id="http-stickiness" href="#http-stickiness" class="field">`stickiness`</a> <span class="type">Boolean</span>  
Вказує, чи ввімкнені липкі сесії.

<span class="parent-field">http.</span><a id="http-allowed-source-ips" href="#http-allowed-source-ips" class="field">`allowed_source_ips`</a> <span class="type">Array of Strings</span>  
CIDR IP адреси, яким дозволено доступ до вашого сервісу.

```yaml
http:
  allowed_source_ips: ["192.0.2.0/24", "198.51.100.10/32"]
```

<span class="parent-field">http.</span><a id="http-alias" href="#http-alias" class="field">`alias`</a> <span class="type">String або Array of Strings або Array of Maps</span>  
HTTPS доменний псевдонім вашого сервісу.

```yaml
# Значення рядок.
http:
  alias: example.com
# Обо масив рядків.
http:
  alias: ["example.com", "v1.example.com"]
# Чи масив з map.
http:
  alias:
    - name: example.com
      hosted_zone: Z0873220N255IR3MTNR4
    - name: v1.example.com
      hosted_zone: AN0THE9H05TED20NEID
```
<span class="parent-field">http.</span><a id="http-hosted-zone" href="#http-hosted-zone" class="field">`hosted_zone`</a> <span class="type">String</span>  
ID наявної приватної зони хостингу, в яку Copilot вставить запис псевдоніма після створення внутрішнього балансувальника навантаження, зіставляючи імʼя псевдоніма з DNS-імʼям LB. Повинно використовуватися з `alias`.

```yaml
http:
  alias: example.com
  hosted_zone: Z0873220N255IR3MTNR4
# Також дивіться приклад http.alias масив з map вище.
```

<span class="parent-field">http.</span><a id="http-version" href="#http-version" class="field">`version`</a> <span class="type">String</span>  
Версія протоколу HTTP(S). Повинна бути однією з `'grpc'`, `'http1'` або `'http2'`. Якщо не вказано, то передбачається `'http1'`. Якщо використовується gRPC, зверніть увагу, що домен повинен бути повʼязаний з вашим застосунком.

<span class="parent-field">http.</span><a id="http-additional-rules" href="#http-additional-rules" class="field">`additional_rules`</a> <span class="type">Array of Maps</span>  
Налаштуйте кілька правил слухача ALB.

{% include 'http-additionalrules.uk.md' %}

{% include 'image-config-with-port.uk.md' %}

Якщо порт встановлено на `443` і ввімкнено внутрішній балансувальник навантаження за допомогою `http`, тоді протокол встановлюється як `HTTPS`, щоб балансувальник навантаження встановлював TLS зʼєднання з завданнями Fargate, використовуючи сертифікати, які ви встановлюєте на контейнер.

{% include 'image-healthcheck.uk.md' %}

{% include 'task-size.uk.md' %}

{% include 'platform.uk.md' %}

<div class="separator"></div>

<a id="count" href="#count" class="field">`count`</a> <span class="type">Integer або Map</span>  
Кількість завдань, які ваш сервіс повинен підтримувати.

Якщо ви вказуєте число:

```yaml
count: 5
```

Сервіс встановить бажану кількість на 5 і підтримуватиме 5 завдань у вашому сервісі.

<span class="parent-field">count.</span><a id="count-spot" href="#count-spot" class="field">`spot`</a> <span class="type">Integer</span>  

Якщо ви хочете використовувати можливість Fargate Spot для запуску ваших сервісів, ви можете вказати число в підполі `spot`:

```yaml
count:
  spot: 5
```

!!! info "Інформація"
    Fargate Spot не підтримується для контейнерів, що працюють на архітектурі ARM.

<div class="separator"></div>

Альтернативно, ви можете вказати map для налаштування автомасштабування

```yaml
count:
  range: 1-10
  cooldown:
    in: 30s
  cpu_percentage: 70
  memory_percentage:
    value: 80
    cooldown:
      out: 45s
  requests: 10000
  response_time: 2s
```

<span class="parent-field">count.</span><a id="count-range" href="#count-range" class="field">`range`</a> <span class="type">String або Map</span>  
Ви можете вказати мінімальну та максимальну межу для кількості завдань, які ваш сервіс повинен підтримувати, на основі значень, які ви вказуєте для метрик.

```yaml
count:
  range: n-m
```

Це налаштує ціль автомасштабування застосунків з `MinCapacity` `n` та `MaxCapacity` `m`.

Альтернативно, якщо ви хочете масштабувати свій сервіс на екземпляри Fargate Spot, вкажіть `min` і `max` в `range`, а потім вкажіть `spot_from` з бажаною кількістю, з якої ви хочете почати розміщувати свої сервіси в Spot. Наприклад:

```yaml
count:
  range:
    min: 1
    max: 10
    spot_from: 3
```

Це встановить ваш діапазон як 1-10, як вище, але розмістить перші дві копії вашого сервісу на виділеній потужності Fargate. Якщо ваш сервіс масштабується до 3 або більше, третя та будь-які додаткові копії будуть розміщені на Spot до досягнення максимуму.

<span class="parent-field">count.range.</span><a id="count-range-min" href="#count-range-min" class="field">`min`</a> <span class="type">Integer</span>  
Мінімальна бажана кількість для вашого сервісу за допомогою автомасштабування.

<span class="parent-field">count.range.</span><a id="count-range-max" href="#count-range-max" class="field">`max`</a> <span class="type">Integer</span>  
Максимальна бажана кількість для вашого сервісу за допомогою автомасштабування.

<span class="parent-field">count.range.</span><a id="count-range-spot-from" href="#count-range-spot-from" class="field">`spot_from`</a> <span class="type">Integer</span>  
Бажана кількість, з якої ви хочете почати розміщувати свій сервіс за допомогою провайдерів потужностей Fargate Spot.

<span class="parent-field">count.</span><a id="count-cooldown" href="#count-cooldown" class="field">`cooldown`</a> <span class="type">Map</span>  
Поля охолодження, які використовуються як стандартне охолодження для всіх полів автомасштабування, що вказані.

<span class="parent-field">count.cooldown.</span><a id="count-cooldown-in" href="#count-cooldown-in" class="field">`in`</a> <span class="type">Duration</span>  
Час охолодження для полів автомасштабування для збільшення масштабу сервісу.

<span class="parent-field">count.cooldown.</span><a id="count-cooldown-out" href="#count-cooldown-out" class="field">`out`</a> <span class="type">Duration</span>  
Час охолодження для полів автомасштабування для зменшення масштабу сервісу.

Наступні опції `cpu_percentage`, `memory_percentage`, `requests` та `response_time` є полями автомасштабування для `count`, які можуть бути визначені як значення поля або як map, що містить розширену інформацію про значення поля та охолодження:

```yaml
value: 50
cooldown:
  in: 30s
  out: 60s
```

Вказане тут охолодження перевизначить стандартне охолодження.

<span class="parent-field">count.</span><a id="count-cpu-percentage" href="#count-cpu-percentage" class="field">`cpu_percentage`</a> <span class="type">Integer або Map</span>  
Масштабування вгору або вниз на основі середнього значення CPU, яке ваш сервіс повинен підтримувати.

<span class="parent-field">count.</span><a id="count-memory-percentage" href="#count-memory-percentage" class="field">`memory_percentage`</a> <span class="type">Integer або Map</span>  
Масштабування вгору або вниз на основі середнього значення памʼяті, яке ваш сервіс повинен підтримувати.

<span class="parent-field">count.</span><a id="requests" href="#count-requests" class="field">`requests`</a> <span class="type">Integer або Map</span>  
Масштабування вгору або вниз на основі кількості запитів, оброблених кожним завданням.

<span class="parent-field">count.</span><a id="response-time" href="#count-response-time" class="field">`response_time`</a> <span class="type">Duration або Map</span>  
Масштабування вгору або вниз на основі середнього часу відповіді сервісу.

{% include 'exec.uk.md' %}

{% include 'deployment.uk.md' %}

```yaml
deployment:
  rollback_alarms:
    cpu_utilization: 70    // Відсоткове значення, при якому або вище якого спрацьовує тривога.
    memory_utilization: 50 // Відсоткове значення, при якому або вище якого спрацьовує тривога.
```

{% include 'entrypoint.uk.md' %}

{% include 'command.uk.md' %}

{% include 'network.uk.md' %}

{% include 'envvars.uk.md' %}

{% include 'secrets.uk.md' %}

{% include 'storage.uk.md' %}

{% include 'publish.uk.md' %}

{% include 'logging.uk.md' %}

{% include 'observability.uk.md' %}

{% include 'taskdef-overrides.uk.md' %}

{% include 'environments.uk.md' %}
