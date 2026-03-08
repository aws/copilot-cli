---
title: Заплановане завдання
---

# Заплановане завдання

Список всіх доступних властивостей для маніфесту `'Scheduled Job'`. Щоб дізнатися більше про завдання Copilot, дивіться сторінку концепції [Завдання](../../concepts/jobs/).

???+ note "Приклад маніфесту Scheduled Job"

    ```yaml
        name: report-generator
        type: Scheduled Job
    
        on:
          schedule: "@daily"
        cpu: 256
        memory: 512
        retries: 3
        timeout: 1h
    
        image:
          build: ./Dockerfile
    
        variables:
          LOG_LEVEL: info
        env_file: log.env
        secrets:
          GITHUB_TOKEN: GITHUB_TOKEN
    
        # Ви можете перевизначити будь-яке з вказаних вище значень для кожного середовища.
        environments:
          prod:
            cpu: 2048
            memory: 4096
    ```

<a id="name" href="#name" class="field">`name`</a> <span class="type">String</span>  
Назва вашого завдання.

<div class="separator"></div>

<a id="type" href="#type" class="field">`type`</a> <span class="type">String</span>  
Тип архітектури для вашого завдання. Наразі Copilot підтримує лише тип "Scheduled Job" для завдань, які запускаються або за фіксованим розкладом, або періодично.

<div class="separator"></div>

<a id="on" href="#on" class="field">`on`</a> <span class="type">Map</span>  
Конфігурація для події, яка запускає ваше завдання.

<span class="parent-field">on.</span><a id="on-schedule" href="#on-schedule" class="field">`schedule`</a> <span class="type">String</span>  
Ви можете вказати частоту для періодичного запуску вашого завдання. Підтримувана частота:

| Частота      | Ідентична до          | Текст, зрозумілий людині, в `UTC`, запускається ... |
| ------------ | --------------------- | --------------------------------------------- |
| `"@yearly"`  | `"cron(0 * * * ? *)"` | опівночі 1 січня                              |
| `"@monthly"` | `"cron(0 0 1 * ? *)"` | опівночі першого дня місяця                   |
| `"@weekly"`  | `"cron(0 0 ? * 1 *)"` | опівночі в неділю                             |
| `"@daily"`   | `"cron(0 0 * * ? *)"` | опівночі                                     |
| `"@hourly"`  | `"cron(0 * * * ? *)"` | на 0-й хвилині                               |

* `"@every {duration}"` (Наприклад, "1m", "5m")
* `"rate({duration})"` на основі [виразів частоти CloudWatch](https://docs.aws.amazon.com/AmazonCloudWatch/latest/events/ScheduledEvents.html#RateExpressions)

Альтернативно, ви можете вказати розклад cron, якщо хочете запускати роботу в конкретний час:

* `"* * * * *"` на основі стандартного [формату cron](https://uk.wikipedia.org/wiki/Cron#Таблиця_crontab).
* `"cron({fields})"` на основі [виразів cron](https://docs.aws.amazon.com/AmazonCloudWatch/latest/events/ScheduledEvents.html#CronExpressions) CloudWatch з шістьма полями.

Нарешті, ви можете заборонити запуск завдання, встановивши в полі `schedule` значення `none`:

```yaml
on:
  schedule: "none"
```

<div class="separator"></div>

{% include 'image.uk.md' %}

{% include 'image-config.uk.md' %}

<div class="separator"></div>  

<a id="entrypoint" href="#entrypoint" class="field">`entrypoint`</a> <span class="type">String або Array of Strings</span>  
Перевизначення стандартного значення entrypoint в образі.

```yaml
# Версія з рядком.
entrypoint: "/bin/entrypoint --p1 --p2"
# Або, версія з масивом рядків.
entrypoint: ["/bin/entrypoint", "--p1", "--p2"]
```

<div class="separator"></div>

<a id="command" href="#command" class="field">`command`</a> <span class="type">String або Array of Strings</span>  
Перевизначення стандартного значення command в образі.

```yaml
# Версія з рядком.
command: ps au
# Або, версія з масивом рядків.
command: ["ps", "au"]
```

<div class="separator"></div>

<a id="cpu" href="#cpu" class="field">`cpu`</a> <span class="type">Integer</span>  
Кількість одиниць CPU для завдання. Допустимі значення CPU див. у [Amazon ECS docs] (https://docs.aws.amazon.com/AmazonECS/latest/developerguide/task-cpu-memory-error.html).

<div class="separator"></div>

<a id="memory" href="#memory" class="field">`memory`</a> <span class="type">Integer</span>  
Обсяг памʼяті в MiB, що використовується завданням. Допустимі значення памʼяті див. у [Amazon ECS docs] (https://docs.aws.amazon.com/AmazonECS/latest/developerguide/task-cpu-memory-error.html).

<div class="separator"></div>

<a id="platform" href="#platform" class="field">`platform`</a> <span class="type">String</span>  
Операційна система та архітектура (у форматі `[os]/[arch]`) передаються за допомогою `docker build --platform`. Наприклад, `linux/arm64` або `windows/x86_64`. Стандартно використовується `linux/x86_64`.

Перевизначає згенерований рядок для збирання з іншим допустимим `osfamily` або `architecture`. Наприклад, користувачі Windows можуть змінити рядок
```yaml
platform: windows/x86_64
```

для встановлення стандартного значення `WINDOWS_SERVER_2019_CORE`, з використанням map:

```yaml
platform:
  osfamily: windows_server_2019_full
  architecture: x86_64
```

```yaml
platform:
  osfamily: windows_server_2019_core
  architecture: x86_64
```

```yaml
platform:
  osfamily: windows_server_2022_core
  architecture: x86_64
```

```yaml
platform:
  osfamily: windows_server_2022_full
  architecture: x86_64
```

<div class="separator"></div>

<a id="retries" href="#retries" class="field">`retries`</a> <span class="type">Integer</span>  
Кількість спроб виконання завдання перед тим, як завдання буде завершено невдачею.

<div class="separator"></div>

<a id="timeout" href="#timeout" class="field">`timeout`</a> <span class="type">Duration</span>  
Скільки часу має працювати завдання, перш ніж воно перерветься і завершиться невдачею. Можна використовувати одиниці: `h`, `m` або ``.

<div class="separator"></div>

{% include 'network-vpc.uk.md' %}

<div class="separator"></div>

<a id="variables" href="#variables" class="field">`variables`</a> <span class="type">Map</span>  
Пари ключ-значення, що представляють змінні оточення, які будуть передані вашому завданню. Стандартно Copilot включає низку змінних середовища для вас.

<div class="separator"></div>

<a id="secrets" href="#secrets" class="field">`secrets`</a> <span class="type">Map</span>  
Пари ключ-значення, які представляють секретні значення зі сховища параметрів [AWS Systems Manager Parameter Store](https://docs.aws.amazon.com/systems-manager/latest/userguide/systems-manager-parameter-store.html), які будуть безпечно передані у ваше завдання як змінні середовища.

<div class="separator"></div>

<a id="storage" href="#storage" class="field">`storage`</a> <span class="type">Map</span>  
У розділі Storage ви можете вказати зовнішні томи EFS для підключення контейнерів і додаткових контейнерів sidecar. Це дозволить вам отримати доступ до постійного сховища у різних регіонах для обробки даних або робочих навантажень CMS. Докладнішу інформацію можна знайти на сторінці [storage](../../developing/storage/).

<span class="parent-field">storage.</span><a id="volumes" href="#volumes" class="field">`volumes`</a> <span class="type">Map</span>  
Визначає назву та конфігурацію будь-яких томів EFS, які ви бажаєте приєднати. Поле `volumes` вказано як map форми:

```yaml
volumes:
  <volume name>:
    path: "/etc/mountpath"
    efs:
      ...
```

<span class="parent-field">storage.volumes.</span><a id="volume" href="#volume" class="field">`<volume>`</a> <span class="type">Map</span>  
Визначає конфігурацію тому.

<span class="parent-field">storage.volumes.`<volume>`.</span><a id="path" href="#path" class="field">`path`</a> <span class="type">String</span>  
Обовʼязкове. Вкажіть місце у контейнері, куди ви бажаєте змонтувати том. Має містити не більше 242 символів і складатися лише з символів `a-zA-Z0-9.-_/`.

<span class="parent-field">storage.volumes.`<volume>`.</span><a id="read_only" href="#read-only" class="field">`read_only`</a> <span class="type">Boolean</span>  
Опціонально. Стандартно має значення `true`. Визначає, чи є том доступним лише для читання. Якщо значення false, контейнеру надаються права на файлову систему `elasticfilesystem:ClientWrite` і том стає доступним для запису.

<span class="parent-field">storage.volumes.`<volume>`.</span><a id="efs" href="#efs" class="field">`efs`</a> <span class="type">Map</span>  
Визначає більш детальну конфігурацію EFS.

<span class="parent-field">storage.volumes.`<volume>`.efs.</span><a id="id" href="#id" class="field">`id`</a> <span class="type">String</span>  
Обовʼязкове. Ідентифікатор файлової системи, яку потрібно змонтувати.

<span class="parent-field">storage.volumes.`<volume>`.efs.</span><a id="root_dir" href="#root-dir" class="field">`root_dir`</a> <span class="type">String</span>  
Опціонально. Стандартне значення `/`. Вкажіть місце у файловій системі EFS, яке ви хочете використовувати як корінь вашого тому. Має містити не більше 255 символів і складатися лише з символів `a-zA-Z0-9.-_/`. Якщо використовується точка доступу, `root_dir` має бути або порожнім, або `/`, а `auth.iam` має бути `true`.

<span class="parent-field">storage.volumes.`<volume>`.efs.</span><a id="auth" href="#auth" class="field">`auth`</a> <span class="type">Map</span>  
Вказує розширену конфігурацію авторизації для EFS.

<span class="parent-field">storage.volumes.`<volume>`.efs.auth.</span><a id="iam" href="#iam" class="field">`iam`</a> <span class="type">Boolean</span>  
Опціонально. Стандартно має значення `true`. Чи потрібно використовувати авторизацію IAM для визначення того, чи дозволено тому підключатися до EFS.

<span class="parent-field">storage.volumes.`<volume>`.efs.auth.</span><a id="access_point_id" href="#access-point-id" class="field">`access_point_id`</a> <span class="type">String</span>  
Опціонально. Стандартно має значення `""`. Ідентифікатор точки доступу до EFS, до якої потрібно підключитися. Якщо використовується точка доступу, `root_dir` має бути або порожнім, або `/`, а `auth.iam` має бути `true`.

<div class="separator"></div>

<a id="logging" href="#logging" class="field">`logging`</a> <span class="type">Map</span>  
Секція логування містить параметри конфігурації логу для драйвера логу [FireLens](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/using_firelens.html) вашого контейнера (див. приклади [тут](../../developing/sidecars/#sidecar-patterns)).

<span class="parent-field">logging.</span><a id="logging-image" href="#logging-image" class="field">`image`</a> <span class="type">Map</span>  
Опціонально. Образ Fluent Bit для використання. Стандартно використовується `public.ecr.aws/aws-observability/aws-for-fluent-bit:stable`.

<span class="parent-field">logging.</span><a id="logging-destination" href="#logging-destination" class="field">`destination`</a> <span class="type">Map</span>  
Опціонально. Параметри конфігурації для надсилання драйверу журналу FireLens.

<span class="parent-field">logging.</span><a id="logging-enableMetadata" href="#logging-enableMetadata" class="field">`enableMetadata`</a> <span class="type">Map</span>  
Опціонально. Чи включати метадані ECS до журналів. Стандартно має значення `true`.

<span class="parent-field">logging.</span><a id="logging-secretOptions" href="#logging-secretOptions" class="field">`secretOptions`</a> <span class="type">Map</span>  
Опціонально. Секрети для передачі конфігурації журналу.

<span class="parent-field">logging.</span><a id="logging-configFilePath" href="#logging-configFilePath" class="field">`configFilePath`</a> <span class="type">Map</span>  
Опціонально. Повний шлях до файлу конфігурації у вашому власному образі Fluent Bit.

{% include 'publish.uk.md' %}

<div class="separator"></div>

<a id="environments" href="#environments" class="field">`environments`</a> <span class="type">Map</span>  
Секція середовища дозволяє вам замінити будь-яке значення у вашому маніфесті залежно від середовища, в якому ви перебуваєте.
У наведеному вище прикладі маніфесту ми перевизначаємо параметр CPU, щоб наш промисловий контейнер був більш продуктивним.
