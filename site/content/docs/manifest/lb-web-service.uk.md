---
title: "Вебсервіс з балансуванням навантаження"
---

# Вебсервіс з балансуванням навантаження

Список усіх доступних властивостей для маніфесту 'Load Balanced Web Service'. Щоб дізнатися більше про сервіси Copilot, перегляньте сторінку концепцій [Сервіси](../../concepts/services/).

???+ note "Приклади маніфестів для сервісу з доступом до інтернету"

    === "Базовий"

        ```yaml
        name: 'frontend'
        type: 'Load Balanced Web Service'

        image:
          build: './frontend/Dockerfile'
          port: 8080

        http:
          path: '/'
          healthcheck: '/_healthcheck'

        cpu: 256
        memory: 512
        count: 3
        exec: true

        variables:
          LOG_LEVEL: info
        secrets:
          GITHUB_TOKEN: GITHUB_TOKEN
          DB_SECRET:
            secretsmanager: 'mysql'
        ```

    === "З доменом"

        ```yaml
        name: 'frontend'
        type: 'Load Balanced Web Service'

        image:
          build: './frontend/Dockerfile'
          port: 8080

        http:
          path: '/'
          alias: 'example.com'

        environments:
          qa:
            http:
              alias: # Середовще "qa" імпортує сертифікат.
                - name: 'qa.example.com'
                  hosted_zone: Z0873220N255IR3MTNR4
        ```

    === "Великі контейнери"

        ```yaml
        # Наприклад, ми можемо захотіти прогріти наш Java-сервіс перед тим, як приймати зовнішній трафік.
        name: 'frontend'
        type: 'Load Balanced Web Service'

        image:
          build:
            dockerfile: './frontend/Dockerfile'
            context: './frontend'
          port: 80

        http:
          path: '/'
          healthcheck:
            path: '/_deephealthcheck'
            port: 8080
            success_codes: '200,301'
            healthy_threshold: 4
            unhealthy_threshold: 2
            interval: 15s
            timeout: 10s
            grace_period: 2m
          deregistration_delay: 50s
          stickiness: true
          allowed_source_ips: ["10.24.34.0/23"]

        cpu: 2048
        memory: 4096
        count: 3
        storage:
          ephemeral: 100

        network:
          vpc:
            placement: 'private'
        ```

    === "Автомасштабування"

        ```yaml
        name: 'frontend'
        type: 'Load Balanced Web Service'

        http:
          path: '/'
        image:
          location: aws_account_id.dkr.ecr.us-west-2.amazonaws.com/frontend:latest
          port: 80

        cpu: 512
        memory: 1024
        count:
          range: 1-10
          cooldown:
            in: 60s
            out: 30s
          cpu_percentage: 70
          requests: 30
          response_time: 2s
        ```

    === "На основі подій"

        ```yaml
        # See https://aws.github.io/copilot-cli/docs/developing/publish-subscribe/
        name: 'orders'
        type: 'Load Balanced Web Service'

        image:
          build: Dockerfile
          port: 80
        http:
          path: '/'
          alias: 'orders.example.com'

        variables:
          DDB_TABLE_NAME: 'orders'

        publish:
          topics:
            - name: 'products'
            - name: 'orders'
              fifo: true
        ```

    === "Мережевий балансувальник"

        ```yaml
        name: 'frontend'
        type: 'Load Balanced Web Service'

        image:
          build: Dockerfile
          port: 8080

        http: false
        nlb:
          alias: 'example.com'
          port: 80/tcp
          target_container: envoy

        network:
          vpc:
            placement: 'private'

        sidecars:
          envoy:
            port: 80
            image: aws_account_id.dkr.ecr.us-west-2.amazonaws.com/envoy:latest
        ```

    === "Спільна файлова система"

        ```yaml
        # Дивіться http://localhost:8000/copilot-cli/docs/developing/storage/#file-systems
        name: 'frontend'
        type: 'Load Balanced Web Service'

        image:
          build: Dockerfile
          port: 80
          depends_on:
            bootstrap: success

        http:
          path: '/'

        storage:
          volumes:
            wp:
              path: /bitnami/wordpress
              read_only: false
              efs: true

        # Наповніть файлову систему деяким вмістом за допомогою контейнера bootstrap.
        sidecars:
          bootstrap:
            image: aws_account_id.dkr.ecr.us-west-2.amazonaws.com/bootstrap:v1.0.0
            essential: false
            mount_points:
              - source_volume: wp
                path: /bitnami/wordpress
                read_only: false
        ```

    === "Наскрізне шифрування"

        ```yaml
        name: 'frontend'
        type: 'Load Balanced Web Service'

        image:
          build: Dockerfile
          port: 8080

        http:
          alias: 'example.com'
          path: '/'
          healthcheck:
            path: '/_health'

          # Порт контейнера envoy - 443, що призводить до того, що протокол і протокол перевірки справності повинні бути "HTTPS"
          # щоб балансувальник навантаження встановлював TLS-зʼєднання із завданнями Fargate, використовуючи сертифікати, які ви
          # встановлюєте на контейнер envoy. Ці сертифікати можуть бути самопідписними.
          target_container: envoy

        sidecars:
          envoy:
            port: 443
            image: aws_account_id.dkr.ecr.us-west-2.amazonaws.com/envoy-proxy-with-selfsigned-certs:v1

        network:
          vpc:
            placement: 'private'
        ```

    === "Експонування кількох портів"

        ```yaml
        name: 'frontend'
        type: 'Load Balanced Web Service'

        image:
          build: './frontend/Dockerfile'
          port: 8080

        nlb:
          port: 8080/tcp              # Трафік через порт 8080/tcp перенаправляється до основного контейнера, на порт 8080.
          additional_listeners:
            - port: 8084/tcp          # Трафік на порт 8084/tcp перенаправляється до основного контейнера, на порт 8084.
            - port: 8085/tcp          # Трафік через порт 8085/tcp перенаправляється на sidecar "envoy", на порт 3000.
              target_port: 3000
              target_container: envoy

        http:
          path: '/'
          target_port: 8083           # Трафік на "/" перенаправляється до основного контейнера, в порт 8083.
          additional_rules:
            - path: 'customerdb'
              target_port: 8081       # Трафік на "/customerdb" перенаправляється в основний контейнер, на порт 8083.
            - path: 'admin'
              target_port: 8082       # Трафік на "/admin" перенаправляється на sidecar "envoy", на порт 8082.
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
Тип архітектури вашого сервісу. [Вебсервіс з балансуванням навантаження](../../concepts/services/#load-balanced-web-service) - це сервіс з доступом до інтернету, який працює за балансувальником навантаження, оркеструється Amazon ECS на AWS Fargate.

<div class="separator"></div>

<a id="http" href="#http" class="field">`http`</a> <span class="type">Boolean або Map</span>
Розділ http містить параметри, повʼязані з інтеграцією вашого сервісу з балансувальником навантаження Application Load Balancer.

Щоб вимкнути Application Load Balancer, вкажіть `http: false`. Зверніть увагу, що для вебсервісу з балансуванням навантаження повинен бути увімкнений хоча б один з балансувальників — Application Load Balancer або Network Load Balancer.

<span class="parent-field">http.</span><a id="http-path" href="#http-path" class="field">`path`</a> <span class="type">String</span>
Запити до цього шляху будуть перенаправлені до вашого сервісу. Кожне правило слухача повинно слухати унікальний шлях.

<span class="parent-field">http.</span><a id="http-alb" href="#http-alb" class="field">`alb`</a> <span class="type">String</span> <span class="version">Додано у [v1.32.0](../../../blogs/release-v132/#imported-albs)</span>
ARN або назва навного публічного ALB для імпорту. Правила слухача будуть додані до ваших слухачів. Copilot не буде керувати ресурсами, повʼязаними з DNS, такими як сертифікати.

{% include 'http-healthcheck.uk.md' %}

<span class="parent-field">http.</span><a id="http-deregistration-delay" href="#http-deregistration-delay" class="field">`deregistration_delay`</a> <span class="type">Duration</span>
Час очікування для зняття цілей з реєстрації під час зняття з реєстрації. Стандартно це 60 секунд. Встановлення цього значення на більше значення дає цілям більше часу для належного завершення зʼєднань, але збільшує час, необхідний для нових розгортань. Діапазон 0s-3600s.

<span class="parent-field">http.</span><a id="http-target-container" href="#http-target-container" class="field">`target_container`</a> <span class="type">String</span>
Контейнер sidecar, до якого перенаправляються запити замість основного контейнера сервісу.
Якщо порт контейнера призначення встановлено на `443`, то протокол встановлюється на `HTTPS`, щоб балансувальник навантаження встановлював TLS-зʼєднання з завданнями Fargate, використовуючи сертифікати, які ви встановлюєте на контейнер призначення.

<span class="parent-field">http.</span><a id="http-target-port" href="#http-target-port" class="field">`target_port`</a> <span class="type">String</span>
Необовʼязково. Порт контейнера, який отримує трафік. Стандартно це буде `image.port`, якщо контейнер призначення є основним контейнером, або `sidecars.<name>.port`, якщо контейнер призначення є sidecar.

<span class="parent-field">http.</span><a id="http-stickiness" href="#http-stickiness" class="field">`stickiness`</a> <span class="type">Boolean</span>
Вказує, чи увімкнено липкі сесії.

<span class="parent-field">http.</span><a id="http-allowed-source-ips" href="#http-allowed-source-ips" class="field">`allowed_source_ips`</a> <span class="type">Array of Strings</span>
CIDR IP-адреси, яким дозволено доступ до вашого сервісу.

```yaml
http:
  allowed_source_ips: ["192.0.2.0/24", "198.51.100.10/32"]
```

<span class="parent-field">http.</span><a id="http-alias" href="#http-alias" class="field">`alias`</a> <span class="type">String або Array of Strings або Array of Maps</span>
HTTPS-доменний псевдонім вашого сервісу.

```yaml
# Версія з рядком.
http:
  alias: example.com
# Або як масив рядків.
http:
  alias: ["example.com", "v1.example.com"]
# Або як масив з map.
http:
  alias:
    - name: example.com
      hosted_zone: Z0873220N255IR3MTNR4
    - name: v1.example.com
      hosted_zone: AN0THE9H05TED20NEID
```

<span class="parent-field">http.</span><a id="http-hosted-zone" href="#http-hosted-zone" class="field">`hosted_zone`</a> <span class="type">String</span>
ID вашої наявної зони хостингу; може використовуватися тільки з `http.alias`. Якщо у вас є середовище з імпортованими сертифікатами, ви можете вказати зону хостингу, в яку Copilot повинен вставити запис A після створення балансувальника навантаження.

```yaml
http:
  alias: example.com
  hosted_zone: Z0873220N255IR3MTNR4
# Також дивіться приклад масиву з map http.alias вище.
```

<span class="parent-field">http.</span><a id="http-redirect-to-https" href="#http-redirect-to-https" class="field">`redirect_to_https`</a> <span class="type">Boolean</span>
Автоматично перенаправляє балансувальник навантаження з HTTP на HTTPS. Стандартно `true`.

<span class="parent-field">http.</span><a id="http-version" href="#http-version" class="field">`version`</a> <span class="type">String</span>
Версія протоколу HTTP(S). Повинна бути однією з `'grpc'`, `'http1'` або `'http2'`. Якщо не вказано, то Стандартно використовується `'http1'`. Якщо ви використовуєте gRPC, зверніть увагу, що домен повинен бути повʼязаний з вашим застосунком.

<span class="parent-field">http.</span><a id="http-additional-rules" href="#http-additional-rules" class="field">`additional_rules`</a> <span class="type">Array of Maps</span>
Налаштуйте кілька правил слухача ALB.

{% include 'http-additionalrules.uk.md' %}

{% include 'nlb.uk.md' %}

{% include 'image-config-with-port.uk.md' %}
Якщо порт встановлено на `443`, то протокол встановлюється на `HTTPS`, щоб балансувальник навантаження встановлював TLS-зʼєднання з завданнями Fargate, використовуючи сертифікати, які ви встановлюєте на контейнер.

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

Якщо ви хочете використовувати потужності Fargate Spot для запуску ваших сервісів, ви можете вказати число в підполі `spot`:

```yaml
count:
  spot: 5
```

!!! info "Інформація"
    Fargate Spot не підтримується для контейнерів, що працюють на архітектурі ARM.

<div class="separator"></div>

Альтернативно, ви можете вказати map для налаштування автомасштабування:

```yaml
count:
  range: 1-10
  cooldown:
    in: 30s
    out: 60s
  cpu_percentage: 70
  memory_percentage:
    value: 80
    cooldown:
      in: 80s
      out: 160s
  requests: 10000
  response_time: 2s
```

<span class="parent-field">count.</span><a id="count-range" href="#count-range" class="field">`range`</a> <span class="type">String або Map</span>
Ви можете вказати мінімальну та максимальну межу для кількості завдань, які ваш сервіс повинен підтримувати, на основі значень, які ви вказуєте для метрик.

```yaml
count:
  range: n-m
```

Це налаштує ціль автомасштабування застосунків з `MinCapacity` на `n` та `MaxCapacity` на `m`.

Альтернативно, якщо ви хочете масштабувати свій сервіс на екземплярі Fargate Spot, вкажіть `min` та `max` під `range`, а потім вкажіть `spot_from` з бажаною кількістю, з якої ви хочете почати розміщувати свої сервіси на потужностях Spot. Наприклад:

```yaml
count:
  range:
    min: 1
    max: 10
    spot_from: 3
```

Це встановить ваш діапазон як 1-10, як вище, але розмістить перші дві копії вашого сервісу на виділеній потужності Fargate. Якщо ваш сервіс масштабується до 3 або більше, третя та будь-які додаткові копії будуть розміщені на Spot до досягнення максимуму.

<span class="parent-field">count.range.</span><a id="count-range-min" href="#count-range-min" class="field">`min`</a> <span class="type">Integer</span>
Мінімальна бажана кількість для вашого сервісу з використанням автомасштабування.

<span class="parent-field">count.range.</span><a id="count-range-max" href="#count-range-max" class="field">`max`</a> <span class="type">Integer</span>
Максимальна бажана кількість для вашого сервісу з використанням автомасштабування.

<span class="parent-field">count.range.</span><a id="count-range-spot-from" href="#count-range-spot-from" class="field">`spot_from`</a> <span class="type">Integer</span>
Бажана кількість, з якої ви хочете почати розміщувати свій сервіс з використанням потужності Fargate Spot.

<span class="parent-field">count.</span><a id="count-cooldown" href="#count-cooldown" class="field">`cooldown`</a> <span class="type">Map</span>
Поля охолодження, які використовуються як стандартне значення для всіх полів автомасштабування.

<span class="parent-field">count.cooldown.</span><a id="count-cooldown-in" href="#count-cooldown-in" class="field">`in`</a> <span class="type">Duration</span>
Час охолодження для полів автомасштабування для зменшення масштабу сервісу.

<span class="parent-field">count.cooldown.</span><a id="count-cooldown-out" href="#count-cooldown-out" class="field">`out`</a> <span class="type">Duration</span>
Час охолодження для полів автомасштабування для збільшення масштабу сервісу.

Наступні опції `cpu_percentage`, `memory_percentage`, `requests` та `response_time` є полями автомасштабування для `count`, які можуть бути визначені як значення поля, або як map, що містить розширену інформацію про значення поля та охолодження:

```yaml
value: 50
cooldown:
  in: 30s
  out: 60s
```

Охолодження, вказане тут, перевизначить стандартне значення.

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
