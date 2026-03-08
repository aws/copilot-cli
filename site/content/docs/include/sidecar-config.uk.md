<a id="port" href="#port" class="field">`port`</a> <span class="type">Integer</span>  
Порт контейнера, який потрібно експонувати (необовʼязково).

<a id="image" href="#image" class="field">`image`</a> <span class="type">String або Map</span>  
URL-адреса образу для sidecar-контейнера (обовʼязково).

{% include 'image-config.uk.md' %}

<a id="essential" href="#essential" class="field">`essential`</a> <span class="type">Bool</span>  
Чи є sidecar-контейнер (essential) контейнером (необовʼязково, Стандартно true).

<a id="credentialsParameter" href="#credentialsParameter" class="field">`credentialsParameter`</a> <span class="type">String</span>  
ARN секрету, що містить облікові дані приватного репозиторію (необовʼязково).

<a id="variables" href="#variables" class="field">`variables`</a> <span class="type">Map</span>  
Змінні середовища для sidecar-контейнера (необовʼязково)

<a id="secrets" href="#secrets" class="field">`secrets`</a> <span class="type">Map</span>  
Секрети для передачі в sidecar-контейнер (необовʼязково)

<a id="envFile" href="#envFile" class="field">`env_file`</a> <span class="type">String</span>  
Шлях до файлу з кореневої теки вашого робочого простору, що містить змінні середовища для передачі в sidecar-контейнер. Для отримання додаткової інформації про файл змінних середовища див. [Особливості визначення файлів змінних середовища](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/taskdef-envfiles.html#taskdef-envfiles-considerations).

<a id="mount-points" href="#mount-points" class="field">`mount_points`</a> <span class="type">Array of Maps</span>  
Точки монтування для томів EFS, визначених на рівні сервісу (необовʼязково).

<span class="parent-field">mount_points.</span><a id="mount-points-source-volume" href="#mount-points-source-volume" class="field">`source_volume`</a> <span class="type">String</span>  
Том джерела для монтування в цьому sidecar (обовʼязково).

<span class="parent-field">mount_points.</span><a id="mount-points-path" href="#mount-points-path" class="field">`path`</a> <span class="type">String</span>  
Шлях всередині sidecar-контейнера, куди монтується том (обовʼязково).

<span class="parent-field">mount_points.</span><a id="mount-points-read-only" href="#mount-points-read-only" class="field">`read_only`</a> <span class="type">Boolean</span>  
Чи дозволяти sidecar доступ тільки для читання до тому (стандартно true).

<a id="labels" href="#labels" class="field">`labels`</a> <span class="type">Map</span>  
Docker-мітки для застосування до цього контейнера (необовʼязково).

<a id="depends_on" href="#depends_on" class="field">`depends_on`</a> <span class="type">Map</span>  
Залежності контейнера для застосування до цього контейнера (необовʼязково).

<a id="entrypoint" href="#entrypoint" class="field">`entrypoint`</a> <span class="type">String або Масив String</span>  
Перевизначає стандартну точку входу в sidecar.

```yaml
# Вкрсія з рядком.
entrypoint: "/bin/entrypoint --p1 --p2"
# Або, з масивом рядків.
entrypoint: ["/bin/entrypoint", "--p1", "--p2"]
```

<a id="command" href="#command" class="field">`command`</a> <span class="type">String або Масив String</span>  
Перевизначає стандартну команду в sidecar.

```yaml
# Вкрсія з рядком.
command: ps au
# Або, з масивом рядків.
command: ["ps", "au"]
```

<a id="healthcheck" href="#healthcheck" class="field">`healthcheck`</a> <span class="type">Map</span>  
Необовʼязкова конфігурація для перевірки стану sidecar-контейнера.

<span class="parent-field">healthcheck.</span><a id="healthcheck-cmd" href="#healthcheck-cmd" class="field">`command`</a> <span class="type">Array of Strings</span>  
Команда для виконання, щоб визначити, чи є sidecar-контейнер справним. Масив рядків може починатися з `CMD` для виконання аргументів команди безпосередньо або `CMD-SHELL` для виконання команди з використанням стандартної оболонки контейнера.

<span class="parent-field">healthcheck.</span><a id="healthcheck-interval" href="#healthcheck-interval" class="field">`interval`</a> <span class="type">Duration</span>  
Період часу між перевірками стану, у секундах. Стандартно 10с.

<span class="parent-field">healthcheck.</span><a id="healthcheck-retries" href="#healthcheck-retries" class="field">`retries`</a> <span class="type">Integer</span>  
Кількість спроб перед тим, як контейнер буде визнано несправним. Стандартно 2.

<span class="parent-field">healthcheck.</span><a id="healthcheck-timeout" href="#healthcheck-timeout" class="field">`timeout`</a> <span class="type">Duration</span>  
Час очікування перед тим, як вважати перевірку стану невдалою, у секундах. Стандартно 5с.

<span class="parent-field">healthcheck.</span><a id="healthcheck-start-period" href="#healthcheck-start-period" class="field">`start_period`</a> <span class="type">Duration</span>
Тривалість періоду очікування для контейнерів перед тим, як невдалі перевірки стану будуть враховані в максимальну кількість спроб. Стандартно 0с.
