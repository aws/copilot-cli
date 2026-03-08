---
title: Доставка контенту
---

# Глобальна доставка контенту

Copilot підтримує Мережу доставки контенту через Amazon CloudFront. Цей ресурс керується Copilot на рівні середовища, дозволяючи користувачам використовувати CloudFront через [маніфест середовища](../../manifest/environment/).

## Інфраструктура CloudFront з Copilot

Коли Copilot створює [дистрибутив CloudFront](https://docs.aws.amazon.com/AmazonCloudFront/latest/DeveloperGuide/distribution-overview.html), він створює дистрибутив як нову точку входу до застосунку замість Application Load Balancer. Це дозволяє CloudFront швидше направляти ваш трафік до балансувальника навантаження через граничні локації, розгорнуті по всьому світу.

## Як використовувати CloudFront з наявним застосунком?

Починаючи з версії Copilot v1.20, `copilot env init` створює файл маніфесту середовища. У цьому маніфесті ви можете вказати значення `cdn: true` і потім виконати `copilot env deploy` для активації базового дистрибутиву CloudFront.

???+ note "Приклади налаштування дистрибутиву CloudFront в маніфесті"

    === "Базовий"

        ```yaml
        cdn: true

        http:
          public:
            security_groups:
              ingress:
                restrict_to:
                  cdn: true
        ```

    === "Імпортовані сертифікати"

        ```yaml
        cdn:
          certificate: arn:aws:acm:us-east-1:${AWS_ACCOUNT_ID}:certificate/13245665-h74x-4ore-jdnz-avs87dl11jd

        http:
          certificates:
            - arn:aws:acm:${AWS_REGION}:${AWS_ACCOUNT_ID}:certificate/13245665-bldz-0an1-afki-p7ll1myafd
            - arn:aws:acm:${AWS_REGION}:${AWS_ACCOUNT_ID}:certificate/56654321-cv8f-adf3-j7gd-adf876af95
        ```

## Як увімкнути HTTPS трафік з CloudFront?

При використанні HTTPS з CloudFront, вкажіть ваші сертифікати в полі `cdn.certificate` маніфесту середовища, так само як ви б це зробили в полі `http.certificates` для Load Balancer. На відміну від Load Balancer, ви можете імпортувати лише один сертифікат. Через це ми рекомендуємо створити новий сертифікат (в регіоні `us-east-1`) з CNAME записами для валідації кожного псевдоніму, який використовують ваші сервіси в цьому середовищі.

!!! info "Інформація"
    CloudFront підтримує лише сертифікати, імпортовані в регіоні `us-east-1`.

!!! info "Інформація"
    Імпорт сертифіката для CloudFront додає додатковий дозвіл до вашої ролі Environment Manager, дозволяючи Copilot використовувати [API-виклик](https://docs.aws.amazon.com/acm/latest/APIReference/API_DescribeCertificate.html) `DescribeCertificate`.

Ви також можете дозволити Copilot керувати сертифікатами, вказавши `--domain` при створенні застосунку. При цьому ви повинні вказати `http.alias` для всіх ваших сервісів, розгорнутих у середовищі з увімкненим CloudFront.

З обома налаштуваннями Copilot налаштує CloudFront для використання [SSL/TLS сертифіката](https://docs.aws.amazon.com/AmazonCloudFront/latest/DeveloperGuide/using-https-alternate-domain-names.html), що дозволяє перевіряти сертифікат переглядача та вмикати HTTPS зʼєднання.

## Що таке обмеження вхідного трафіку?

Ви можете обмежити вхідний трафік, щоб він надходив лише з певного джерела. Для CloudFront Copilot використовує [керований AWS список префіксів](https://docs.aws.amazon.com/vpc/latest/userguide/working-with-aws-managed-prefix-lists.html) для обмеження дозволеного трафіку набором IP-адрес CIDR, повʼязаних з граничними локаціями CloudFront. Коли ви вказуєте `restrict_to.cdn: true`, ваш публічний Load Balancer більше не буде загальнодоступним і доступ до нього можливий лише через дистрибутив CloudFront, захищаючи від загроз безпеці ваших сервісів.

## Як використовувати CloudFront для термінації TLS?

!!! attention "Увага"
    1. Вимкніть [перенаправлення HTTP на HTTPS](../../manifest/lb-web-service/#http-redirect-to-https) для ваших вебсервісів з балансуванням навантаження.
    2. Запустіть `svc deploy` окремо для повторного розгортання всіх вебсервісів з балансуванням навантаження перед увімкненням завершення TLS CloudFront.
    3. Після того, як усі ваші вебсервіси з балансуванням навантаження більше не перенаправляють HTTP на HTTPS, ви можете безпечно увімкнути завершення TLS CloudFront у маніфесті середовища та запустити `env deploy`.

Ви можете опціонально використовувати CloudFront для термінації TLS, налаштувавши маніфест середовища як

```yaml
cdn:
  terminate_tls: true
```

І трафік від `CloudFront → Application Load Balancer (ALB) → ECS` буде лише HTTP. Це приносить користь термінації TLS на географічно ближчій кінцевій точці до кінцевого користувача для швидших рукостискань TLS.

## Як використовувати CloudFront з S3 кошиком?

Ви можете опціонально використовувати CloudFront з кошиком Amazon S3 для швидшої доставки статичного контенту, налаштувавши `cdn.static_assets` у маніфесті середовища.

### Використання наявного кошику S3

!!! attention "Увага"
    З міркувань безпеки ми рекомендуємо використовувати **приватний** S3 кошик, щоб весь публічний доступ був стандартно заблокований.

Приклад маніфесту середовища нижче ілюструє, як використовувати наявний кошик S3 для CloudFront:

???+ note "Приклад налаштування маніфесту середовища для використання наявного кошика S3 з CloudFront"
    ```yaml
    cdn:
      static_assets:
        location: cf-s3-ecs-demo-bucket.s3.us-west-2.amazonaws.com
        alias: example.com
        path: static/*
    ```

Зверніть увагу, що `static_assets.location` є DNS-доменним іменем S3 кошика (наприклад, `EXAMPLE-BUCKET.s3.us-west-2.amazonaws.com`). Якщо ви не використовуєте псевдонім [кореневого домену, повʼязаного з застосунком](../domain/#use-app-associated-root-domain), не забудьте створити A-запис для вашого псевдоніма, що вказує на доменне імʼя CloudFront.

Після розгортання середовища за допомогою маніфесту середовища, вам потрібно оновити політику вашого S3 кошика (якщо він приватний), щоб CloudFront міг отримати до нього доступ.

???+ note "Приклад політики S3 кошика, яка надає доступ лише для читання до CloudFront"
    ```json
    {
        "Version": "2012-10-17",
        "Statement": {
            "Sid": "AllowCloudFrontServicePrincipalReadOnly",
            "Effect": "Allow",
            "Principal": {
                "Service": "cloudfront.amazonaws.com"
            },
            "Action": "s3:GetObject",
            "Resource": "arn:aws:s3:::EXAMPLE-BUCKET/*",
            "Condition": {
                "StringEquals": {
                    "AWS:SourceArn": "arn:aws:cloudfront::111122223333:distribution/EDFDVBD6EXAMPLE"
                }
            }
        }
    }
    ```

## Як використовувати CloudFront для обслуговування статичного вебсайту?

Використовуйте команду [copilot init](../../commands/init/) або [copilot svc init](../../commands/svc-init/) для створення робочого навантаження статичного сайту. Після вибору файлів для завантаження Copilot створить окремий, спеціальний дистрибутив CloudFront, а також S3 кошик з вашими активами. З кожним повторним розгортанням Copilot буде анулювати наявний кеш для більш динамічної розробки.
