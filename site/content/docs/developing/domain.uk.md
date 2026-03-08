---
title: Домен
---

# Домен

## Вебсервіс з балансуванням навантаження

У Copilot є два способи використання користувацьких доменів для вашого вебсервісу з балансуванням навантаження:

1. Використовуйте `--domain` при створенні застосунку для асоціації домену Route 53 в тому ж обліковому записі.
2. Використовуйте поле [`http.[public/private].certificates`](../../manifest/environment/#http-public-certificates) у маніфесті середовища для імпорту ваших перевірених сертифікатів ACM у середовище.

!!!attention "Увага"
    На сьогодні доменне імʼя Route 53 можна асоціювати лише при запуску `copilot app init`.
    Якщо ви хочете оновити свій застосунок з новим доменом ([#3045](https://github.com/aws/copilot-cli/issues/3045)), вам потрібно буде запустити `copilot app delete`, щоб видалити старий, перед створенням нового з `--domain` для асоціації нового домену.

### Використання кореневого домену, асоційованого з застосунком <a id="use-app-associated-root-domain"></a>

Як зазначено в [посібнику Застосунки](../../concepts/applications/#additional-app-configurations), ви можете налаштувати доменне імʼя вашого застосунку при запуску `copilot app init --domain`.

**Типове доменне імʼя для вашого сервісу**

Після розгортання ваших [Вебсервісів з балансуванням навантаження](../../concepts/services/#load-balanced-web-service), стандартно ви можете отримати до них публічний доступ через

```
${SvcName}.${EnvName}.${AppName}.${DomainName}
```

якщо ви використовуєте Application Load Balancer, або

```
${SvcName}-nlb.${EnvName}.${AppName}.${DomainName}
```

якщо ви використовуєте Network Load Balancer.

Наприклад, `https://kudo.test.coolapp.example.aws` або `kudo-nlb.test.coolapp.example.aws:443`.

#### Налаштований доменний псевдонім <a id="customized-domain-alias"></a>

Якщо вам не подобається типове доменне імʼя, яке Copilot призначає вашому сервісу, ви можете встановити власний [псевдонім](https://docs.aws.amazon.com/Route53/latest/DeveloperGuide/resource-record-sets-choosing-alias-non-alias.html) для вашого сервісу, відредагувавши розділ `alias` у вашому [маніфесті](../../manifest/lb-web-service/#http-alias). Наступний фрагмент коду встановлює псевдонім для вашого сервісу.

``` yaml
# у copilot/{service name}/manifest.yml
http:
  path: '/'
  alias: example.aws
```

Аналогічно, якщо ваш сервіс використовує Network Load Balancer, ви можете вказати:

```yaml
nlb:
  port: 443/tls
  alias: example-v1.aws
```

Однак, оскільки ми [делегуємо відповідальність за субдомен Route 53](https://docs.aws.amazon.com/Route53/latest/DeveloperGuide/CreatingNewSubdomain.html#UpdateDNSParentDomain), псевдонім, який ви вказуєте, повинен відповідати одному з наступних шаблонів, підтримуваних Copilot:

- `{domain}`, наприклад `example.aws`
- `{subdomain}.{domain}`, наприклад `v1.example.aws`
- `{appName}.{domain}`, наприклад `coolapp.example.aws`
- `{subdomain}.{appName}.{domain}`, наприклад `v1.coolapp.example.aws`
- `{envName}.{appName}.{domain}`, наприклад `test.coolapp.example.aws`
- `{subdomain}.{envName}.{appName}.{domain}`, наприклад `v1.test.coolapp.example.aws`

#### Що відбувається під капотом?

Під капотом, Copilot

* створює хост-зону у вашому обліковому записі застосунку для нового субдомену застосунку `${AppName}.${DomainName}`
* створює ще одну хост-зону у вашому обліковому записі середовища для нового субдомену середовища `${EnvName}.${AppName}.${DomainName}`
* створює та перевіряє сертифікат ACM для субдомену середовища
* асоціює сертифікат з:
  - Вашим HTTPS слухачем та перенаправляє HTTP трафік на HTTPS, якщо псевдонім використовується для Application Load Balancer (`http.alias`)
  - Вашим TLS слухачем Network Load Balancer, якщо псевдонім використовується для `nlb.alias` і увімкнено термінацію TLS.
* створює необовʼязковий A запис для вашого псевдоніму

#### Як це виглядає?

<iframe width="560" height="315" src="https://www.youtube.com/embed/Oyr-n59mVjI" title="YouTube video player" frameborder="0" allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture" allowfullscreen></iframe>

### Використання домену у ваших поточних перевірених сертифікатах <a id="use-domain-in-your-existing-validated-certificates"></a>

Якщо ви хочете мати більш детальний контроль над створеним сертифікатом ACM, або якщо [типові опції `alias`](#customized-domain-alias) не є достатньо гнучкими, ви можете імпортувати перевірені сертифікати ACM, які включають псевдонім, у ваше середовище. У [маніфесті середовища](../../manifest/environment/) вкажіть `http.[public/private].certificates`:

```yaml
type: Environment
http:
  public:
    certificates:
      - arn:aws:acm:us-east-1:123456789012:certificate/12345678-1234-1234-1234-123456789012
```

Потім, у маніфесті вашого сервісу, ви можете:

- Вказати ID [`hosted_zone`](../../manifest/lb-web-service/#http-hosted-zone), у яку Copilot повинен вставити A запис:

  ``` yaml
  # у copilot/{service name}/manifest.yml
  http:
    path: '/'
    alias: example.aws
    hosted_zone: Z0873220N255IR3MTNR4
  ```

- Або, розгорнути сервіс без поля `hosted_zone`, а потім вручну додати DNS імʼя Application Load Balancer (ALB), створеного у цьому середовищі, як A запис там, де розміщений ваш псевдонім домену.

У нас є [приклад](../../../blogs/release-v118/#certificate-import) Опції 2 у наших блогах.

## Вебсервіс на запит <a id="request-driven-web-service"></a>

Ви також можете додати [користувацький домен](https://docs.aws.amazon.com/apprunner/latest/dg/manage-custom-domains.html) для вашого вебсервісу на запит. Подібно до Вебсервісу з балансуванням навантаження, ви можете зробити це, змінивши поле [`alias`](../../manifest/rd-web-service/#http-alias) у вашому маніфесті:

```yaml
# у copilot/{service name}/manifest.yml
http:
  path: '/'
  alias: web.example.aws
```

Так само, ваш застосунок повинен бути асоційований з доменом (наприклад, `example.aws`), щоб ваш Вебсервіс на запит міг його використовувати.

!!!info "Інформація"
    Зараз ми підтримуємо лише субдомени першого рівня, такі як `web.example.aws`.

    Доменів рівня середовища (наприклад, `web.${envName}.${appName}.example.aws`), доменів рівня застосунку (наприклад, `web.${appName}.example.aws`), або кореневих доменів (тобто `example.aws`) поки що не підтримується. Це також означає, що ваш субдомен не повинен збігатися з імʼям вашого застосунку.

Під капотом, Copilot:

* асоціює домен з вашим сервісом app runner
* створює запис домену, а також записи перевірки у хост-зоні вашого кореневого домену
