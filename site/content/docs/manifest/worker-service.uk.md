---
title: Сервіс робочих навантажень
---

# Сервіс робочих навантажень

Список всіх доступних властивостей для маніфесту `'Worker Service'`. Щоб дізнатися більше про сервіси Copilot, перегляньте сторінку концепцій [Сервіси](../../concepts/services/).

???+ note "Приклади маніфестів Worker Service"

    === "Одна черга"

        ```yaml
        # Збирає повідомлення з кількох тем, опублікованих з інших сервісів, в одну чергу SQS.
        name: cost-analyzer
        type: Worker Service

        image:
          build: ./cost-analyzer/Dockerfile

        subscribe:
          topics:
            - name: products
              service: orders
              filter_policy:
                event:
                - anything-but: order_cancelled
            - name: inventory
              service: warehouse
          queue:
            retention: 96h
            timeout: 30s
            dead_letter:
              tries: 10

        cpu: 256
        memory: 512
        count: 3
        exec: true

        secrets:
          DB:
            secretsmanager: 'mysql'
        ```

    === "Автомасштабування Spot"

        ```yaml
        # Передає завдання на Fargate Spot, якщо є можливість.
        name: cost-analyzer
        type: Worker Service

        image:
          build: ./cost-analyzer/Dockerfile

        subscribe:
          topics:
            - name: products
              service: orders
            - name: inventory
              service: warehouse

        cpu: 256
        memory: 512
        count:
          range:
            min: 1
            max: 10
            spot_from: 2
          queue_delay: # Ensure messages are processed within 10mins assuming a single message takes 250ms to process.
            acceptable_latency: 10m
            msg_processing_time: 250ms
        exec: true
        ```

    === "Окремі черги"

        ```yaml
        # Assign individual queues to each topic.
        name: cost-analyzer
        type: Worker Service

        image:
          build: ./cost-analyzer/Dockerfile

        subscribe:
          topics:
            - name: products
              service: orders
              queue:
                retention: 5d
                timeout: 1h
                dead_letter:
                  tries: 3
            - name: inventory
              service: warehouse
              queue:
                retention: 1d
                timeout: 5m
        count: 1
        ```

<a id="name" href="#name" class="field">`name`</a> <span class="type">String</span>  
Назва вашого сервісу.

<div class="separator"></div>

<a id="type" href="#type" class="field">`type`</a> <span class="type">String</span>  
Тип архітектури вашого сервісу. [Worker Service](../../concepts/services/#worker-service) недоступні з інтернету або з інших місць у VPC. Вони призначені для отримання повідомлень з їх повʼязаних черг SQS, які заповнюються їх підписками на теми SNS, створені полями `publish` інших сервісів Copilot.

<div class="separator"></div>

<a id="subscribe" href="#subscribe" class="field">`subscribe`</a> <span class="type">Map</span>  
Розділ `subscribe` дозволяє worker сервісам створювати підписки на теми SNS, які надаються іншими сервісами Copilot у тому ж застосунку та середовищі. Кожна тема може визначати свою власну чергу SQS, але стандартно всі теми підписуються на чергу стандартного worker сервісу.

URI черги стандартно буде вставлено в контейнер як змінну середовища, `COPILOT_QUEUE_URI`. 

```yaml
subscribe:
  topics:
    - name: events
      service: api
      queue: # Визначає конкретну чергу для теми api-events.
        timeout: 20s
    - name: events
      service: fe
  queue: # Стандартно повідомлення з усіх тем потрапляють до спільної черги.
    timeout: 45s
    retention: 96h
    delay: 30s
```

<span class="parent-field">subscribe.</span><a id="subscribe-queue" href="#subscribe-queue" class="field">`queue`</a> <span class="type">Map</span>  
Стандартно завжди створюється черга на рівні сервісу. `queue` дозволяє налаштувати певні атрибути цієї стандартної черги.

<span class="parent-field">subscribe.queue.</span><a id="subscribe-queue-delay" href="#subscribe-queue-delay" class="field">`delay`</a> <span class="type">Duration</span>  
Час у секундах, на який затримується доставка всіх повідомлень у черзі. стандартно 0s. Діапазон 0s-15m.

<span class="parent-field">subscribe.queue.</span><a id="subscribe-queue-retention" href="#subscribe-queue-retention" class="field">`retention`</a> <span class="type">Duration</span>  
Retention визначає час, протягом якого повідомлення залишатиметься в черзі перед видаленням. Стандартно 4d. Діапазон 60s-336h.

<span class="parent-field">subscribe.queue.</span><a id="subscribe-queue-timeout" href="#subscribe-queue-timeout" class="field">`timeout`</a> <span class="type">Duration</span>  
Timeout визначає тривалість часу, протягом якого повідомлення недоступне після доставки. Стандартно 30s. Діапазон 0s-12h.

<span class="parent-field">subscribe.queue.</span><a id="subscribe-queue-fifo" href="#subscribe-queue-fifo" class="field">`fifo`</a> <span class="type">Boolean або Map</span>  
Увімкніть FIFO (перший прийшов, перший пішов) упорядкування у вашій черзі SQS, щоб обробляти сценарії, де порядок операцій та подій є критичним, або де дублікати не можуть бути допущені.

```yaml
subscribe:
  topics:
    - name: events
      service: api
    - name: events
      service: fe
  queue: # Повідомлення з обох тем FIFO SNS надходять до спільної черги FIFO SQS.
    fifo: true
```

Коли черга увімкнена з можливостями FIFO, Copilot вимагає, щоб вихідні теми SNS [також були FIFO](../../include/publish/#publish-topics-topic-fifo).

Альтернативно, ви також можете вказати розширені конфігурації черги SQS FIFO:

```yaml
subscribe:
  topics:
    - name: events
      service: api
      queue: # Визначає специфічну для теми чергу Standard для теми api-events.
        timeout: 20s
    - name: events
      service: fe
  queue: # Стандартно повідомлення з усіх тем FIFO потрапляють до спільної черги FIFO.
    fifo:
      content_based_deduplication: true
      high_throughput: true 
```

<span class="parent-field">subscribe.queue.fifo.</span><a id="subscribe-queue-fifo-content-based-deduplication" href="#subscribe-queue-fifo-content-based-deduplication" class="field">`content_based_deduplication`</a> <span class="type">Boolean</span>  
Якщо тіло повідомлення гарантовано унікальне для кожного опублікованого повідомлення, ви можете увімкнути дедуплікацію на основі вмісту для теми SNS FIFO.

<span class="parent-field">subscribe.queue.fifo.</span><a id="subscribe-queue-fifo-deduplication-scope" href="#subscribe-queue-fifo-deduplication-scope" class="field">`deduplication_scope`</a> <span class="type">String</span>  
Для високої пропускної здатності для черг FIFO вказує, чи відбувається дедуплікація повідомлень на рівні групи повідомлень або черги. Дійсні значення: "messageGroup" і "queue".

<span class="parent-field">subscribe.queue.fifo.</span><a id="subscribe-queue-fifo-throughput-limit" href="#subscribe-queue-fifo-throughput-limit" class="field">`throughput_limit`</a> <span class="type">String</span>  
Для високої пропускної здатності для черг FIFO вказує, чи квота пропускної здатності черги FIFO застосовується до всієї черги або на рівні групи повідомлень. Дійсні значення: "perQueue" і "perMessageGroupId".

<span class="parent-field">subscribe.queue.fifo.</span><a id="subscribe-queue-fifo-high-throughput" href="#subscribe-queue-fifo-high-throughput" class="field">`high_throughput`</a> <span class="type">Boolean</span>  
Якщо увімкнено, забезпечує більшу кількість транзакцій в секунду (TPS) для повідомлень у чергах FIFO. Конфліктує з `deduplication_scope` і `throughput_limit`.

<span class="parent-field">subscribe.queue.dead_letter.</span><a id="subscribe-queue-dead-letter-tries" href="#subscribe-queue-dead-letter-tries" class="field">`tries`</a> <span class="type">Integer</span>  
Якщо вказано, створює чергу мертвих листів і політику повторного надсилання, яка перенаправляє повідомлення до DLQ після `tries` спроб. Тобто, якщо worker сервіс не вдається обробити повідомлення успішно `tries` разів, воно буде перенаправлено до DLQ для перевірки замість повторного надсилання.

<span class="parent-field">subscribe.</span><a id="subscribe-topics" href="#subscribe-topics" class="field">`topics`</a> <span class="type">Масив `topic`</span>  
Містить інформацію про те, на які теми SNS повинен підписатися worker сервіс.

<span class="parent-field">subscribe.topics.topic</span><a id="topic-name" href="#topic-name" class="field">`name`</a> <span class="type">String</span>  
Обовʼязково. Назва теми SNS для підписки.

<span class="parent-field">subscribe.topics.topic</span><a id="topic-service" href="#topic-service" class="field">`service`</a> <span class="type">String</span>  
Обовʼязково. Сервіс, який надає цю тему SNS. Разом з назвою теми це унікально ідентифікує тему SNS у середовищі copilot.

<span class="parent-field">subscribe.topics.topic</span><a id="topic-filter-policy" href="#topic-filter-policy" class="field">`filter_policy`</a> <span class="type">Map</span>  
Необовʼязково. Вкажіть політику фільтрації підписки SNS для оцінки атрибутів вхідних повідомлень відповідно до політики.  
Політику фільтрації можна вказати у форматі JSON, наприклад:

```json
filter_policy: {"store":["example_corp"],"event":[{"anything-but":"order_cancelled"}],"customer_interests":["rugby","football","baseball"],"price_usd":[{"numeric":[">=",100]}]}
```

або альтернативно як map в YAML:

```yaml
filter_policy:
  store:
    - example_corp
  event:
    - anything-but: order_cancelled
  customer_interests:
    - rugby
    - football
    - baseball
  price_usd:
    - numeric:
      - ">="
      - 100
```

Для додаткової інформації про те, як писати політики фільтрації, перегляньте [документацію SNS](https://docs.aws.amazon.com/sns/latest/dg/sns-subscription-filter-policies.html).

<span class="parent-field">subscribe.topics.topic.</span><a id="topic-queue" href="#topic-queue" class="field">`queue`</a> <span class="type">Boolean або Map</span>  
Необовʼязково. Вкажіть конфігурацію черги SQS для теми. Якщо вказано як `true`, черга буде створена з конфігурацією стандартно. Вкажіть це поле як map для налаштування певних атрибутів для цієї черги, специфічної для теми.
Якщо ви вказуєте одну або більше черг, специфічних для теми, ви можете отримати доступ до цих URI черг через змінну `COPILOT_TOPIC_QUEUE_URIS`. Ця змінна є JSON map від унікального ідентифікатора для черги, специфічної для теми, до її URI.

Наприклад, worker сервіс з чергою, специфічною для теми `orders` від сервісу `merchant`, і FIFO
тема `transactions` від сервісу `merchant` матиме наступну структуру JSON.

```json
// COPILOT_TOPIC_QUEUE_URIS
{
  "merchantOrdersEventsQueue": "https://sqs.eu-central-1.amazonaws.com/...",
  "merchantTransactionsfifoEventsQueue": "https://sqs.eu-central-1.amazonaws.com/..."
}
```

<span class="parent-field">subscribe.topics.topic.queue.</span><a id="subscribe-topics-topic-queue-fifo" href="#subscribe-topics-topic-queue-fifo" class="field">`fifo`</a> <span class="type">Boolean або Map</span>   
Необовʼязково. Вкажіть конфігурацію черги SQS FIFO для теми. Якщо вказано як `true`, черга FIFO буде створена з конфігурацією FIFO стандартно. Вкажіть це поле як map для налаштування певних атрибутів для цієї черги, специфічної для теми.

{% include 'image.uk.md' %}

{% include 'image-config.uk.md' %}

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

Якщо ви хочете використовувати потужність Fargate Spot для запуску ваших сервісів, ви можете вказати число в підполі `spot`:

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
  cpu_percentage: 70
  memory_percentage:
    value: 80
    cooldown:
      in: 80s
      out: 160s
  queue_delay:
    acceptable_latency: 10m
    msg_processing_time: 250ms
    cooldown:
      in: 30s
      out: 60s
```

<span class="parent-field">count.</span><a id="count-range" href="#count-range" class="field">`range`</a> <span class="type">String або Map</span>  
Ви можете вказати мінімальну та максимальну межу для кількості завдань, які ваш сервіс повинен підтримувати, на основі значень, які ви вказуєте для метрик.

```yaml
count:
  range: n-m
```

Це налаштує цільове значення Application Autoscaling з `MinCapacity` `n` і `MaxCapacity` `m`.

Альтернативно, якщо ви бажаєте масштабувати свій сервіс на екземпляри Fargate Spot, вкажіть `min` і `max` під `range`, а потім вкажіть `spot_from` з бажаною кількістю, з якої ви бажаєте почати розміщувати свої сервіси на потужностях Spot. Наприклад:

```yaml
count:
  range:
    min: 1
    max: 10
    spot_from: 3
```

Це встановить ваш діапазон як 1-10, як зазначено вище, але розмістить перші дві копії вашого сервісу на виділеній потужності Fargate. Якщо ваш сервіс масштабується до 3 або більше, третя та будь-які додаткові копії будуть розміщені на Spot до досягнення максимуму.

<span class="parent-field">count.range.</span><a id="count-range-min" href="#count-range-min" class="field">`min`</a> <span class="type">Integer</span>
Мінімальна бажана кількість для вашого сервісу з використанням автомасштабування.

<span class="parent-field">count.range.</span><a id="count-range-max" href="#count-range-max" class="field">`max`</a> <span class="type">Integer</span>
Максимальна бажана кількість для вашого сервісу з використанням автомасштабування.

<span class="parent-field">count.range.</span><a id="count-range-spot-from" href="#count-range-spot-from" class="field">`spot_from`</a> <span class="type">Integer</span>
Бажана кількість, з якої ви бажаєте почати розміщувати свій сервіс з використанням потужностей Fargate Spot.

<span class="parent-field">count.</span><a id="count-cooldown" href="#count-cooldown" class="field">`cooldown`</a> <span class="type">Map</span>
Поля охолодження, які використовуються як значення охолодження стандартно для всіх полів автомасштабування, що вказані.

<span class="parent-field">count.cooldown.</span><a id="count-cooldown-in" href="#count-cooldown-in" class="field">`in`</a> <span class="type">Duration</span>
Час охолодження для полів автомасштабування для збільшення масштабу сервісу.

<span class="parent-field">count.cooldown.</span><a id="count-cooldown-out" href="#count-cooldown-out" class="field">`out`</a> <span class="type">Duration</span>
Час охолодження для полів автомасштабування для зменшення масштабу сервісу.

Наступні опції `cpu_percentage` і `memory_percentage` є полями автомасштабування для `count`, які можуть бути визначені як значення поля або як map, що містить розширену інформацію про значення поля та охолодження:

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

<span class="parent-field">count.</span><a id="count-queue-delay" href="#count-queue-delay" class="field">`queue_delay`</a> <span class="type">Map</span>
Масштабування вгору або вниз для підтримки прийнятної затримки черги шляхом відстеження прийнятного відставання на завдання.  
Прийнятне відставання на завдання розраховується шляхом ділення `acceptable_latency` на `msg_processing_time`. Наприклад, якщо ви можете терпіти споживання повідомлення протягом 10 хвилин
після його прибуття і вашому завданню в середньому потрібно 250 мілісекунд для обробки повідомлення, тоді `acceptableBacklogPerTask = 10 * 60 / 0.25 = 2400`. Таким чином, кожне завдання може містити до
2400 повідомлень.  
Цільова політика відстеження налаштовується від вашого імені, щоб забезпечити масштабування вашого сервісу вгору і вниз для підтримки <= 2400 повідомлень на завдання. Щоб дізнатися більше, перегляньте [документацію](https://docs.aws.amazon.com/autoscaling/ec2/userguide/as-using-sqs-queue.html).

<span class="parent-field">count.queue_delay.</span><a id="count-queue-delay-acceptable-latency" href="#count-queue-delay-acceptable-latency" class="field">`acceptable_latency`</a> <span class="type">Duration</span>
Прийнятний час, протягом якого повідомлення може залишатися в черзі. Наприклад, `"45s"`, `"5m"`, `10h`.

<span class="parent-field">count.queue_delay.</span><a id="count-queue-delay-msg-processing-time" href="#count-queue-delay-msg-processing-time" class="field">`msg_processing_time`</a> <span class="type">Duration</span>
Середній час, необхідний для обробки повідомлення SQS. Наприклад, `"250ms"`, `"1s"`.

<span class="parent-field">count.queue_delay.</span><a id="count-queue-delay-cooldown" href="#count-queue-delay-cooldown" class="field">`cooldown`</a> <span class="type">Map</span>
Поля охолодження для масштабування вгору і вниз для затримки черги.

{% include 'exec.uk.md' %}

{% include 'deployment.uk.md' %}

```yaml 
deployment:
  rollback_alarms:
    cpu_utilization: 70    // Відсоткове значення, при досягненні або перевищенні якого спрацьовує тривога.
    memory_utilization: 50 // Відсоткове значення, при досягненні або перевищенні якого спрацьовує тривога.
    messages_delayed: 5    // Кількість затриманих повідомлень у черзі, при досягненні якої спрацьовує тривога. 
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
