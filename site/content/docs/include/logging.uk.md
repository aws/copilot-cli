<div class="separator"></div>

<a id="logging" href="#logging" class="field">`logging`</a> <span class="type">Map</span>  
Розділ журналювання містить конфігурацію логів. Ви також можете налаштувати параметри для драйвера журналювання [FireLens](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/using_firelens.html) вашого контейнера в цьому розділі (дивіться приклади [тут](../../developing/sidecars/#sidecar-patterns)).

<span class="parent-field">logging.</span><a id="retention" href="#logging-retention" class="field">`retention`</a> <span class="type">Integer</span>  
Необовʼязково. Кількість днів зберігання подій журналу. Дивіться [цю сторінку](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-logs-loggroup.html#cfn-logs-loggroup-retentionindays) для всіх прийнятних значень. Якщо не вказано, стандартно встановлено 30.

<span class="parent-field">logging.</span><a id="logging-image" href="#logging-image" class="field">`image`</a> <span class="type">Map</span>  
Необовʼязково. Образ Fluent Bit для використання. Стандартно `public.ecr.aws/aws-observability/aws-for-fluent-bit:stable`.

<span class="parent-field">logging.</span><a id="logging-destination" href="#logging-destination" class="field">`destination`</a> <span class="type">Map</span>  
Необовʼязково. Параметри конфігурації для відправлення драйверу журналювання FireLens.

<span class="parent-field">logging.</span><a id="logging-enableMetadata" href="#logging-enableMetadata" class="field">`enableMetadata`</a> <span class="type">Map</span>  
Необовʼязково. Чи включати метадані ECS у логи. Стандартно `true`.

<span class="parent-field">logging.</span><a id="logging-secretOptions" href="#logging-secretOptions" class="field">`secretOptions`</a> <span class="type">Map</span>  
Необовʼязково. Секрети для передачі в конфігурацію журналювання.

<span class="parent-field">logging.</span><a id="logging-configFilePath" href="#logging-configFilePath" class="field">`configFilePath`</a> <span class="type">Map</span>  
Необовʼязково. Повний шлях до файлу конфігурації у вашому власному образі Fluent Bit.

<span class="parent-field">logging.</span><a id="logging-envFile" href="#logging-envFile" class="field">`env_file`</a> <span class="type">String</span>  
Шлях до файлу з кореневої теки вашого робочого простору, що містить змінні середовища для передачі в контейнер журналювання sidecar. Для отримання додаткової інформації про файл змінних середовища дивіться [Особливості визначення файлів змінних середовища](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/taskdef-envfiles.html#taskdef-envfiles-considerations).
