<span class="parent-field">image.</span><a id="image-build" href="#image-build" class="field">`build`</a> <span class="type">String або Map</span>  
Створіть контейнер з Dockerfile з додатковими аргументами. Несумісний з [`image.location`](#image-location).

Якщо ви вказуєте рядок, Copilot інтерпретує його як шлях до вашого Dockerfile. Він припустить, що dirname вказаного вами рядка має бути контекстом збірки. Маніфест:

```yaml
image:
  build: path/to/dockerfile
```

призведе до наступного виклику docker build: `$ docker build --file path/to/dockerfile path/to`.

Ви також можете вказати build у вигляді map:

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

У цьому випадку Copilot використає вказаний вами теку контексту і перетворить пари ключ-значення у args у перевизначення --build-arg. Еквівалентний виклик docker build буде таким:
`$ docker build --file path/to/dockerfile --target build-stage --cache-from image:tag --build-arg key=value context/dir`.

Ви можете опустити поля, і Copilot зробить все можливе, щоб зрозуміти, що ви маєте на увазі. Наприклад, якщо ви вкажете `context`, але не `dockerfile`, Copilot запустить Docker у теці контексту і вважатиме, що ваш Docker-файл має назву "Dockerfile". Якщо ви вкажете `dockerfile`, але не `context`, Copilot вважатиме, що ви хочете запустити Docker у теці, яка містить `dockerfile`.

Усі шляхи належать до кореня вашої робочої області.

<span class="parent-field">image.</span><a id="image-location" href="#image-location" class="field">`location`</a> <span class="type">String</span>  
Замість того, щоб збирати контейнер з Dockerfile, ви можете вказати імʼя наявного образу. Взаємовиключно з [`image.build`](#image-build).
Поле `location` має таке саме визначення, як і параметр [`image`](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/task_definition_parameters.html#container_definition_image) у визначенні завдання Amazon ECS.

!!! warning "Попередження"
    Якщо ви передаєте образ Windows, ви повинні додати `platform: windows/x86_64` до вашого маніфесту.  
    Якщо ви передаєте образ на основі архітектури ARM, ви повинні додати `platform: linux/arm64` до вашого маніфесту.

<span class="parent-field">image.</span><a id="image-credential" href="#image-credential" class="field">`credentials`</a> <span class="type">String</span>  
Необовʼязковий ARN облікових даних для приватного сховища. Поле `credentials` має таке саме визначення, як і поле [`credentialsParameter`](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/private-auth.html) у визначенні завдання Amazon ECS.

<span class="parent-field">image.</span><a id="image-labels" href="#image-labels" class="field">`labels`</a> <span class="type">Map</span>  
Опціональний map ключ/значення [міток Docker labels](https://docs.docker.com/config/labels-custom-metadata/) для додавання до контейнера.

<span class="parent-field">image.</span><a id="image-depends-on" href="#image-depends-on" class="field">`depends_on`</a> <span class="type">Map</span>  
Опціональний map ключ/значення [Container Dependencies](https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_ContainerDependency.html) для додавання до контейнера. Ключем map є імʼя контейнера, а значенням є умова, від якої залежить залежність. Допустимими умовами є: `start`, `healthy`, `complete` та `success`. Ви не можете вказати залежність `complete` або `success` для основного контейнера.

Наприклад:

```yaml
image:
  build: ./Dockerfile
  depends_on:
    nginx: start
    startup: success
```

У наведеному вище прикладі головний контейнер завдання буде запущено лише після того, як буде запущено контейнер `nginx` і успішно завершиться контейнер `startup`.
