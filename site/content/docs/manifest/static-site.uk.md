---
title: "Статичний сайт"
---

# Статичний сайт

Список усіх доступних властивостей для маніфесту `'Static Site'`.

???+ note "Приклад маніфесту Static Site"

    ```yaml
    name: example
    type: Static Site

    http:
      alias: 'example.com'

    files:
      - source: src/someDirectory
        recursive: true
      - source: someFile.html
    
    # Ви можете перевизначити будь-яке з наведених вище значень для кожного середовища.
    # environments:
    #   test:
    #     files:
    #       - source: './blob'
    #         destination: 'assets'
    #         recursive: true
    #         exclude: '*'
    #         reinclude:
    #           - '*.txt'
    #           - '*.png'
    ```

<a id="name" href="#name" class="field">`name`</a> <span class="type">String</span>  
Назва вашого сервісу.

<div class="separator"></div>

<a id="type" href="#type" class="field">`type`</a> <span class="type">String</span>  
Тип архітектури вашого сервісу. [Статичний сайт](../../concepts/services/#static-site) — це сервіс, що доступний в інтернет і розміщений на Amazon S3.

<div class="separator"></div>

<a id="http" href="#http" class="field">`http`</a> <span class="type">Map</span>  
Конфігурація для вхідного трафіку на ваш сайт.

<span class="parent-field">http.</span><a id="http-alias" href="#http-alias" class="field">`alias`</a> <span class="type">String</span>  
HTTPS доменний псевдонім вашого сервісу.

<span class="parent-field">http.</span><a id="http-certificate" href="#http-certificate" class="field">`certificate`</a> <span class="type">String</span>  
ARN сертифіката, який використовується для вашого HTTPS трафіку.
CloudFront вимагає, щоб імпортовані сертифікати були в регіоні `us-east-1`. Наприклад:

```yaml
http:
  alias: example.com
  certificate: "arn:aws:acm:us-east-1:1234567890:certificate/e5a6e114-b022-45b1-9339-38fbfd6db3e2"
```

<div class="separator"></div>

<a id="files" href="#files" class="field">`files`</a> <span class="type">Array of Maps</span>  
Параметри, повʼязані з вашими статичними ресурсами.

<span class="parent-field">files.</span><a id="files-source" href="#files-source" class="field">`source`</a> <span class="type">String</span>  
Шлях, відносно кореня вашого робочого простору, до теки або файлу, який потрібно завантажити на S3.

<span class="parent-field">files.</span><a id="files-recursive" href="#files-recursive" class="field">`recursive`</a> <span class="type">Boolean</span>  
Чи слід завантажувати вихідну теку рекурсивно. Стандартно true для тек.

<span class="parent-field">files.</span><a id="files-destination" href="#files-destination" class="field">`destination`</a> <span class="type">String</span>  
Необовʼязково. Підшлях, який буде додано до ваших файлів у вашому S3 кошику. Стандартно — `.`

<span class="parent-field">files.</span><a id="files-exclude" href="#files-exclude" class="field">`exclude`</a> <span class="type">String</span>  
Необовʼязково. Патерн-фільтри [filters](https://awscli.amazonaws.com/v2/documentation/api/latest/reference/s3/index.html#use-of-exclude-and-include-filters) для виключення файлів з завантаження. Допустимі символи:  
`*` (відповідає всьому)  
`?` (відповідає будь-якому одному символу)  
`[sequence]` (відповідає будь-якому символу в `sequence`)  
`[!sequence]` (відповідає будь-якому символу, якого немає в `sequence`)  

<span class="parent-field">files.</span><a id="files-reinclude" href="#files-reinclude" class="field">`reinclude`</a> <span class="type">String</span>  
Необовʼязково. Патерн-фільтри [filters](https://awscli.amazonaws.com/v2/documentation/api/latest/reference/s3/index.html#use-of-exclude-and-include-filters) для повторного включення файлів, які були виключені з завантаження за допомогою [`exclude`](#files-exclude). Допустимі символи:  
`*` (відповідає всьому)  
`?` (відповідає будь-якому одному символу)  
`[sequence]` (відповідає будь-якому символу в `sequence`)  
`[!sequence]` (відповідає будь-якому символу, якого немає в `sequence`)
