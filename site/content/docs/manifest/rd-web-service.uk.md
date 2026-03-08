---
title: "Вебсервіс на основі запитів"
---

# Вебсервіс на основі запитів

Список всіх доступних властивостей для маніфесту `'Request-Driven Web Service'`.

???+ note "Приклади маніфестів AWS App Runner"

    === "Публічний"

        ```yaml
        # Розгортає вебсервіс, доступний за адресою https://web.example.com.
        name: frontend
        type: Request-Driven Web Service

        http:
          healthcheck: '/_healthcheck'
          alias: web.example.com

        image:
          build: ./frontend/Dockerfile
          port: 80
        cpu: 1024
        memory: 2048

        variables:
          LOG_LEVEL: info
        tags:
          owner: frontend
        observability:
          tracing: awsxray
        secrets:
          GITHUB_TOKEN: GITHUB_TOKEN
          DB_SECRET:
            secretsmanager: 'mysql'

        environments:
          test:
            variables:
              LOG_LEVEL: debug
        ```

    === "Підключений до VPC середовища"

        ```yaml
        # Весь вихідний трафік маршрутизується через VPC середовища.
        name: frontend
        type: Request-Driven Web Service

        image:
          build: ./frontend/Dockerfile
          port: 8080
        cpu: 1024
        memory: 2048

        network:
          vpc:
            placement: private
        ```

    === "Подієво-орієнтований"

        ```yaml
        # Дивіться https://aws.github.io/copilot-cli/docs/developing/publish-subscribe/
        name: refunds
        type: Request-Driven Web Service

        image:
          build: ./refunds/Dockerfile
          port: 8080

        http:
          alias: refunds.example.com
        cpu: 1024
        memory: 2048

        publish:
          topics:
            - name: 'refunds'
            - name: 'orders'
              fifo: true
        ```

<a id="name" href="#name" class="field">`name`</a> <span class="type">String</span>
Назва вашого сервісу.

<div class="separator"></div>

<a id="type" href="#type" class="field">`type`</a> <span class="type">String</span>
Тип архітектури вашого сервісу. [Вебсервіс на основі запитів](../../concepts/services/#request-driven-web-service) — це сервіс, що доступний через інтернет і розгортається на AWS App Runner.

<div class="separator"></div>

<a id="http" href="#http" class="field">`http`</a> <span class="type">Map</span>
Розділ http містить параметри, повʼязані з керованим балансувальником навантаження.

<span class="parent-field">http.</span><a id="http-private" href="#http-private" class="field">`private`</a> <span class="type">Bool або Map</span>
Обмежити вхідний трафік лише вашим середовищем. Стандартно false.

<span class="parent-field">http.private</span><a id="http-private-endpoint" href="#http-private-endpoint" class="field">`endpoint`</a> <span class="type">String</span>
ID наявного VPC Endpoint до App Runner.

```yaml
http:
  private:
    endpoint: vpce-12345
```

<span class="parent-field">http.</span><a id="http-healthcheck" href="#http-healthcheck" class="field">`healthcheck`</a> <span class="type">String або Map</span>
Якщо ви вказуєте рядок, Copilot інтерпретує його як шлях, що відкривається у вашому контейнері для обробки запитів на перевірку стану цільової групи. Стандартно "/".

```yaml
http:
  healthcheck: '/'
```

Ви також можете вказати healthcheck як map:

```yaml
http:
  healthcheck:
    path: '/'
    healthy_threshold: 3
    unhealthy_threshold: 2
    interval: 15s
    timeout: 10s
```

<span class="parent-field">http.healthcheck.</span><a id="http-healthcheck-path" href="#http-healthcheck-path" class="field">`path`</a> <span class="type">String</span>
Місце призначення, куди надсилаються запити на перевірку стану.

<span class="parent-field">http.healthcheck.</span><a id="http-healthcheck-healthy-threshold" href="#http-healthcheck-healthy-threshold" class="field">`healthy_threshold`</a> <span class="type">Integer</span>
Кількість послідовних успішних перевірок стану, необхідних для визнання несправної цілі справною. Стандартно 3. Діапазон: 1-20.

<span class="parent-field">http.healthcheck.</span><a id="http-healthcheck-unhealthy-threshold" href="#http-healthcheck-unhealthy-threshold" class="field">`unhealthy_threshold`</a> <span class="type">Integer</span>
Кількість послідовних невдалих перевірок стану, необхідних для визнання цілі несправною. Стандартно 3. Діапазон: 1-20.

<span class="parent-field">http.healthcheck.</span><a id="http-healthcheck-interval" href="#http-healthcheck-interval" class="field">`interval`</a> <span class="type">Duration</span>
Приблизний час, у секундах, між перевірками стану окремої цілі. Стандартно 5s. Діапазон: 1s–20s.

<span class="parent-field">http.healthcheck.</span><a id="http-healthcheck-timeout" href="#http-healthcheck-timeout" class="field">`timeout`</a> <span class="type">Duration</span>
Час, у секундах, протягом якого відсутність відповіді від цілі означає невдалу перевірку стану. Стандартно 2s. Діапазон 1s-20s.

<span class="parent-field">http.</span><a id="http-alias" href="#http-alias" class="field">`alias`</a> <span class="type">String</span>
Призначте дружнє доменне імʼя вашим вебсервісам на основі запитів. Щоб дізнатися більше, дивіться [`developing/domain`](../../developing/domain/#request-driven-web-service).

<div class="separator"></div>

<a id="image" href="#image" class="field">`image`</a> <span class="type">Map</span>
Розділ image містить параметри, що стосуються конфігурації збірки Docker та експонованого порту.

<span class="parent-field">image.</span><a id="image-build" href="#image-build" class="field">`build`</a> <span class="type">Рядок або Карта</span>
Якщо ви вказуєте рядок, Copilot інтерпретує його як шлях до вашого Dockerfile. Він припускає, що імʼя теки рядка, який ви вказуєте, має бути контекстом збірки. Маніфест:

```yaml
image:
  build: path/to/dockerfile
```

призведе до наступного виклику docker build: `$ docker build --file path/to/dockerfile path/to`

Ви також можете вказати build як map:

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
У цьому випадку Copilot використовуватиме вказаний вами контекстну теку і перетворюватиме пари ключ-значення з args на перевизначення --build-arg. Еквівалентний виклик docker build буде:
`$ docker build --file path/to/dockerfile --target build-stage --cache-from image:tag --build-arg key=value context/dir`.

Ви можете пропустити поля, і Copilot зробить все можливе, щоб зрозуміти, що ви маєте на увазі. Наприклад, якщо ви вказуєте `context`, але не `dockerfile`, Copilot запустить Docker у контекстній теці та припустить, що ваш Dockerfile називається "Dockerfile". Якщо ви вказуєте `dockerfile`, але не `context`, Copilot припускає, що ви хочете запустити Docker у теці, що містить `dockerfile`.

Усі шляхи відносні до кореня вашого робочого простору.

<span class="parent-field">image.</span><a id="image-location" href="#image-location" class="field">`location`</a> <span class="type">String</span>
Замість створення контейнера з Dockerfile, ви можете вказати наявне імʼя образу. Конфліктує з [`image.build`](#image-build).

!!! note "Примітка"
    Лише публічні образи, збережені в [Amazon ECR Public](https://docs.aws.amazon.com/AmazonECR/latest/public/public-repositories.html), доступні у AWS App Runner.

<span class="parent-field">image.</span><a id="image-port" href="#image-port" class="field">`port`</a> <span class="type">Integer</span>
Порт, експонований у вашому Dockerfile. Copilot повинен проаналізувати це значення для вас з вашої інструкції `EXPOSE`.

<div class="separator"></div>

<a id="cpu" href="#cpu" class="field">`cpu`</a> <span class="type">Integer</span>
Кількість одиниць CPU, зарезервованих для кожного екземпляра вашого сервісу. Дивіться [документацію AWS App Runner](https://docs.aws.amazon.com/apprunner/latest/api/API_InstanceConfiguration.html#apprunner-Type-InstanceConfiguration-Cpu) для дійсних значень CPU.

<div class="separator"></div>

<a id="memory" href="#memory" class="field">`memory`</a> <span class="type">Integer</span>
Кількість памʼяті в MiB, зарезервованої для кожного екземпляра вашого сервісу. Дивіться [документацію AWS App Runner](https://docs.aws.amazon.com/apprunner/latest/api/API_InstanceConfiguration.html#apprunner-Type-InstanceConfiguration-Memory) для дійсних значень памʼяті.

<div class="separator"></div>

<a id="network" href="#network" class="field">`network`</a> <span class="type">Map</span>
Розділ `network` містить параметри для підключення сервісу до ресурсів AWS у VPC середовища.
Підключивши сервіс до VPC, ви можете використовувати [service discovery](../../developing/svc-to-svc-communication/#service-discovery) для спілкування з іншими сервісами
у вашому середовищі або підключитися до бази даних у вашому VPC, такої як Amazon Aurora, за допомогою [`storage init`](../../commands/storage-init/).

<span class="parent-field">network.</span><a id="network-vpc" href="#network-vpc" class="field">`vpc`</a> <span class="type">Map</span>
Підмережі у VPC для маршрутизації вихідного трафіку з сервісу.

<span class="parent-field">network.vpc.</span><a id="network-vpc-placement" href="#network-vpc-placement" class="field">`placement`</a> <span class="type">String</span>
Єдиний правильний варіант сьогодні — `'private'`. Якщо ви не хочете, щоб сервіс був підключений до VPC, ви можете видалити поле `network`.

Коли розміщення - `'private'`, сервіс App Runner маршрутизує вихідний трафік через приватні підмережі VPC.
Якщо ви використовуєте VPC, створений Copilot, Copilot автоматично додасть NAT-шлюзи до вашого середовища для підключення до інтернету. (Дивіться [ціни](https://aws.amazon.com/vpc/pricing/).) Альтернативно, при запуску `copilot env init`, ви можете імпортувати наявний VPC з NAT-шлюзами або один з VPC endpoints для ізольованих робочих навантажень. Дивіться нашу сторінку [custom environment resources](../../developing/custom-environment-resources/) для отримання додаткової інформації.

{% include 'observability.uk.md' %}

<div class="separator"></div>

<a id="command" href="#command" class="field">`command`</a> <span class="type">String</span>
Необовʼязково. Перевизначає стандартну команду в образі.

<div class="separator"></div>

<a id="variables" href="#variables" class="field">`variables`</a> <span class="type">Map</span>
Пари ключ-значення, що представляють змінні середовища, які будуть передані вашому сервісу. Copilot стандартно включить для вас ряд змінних середовища.

{% include 'secrets.uk.md' %}

{% include 'publish.uk.md' %}

<div class="separator"></div>

<a id="variables" href="#variables" class="field">`tags`</a> <span class="type">Map</span>
Пари ключ-значення, що представляють теґи AWS, які передаються вашим ресурсам AWS App Runner.

<div class="separator"></div>

<a id="count" href="#count" class="field">`count`</a> <span class="type">String</span>
Вкажіть імʼя наявної конфігурації автомасштабування.

```yaml
count: high-availability/3
```

<div class="separator"></div>

<a id="environments" href="#environments" class="field">`environments`</a> <span class="type">Map</span>
Розділ середовища дозволяє перевизначити будь-яке значення у вашому маніфесті на основі середовища, в якому ви знаходитесь. У наведеному вище прикладі маніфесту ми перевизначаємо змінну середовища `LOG_LEVEL` у нашому середовищі 'test'.
