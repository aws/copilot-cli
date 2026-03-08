---
title: "Environment"
---

# Environment

Список всіх доступних властивостей для маніфесту `'Environment'`.
Щоб дізнатися більше про середовища Copilot, дивіться сторінку концепції [Середовища](../../concepts/environments/).

???+ note "Приклади маніфестів середовища"

    === "Базовий"

        ```yaml
        name: prod
        type: Environment
        observability:
          container_insights: true
        ```

    === "Імпортований VPC"

        ```yaml
        name: imported
        type: Environment
        network:
          vpc:
            id: 'vpc-12345'
            subnets:
              public:
                - id: 'subnet-11111'
                - id: 'subnet-22222'
              private:
                - id: 'subnet-33333'
                - id: 'subnet-44444'
        ```

    === "Налаштований VPC"

        ```yaml
        name: qa
        type: Environment
        network:
          vpc:
            cidr: '10.0.0.0/16'
            subnets:
              public:
                - cidr: '10.0.0.0/24'
                  az: 'us-east-2a'
                - cidr: '10.0.1.0/24'
                  az: 'us-east-2b'
              private:
                - cidr: '10.0.3.0/24'
                  az: 'us-east-2a'
                - cidr: '10.0.4.0/24'
                  az: 'us-east-2b'
        ```

    === "З публічними сертифікатами"

        ```yaml
        name: prod-pdx
        type: Environment
        http:
          public: # Застосувати існуючий сертифікат до публічного балансувальника навантаження.
            certificates:
              - arn:aws:acm:${AWS_REGION}:${AWS_ACCOUNT_ID}:certificate/13245665-cv8f-adf3-j7gd-adf876af95
        ```

    === "Приватний"

        ```yaml
        name: onprem
        type: Environment
        network:
          vpc:
            id: 'vpc-12345'
            subnets:
              private:
                - id: 'subnet-11111'
                - id: 'subnet-22222'
                - id: 'subnet-33333'
                - id: 'subnet-44444'
        http:
          private: # Застосувати існуючий сертифікат до приватного балансувальника навантаження.
            certificates:
              - arn:aws:acm:${AWS_REGION}:${AWS_ACCOUNT_ID}:certificate/13245665-cv8f-adf3-j7gd-adf876af95
            subnets: ['subnet-11111', 'subnet-22222']
        ```

    === "Мережа доставки контенту"

        ```yaml
        name: cloudfront
        type: Environment
        cdn: true
        http:
          public:
            ingress:
               cdn: true
        ```

<a id="name" href="#name" class="field">`name`</a> <span class="type">String</span>
Назва вашого середовища.

<div class="separator"></div>

<a id="type" href="#type" class="field">`type`</a> <span class="type">String</span>
Має бути встановлено як `'Environment'`.

<div class="separator"></div>

<a id="network" href="#network" class="field">`network`</a> <span class="type">Map</span>
Розділ мережі містить параметри для імпорту наявного VPC або налаштування VPC, створеного Copilot.

<span class="parent-field">network.</span><a id="network-vpc" href="#network-vpc" class="field">`vpc`</a> <span class="type">Map</span>
Розділ vpc містить параметри для налаштування параметрів CIDR та підмереж.

<span class="parent-field">network.vpc.</span><a id="network-vpc-id" href="#network-vpc-id" class="field">`id`</a> <span class="type">String</span>
Ідентифікатор VPC для імпорту. Це поле конфліктує з `cidr`.

<span class="parent-field">network.vpc.</span><a id="network-vpc-cidr" href="#network-vpc-cidr" class="field">`cidr`</a> <span class="type">String</span>
Блок IPv4 CIDR, який потрібно асоціювати з VPC, створеним Copilot. Це поле конфліктує з `id`.

<span class="parent-field">network.vpc.</span><a id="network-vpc-subnets" href="#network-vpc-subnets" class="field">`subnets`</a> <span class="type">Map</span>
Налаштуйте публічні та приватні підмережі у VPC.

Наприклад, якщо ви імпортуєте наявний VPC:

```yaml
network:
  vpc:
    id: 'vpc-12345'
    subnets:
      public:
        - id: 'subnet-11111'
        - id: 'subnet-22222'
```

Або, якщо ви налаштовуєте VPC, створений Copilot:

```yaml
network:
  vpc:
    cidr: '10.0.0.0/16'
    subnets:
      public:
        - cidr: '10.0.0.0/24'
          az: 'us-east-2a'
        - cidr: '10.0.1.0/24'
          az: 'us-east-2b'
```

<span class="parent-field">network.vpc.subnets.</span><a id="network-vpc-subnets-public" href="#network-vpc-subnets-public" class="field">`public`</a> <span class="type">Array of Subnets</span>
Список конфігурацій публічних підмереж.

<span class="parent-field">network.vpc.subnets.</span><a id="network-vpc-subnets-private" href="#network-vpc-subnets-private" class="field">`private`</a> <span class="type">Array of Subnets</span>
Список конфігурацій приватних підмереж.

<span class="parent-field">network.vpc.subnets.<type\>.</span><a id="network-vpc-subnets-id" href="#network-vpc-subnets-id" class="field">`id`</a> <span class="type">String</span>
Ідентифікатор підмережі для імпорту. Це поле конфліктує з `cidr` та `az`.

<span class="parent-field">network.vpc.subnets.<type\>.</span><a id="network-vpc-subnets-cidr" href="#network-vpc-subnets-cidr" class="field">`cidr`</a> <span class="type">String</span>
Блок IPv4 CIDR, призначений підмережі. Це поле конфліктує з `id`.

<span class="parent-field">network.vpc.subnets.<type\>.</span><a id="network-vpc-subnets-az" href="#network-vpc-subnets-az" class="field">`az`</a> <span class="type">String</span>
Назва зони доступності, призначена підмережі. Поле `az` є необовʼязковим, стандартно зони доступності призначаються в алфавітному порядку.
Це поле конфліктує з `id`.

<span class="parent-field">network.vpc.</span><a id="network-vpc-security-group" href="#network-vpc-security-group" class="field">`security_group`</a> <span class="type">Map</span>
Правила для групи безпеки середовища.

```yaml
network:
  vpc:
    security_group:
      ingress:
        - ip_protocol: tcp
          ports: 80
          cidr: 0.0.0.0/0
```

<span class="parent-field">network.vpc.security_group.</span><a id="network-vpc-security-group-ingress" href="#network-vpc-security-group-ingress" class="field">`ingress`</a> <span class="type">Array of Security Group Rules</span>
Список правил вхідного трафіку для групи безпеки.

<span class="parent-field">network.vpc.security_group.</span><a id="network-vpc-security-group-egress" href="#network-vpc-security-group-egress" class="field">`egress`</a> <span class="type">Array of Security Group Rules</span>
Список правил вихідного трафіку для групи безпеки.


<span class="parent-field">network.vpc.security_group.<type\>.</span><a id="network-vpc-security-group-ip-protocol" href="#network-vpc-security-group-ip-protocol" class="field">`ip_protocol`</a> <span class="type">String</span>
Назва або номер IP-протоколу.

<span class="parent-field">network.vpc.security_group.<type\>.</span><a id="network-vpc-security-group-ports" href="#network-vpc-security-group-ports" class="field">`ports`</a> <span class="type">String або Integer</span>
Діапазон або номер порту для правила групи безпеки.

```yaml
ports: 0-65535
```

або

```yaml
ports: 80
```

<span class="parent-field">network.vpc.security_group.<type\>.</span><a id="network-vpc-security-group-cidr" href="#network-vpc-security-group-cidr" class="field">`cidr`</a> <span class="type">String</span>
Діапазон IP-адрес у форматі CIDR.

<span class="parent-field">network.vpc.</span><a id="network-vpc-flowlogs" href="#network-vpc-flowlogs" class="field">`flow_logs`</a> <span class="type">Boolean or Map</span>
Якщо ви вказуєте 'true', Copilot увімкне журнали потоку VPC для захоплення інформації про IP-трафік, що входить і виходить з VPC середовища.
Стандартне значення для журналів потоку VPC — 14 днів (2 тижні).

```yaml
network:
  vpc:
    flow_logs: on
```

Ви можете налаштувати кількість днів для збереження:

```yaml
network:
  vpc:
    flow_logs:
      retention: 30
```

<span class="parent-field">network.vpc.flow_logs.</span><a id="network-vpc-flowlogs-retention" href="#network-vpc-flowlogs-retention" class="field">`retention`</a> <span class="type">String</span>
Кількість днів для збереження подій журналу. Дивіться [цю сторінку](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-logs-loggroup.html#cfn-logs-loggroup-retentionindays) для всіх прийнятних значень.

<div class="separator"></div>

<a id="cdn" href="#cdn" class="field">`cdn`</a> <span class="type">Boolean or Map</span>
Розділ cdn містить параметри, повʼязані з інтеграцією вашого сервісу з розподілом CloudFront. Щоб увімкнути розподіл CloudFront, вкажіть `cdn: true`.

<span class="parent-field">cdn.</span><a id="cdn-certificate" href="#cdn-certificate" class="field">`certificate`</a> <span class="type">String</span>
Сертифікат, за допомогою якого можна увімкнути HTTPS-трафік на розподілі CloudFront.
CloudFront вимагає, щоб імпортовані сертифікати були в регіоні `us-east-1`. Наприклад:

```yaml
cdn:
  certificate: "arn:aws:acm:us-east-1:1234567890:certificate/e5a6e114-b022-45b1-9339-38fbfd6db3e2"
```

<span class="parent-field">cdn.</span><a id="cdn-static-assets" href="#cdn-static-assets" class="field">`static_assets`</a> <span class="type">Map</span>
Необовʼязково. Конфігурація для статичних активів, повʼязаних з CloudFront.

<span class="parent-field">cdn.static_assets.</span><a id="cdn-static-assets-alias" href="#cdn-static-assets-alias" class="field">`alias`</a> <span class="type">String</span>
Додатковий HTTPS-доменний псевдонім для використання для статичних активів.

<span class="parent-field">cdn.static_assets.</span><a id="cdn-static-assets-location" href="#cdn-static-assets-location" class="field">`location`</a> <span class="type">String</span>
Доменне імʼя DNS для S3 кошику (наприклад, `EXAMPLE-BUCKET.s3.us-west-2.amazonaws.com`).

<span class="parent-field">cdn.static_assets.</span><a id="cdn-static-assets-path" href="#cdn-static-assets-path" class="field">`path`</a> <span class="type">String</span>
Шаблон шляху (наприклад, `static/*`), який вказує, які запити повинні бути перенаправлені до S3 кошику.

<span class="parent-field">cdn.</span><a id="cdn-tls-termination" href="#cdn-tls-termination" class="field">`terminate_tls`</a> <span class="type">Boolean</span>
Увімкнути завершення TLS для CloudFront.

<div class="separator"></div>

<a id="http" href="#http" class="field">`http`</a> <span class="type">Map</span>
Розділ http містить параметри для налаштування публічного балансувальника навантаження, який використовується [Load Balanced Web Services](../lb-web-service/)
та внутрішнього балансувальника навантаження, який використовується [Backend Services](../backend-service/).

<span class="parent-field">http.</span><a id="http-public" href="#http-public" class="field">`public`</a> <span class="type">Map</span>
Конфігурація для публічного балансувальника навантаження.

<span class="parent-field">http.public.</span><a id="http-public-certificates" href="#http-public-certificates" class="field">`certificates`</a> <span class="type">Array of Strings</span>
Список [публічних сертифікатів AWS Certificate Manager](https://docs.aws.amazon.com/acm/latest/userguide/gs-acm-request-public.html) ARNs.
Додаючи публічні сертифікати до вашого балансувальника навантаження, ви можете асоціювати ваші Load Balanced Web Services з доменним іменем і досягати їх за допомогою HTTPS.
Дивіться [Розробка/Домени](../../developing/domain/#use-domain-in-your-existing-validated-certificates) для отримання додаткової інформації про те, як повторно розгортати служби, використовуючи [`http.alias`](../lb-web-service/#http-alias).

<span class="parent-field">http.public.</span><a id="http-public-access-logs" href="#http-public-access-logs" class="field">`access_logs`</a> <span class="type">Boolean or Map</span>
Увімкнути [журнали доступу Elastic Load Balancing](https://docs.aws.amazon.com/elasticloadbalancing/latest/application/load-balancer-access-logs.html).
Якщо ви вказуєте `true`, Copilot створить S3 кошик, де публічний балансувальник навантаження буде зберігати журнали доступу.

```yaml
http:
  public:
    access_logs: true
```

Ви можете налаштувати префікс журналу:

```yaml
http:
  public:
    access_logs:
      prefix: access-logs
```

Також можливо використовувати ваш власний S3 кошик замість того, щоб дозволити Copilot створити його для вас:

```yaml
http:
  public:
    access_logs:
      bucket_name: my-bucket
      prefix: access-logs
```

<span class="parent-field">http.public.access_logs.</span><a id="http-public-access-logs-bucket-name" href="#http-public-access-logs-bucket-name" class="field">`bucket_name`</a> <span class="type">String</span>
Назва наявного S3 кошику, в якому зберігати журнали доступу.

<span class="parent-field">http.public.access_logs.</span><a id="http-public-access-logs-prefix" href="#http-public-access-logs-prefix" class="field">`prefix`</a> <span class="type">String</span>
Префікс для обʼєктів журналу.

<span class="parent-field">http.public.</span><a id="http-public-sslpolicy" href="#http-public-sslpolicy" class="field">`ssl_policy`</a> <span class="type">String</span>
Необовʼязково. Вказує політику SSL для HTTPS-слухача вашого публічного балансувальника навантаження, коли це застосовне.

<span class="parent-field">http.public.</span><a id="http-public-ingress" href="#http-public-ingress" class="field">`ingress`</a> <span class="type">Map</span><span class="version">Змінено у [v1.23.0](../../../blogs/release-v123/#move-misplaced-http-fields-in-environment-manifest-backward-compatible)</span>
Правила вхідного трафіку для обмеження трафіку публічного балансувальника навантаження.

```yaml
http:
  public:
    ingress:
      cdn: true
```

???- note "<span class="faint"> "http.public.ingress" раніше було "http.public.security_groups.ingress"</span>"
    Це поле було `http.public.security_groups.ingress` до [v1.23.0](../../../blogs/release-v123/).
    Ця зміна вплинула на дочірнє поле [`cdn`](#http-public-ingress-cdn) (єдине дочірнє поле на той час), яке раніше було `http.public.security_groups.ingress.restrict_to.cdn`.
    Для отримання додаткової інформації дивіться [блог-пост для v1.23.0](../../../blogs/release-v123/#move-misplaced-http-fields-in-environment-manifest-backward-compatible).

<span class="parent-field">http.public.ingress.</span><a id="http-public-ingress-cdn" href="#http-public-ingress-cdn" class="field">`cdn`</a> <span class="type">Boolean</span><span class="version">Змінено у [v1.23.0](../../../blogs/release-v123/#move-misplaced-http-fields-in-environment-manifest-backward-compatible)</span>
Обмежити вхідний трафік для публічного балансувальника навантаження, щоб він надходив з розподілу CloudFront.

<span class="parent-field">http.public.ingress.</span><a id="http-public-ingress-source-ips" href="#http-public-ingress-source-ips" class="field">`source_ips`</a> <span class="type">Array of Strings</span>
Обмежити вхідний трафік публічного балансувальника навантаження до вихідних IP-адрес.

```yaml
http:
  public:
    ingress:
      source_ips: ["192.0.2.0/24", "198.51.100.10/32"]
```

<span class="parent-field">http.</span><a id="http-private" href="#http-private" class="field">`private`</a> <span class="type">Map</span>
Конфігурація для внутрішнього балансувальника навантаження.

<span class="parent-field">http.private.</span><a id="http-private-certificates" href="#http-private-certificates" class="field">`certificates`</a> <span class="type">Array of Strings</span>
Список [сертифікатів AWS Certificate Manager](https://docs.aws.amazon.com/acm/latest/userguide/gs.html) ARNs.
Додаючи публічні або приватні сертифікати до вашого балансувальника навантаження, ви можете асоціювати ваші Backend Services з доменним іменем і досягати їх за допомогою HTTPS.
Дивіться [Розробка/Домени](../../developing/domain/#use-domain-in-your-existing-validated-certificates) для отримання додаткової інформації про те, як повторно розгортати служби, використовуючи [`http.alias`](../backend-service/#http-alias).

<span class="parent-field">http.private.</span><a id="http-private-subnets" href="#http-private-subnets" class="field">`subnets`</a> <span class="type">Array of Strings</span>
Ідентифікатори підмереж для розміщення внутрішнього балансувальника навантаження.

<span class="parent-field">http.private.</span><a id="http-private-ingress" href="#http-private-ingress" class="field">`ingress`</a> <span class="type">Map</span><span class="version">Змінено у [v1.23.0](../../../blogs/release-v123/#move-misplaced-http-fields-in-environment-manifest-backward-compatible)</span>
Правила вхідного трафіку для внутрішнього балансувальника навантаження.
```yaml
http:
  private:
    ingress:
      vpc: true  # Увімкнути вхідний трафік у межах VPC до внутрішнього балансувальника навантаження.
```

???- note "<span class="faint"> "http.private.ingress" раніше було "http.private.security_groups.ingress"</span>"
    Це поле було `http.private.security_groups.ingress` до [v1.23.0](../../../blogs/release-v123/).
    Ця зміна вплинула на дочірнє поле [`vpc`](#http-private-ingress-vpc) (єдине дочірнє поле на той час),
    яке раніше було `http.private.security_groups.ingress.from_vpc`.
    Для отримання додаткової інформації дивіться [блог-пост для v1.23.0](../../../blogs/release-v123/#move-misplaced-http-fields-in-environment-manifest-backward-compatible).

<span class="parent-field">http.private.ingress.</span><a id="http-private-ingress-vpc" href="#http-private-ingress-vpc" class="field">`vpc`</a> <span class="type">Boolean</span><span class="version">Змінено у [v1.23.0](../../../blogs/release-v123/#move-misplaced-http-fields-in-environment-manifest-backward-compatible)</span>
Увімкнути трафік з меж VPC до внутрішнього балансувальника навантаження.

<span class="parent-field">http.private.</span><a id="http-private-sslpolicy" href="#http-private-sslpolicy" class="field">`ssl_policy`</a> <span class="type">String</span>
Необовʼязково. Вказує політику SSL для HTTPS-слухача вашого внутрішнього балансувальника навантаження, коли це застосовне.

<div class="separator"></div>

<a id="observability" href="#observability" class="field">`observability`</a> <span class="type">Map</span>
Розділ спостережуваності дозволяє налаштувати способи збору даних про служби та завдання, розгорнуті у вашому середовищі.

<span class="parent-field">observability.</span><a id="http-container-insights" href="#http-container-insights" class="field">`container_insights`</a> <span class="type">Bool</span>
Чи увімкнути [CloudWatch container insights](https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/ContainerInsights.html) у кластері ECS вашого середовища.
