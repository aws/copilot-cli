---
title: Внутрішні балансувальники навантаження застосунків
---

# Внутрішні балансувальники навантаження застосунків

Стандартно, ALB, створені для середовищ із вебсервісами з балансуванням навантаження, є [загальнодоступними](https://docs.aws.amazon.com/elasticloadbalancing/latest/classic/elb-internet-facing-load-balancers.html). Щоб створити [внутрішній балансувальник навантаження](https://docs.aws.amazon.com/elasticloadbalancing/latest/classic/elb-internal-load-balancers.html), вузли якого мають лише приватні IP-адреси, вам потрібно налаштувати кілька параметрів у вашому середовищі та робочому навантаженні.

## Середовище

Внутрішній балансувальник навантаження є ресурсом на рівні середовища, яким можуть користуватися інші дозволені сервіси.
Щоб увімкнути HTTPS для вашого балансувальника навантаження, змініть [маніфест середовища](../../manifest/environment/#http-private), щоб
імпортувати ARN ваших наявних сертифікатів.

Якщо до балансувальника навантаження не застосовано жодних сертифікатів, Copilot повʼяже балансувальник з кінцевою точкою `http://{env name}.{app name}.internal`, а окремі сервіси будуть доступні за адресою `http://{service name}.{env name}.{app name}.internal`.
```go
// Щоб отримати доступ до сервісу "api" за внутрішнім балансувальником навантаження
endpoint := fmt.Sprintf("http://api.%s.%s.internal", os.Getenv("COPILOT_ENVIRONMENT_NAME"), os.Getenv("COPILOT_APPLICATION_NAME"))
resp, err := http.Get(endpoint)
```

## Сервіс

Єдиним типом сервісу, який можна розмістити за внутрішнім балансувальником навантаження, є [Backend Service](../../concepts/services/#backend-service). Щоб вказати Copilot створити внутрішній ALB у середовищі, де ви розгортаєте цей сервіс, додайте поле `http` до маніфесту робочого навантаження вашого Backend Service:

```yaml
# in copilot/{service name}/manifest.yml
http:
  path: '/'
network:
  vpc:
    placement: private
```

Якщо у вас є наявний внутрішній ALB у VPC, до якого буде розгорнуто ваш сервіс, ви можете імпортувати його для кожного Backend Service, вказавши його у вашому маніфесті перед розгортанням:

```yaml
http:
  path: '/'
  alb: [name or ARN]
```

## Розширена конфігурація

### Розміщення підмереж

Ви можете точно вказати, у яких приватних підмережах буде розміщено ваш внутрішній ALB.

Коли ви запускаєте `copilot env init`, використовуйте прапорець [`--internal-alb-subnets`](../../commands/env-init/#what-are-the-flags), щоб передати ідентифікатори підмереж, у яких ви хочете розмістити ALB.

### Псевдоніми, перевірки стану та інше

Поле `http` для Backend Services має всі підполя та можливості, які має поле `http` для Load Balanced Web Services.

``` yaml
http:
  path: '/'
  healthcheck:
    path: '/_healthcheck'
    port: 8080
    success_codes: '200,301'
    healthy_threshold: 3
    unhealthy_threshold: 2
    interval: 15s
    timeout: 10s
    grace_period: 45s
  deregistration_delay: 5s
  stickiness: false
  allowed_source_ips: ["10.24.34.0/23"]
  alias: example.com
```

Для `alias` ви можете 1. використовувати наявні приватні зони хостингу, або 2. додати власні записи псевдонімів після розгортання, незалежно від Copilot. Ви можете додати один псевдонім:

```yaml
http:
  alias: example.com
  hosted_zone: HostedZoneID1
```

або кілька псевдонімів, які використовують одну зону хостингу:

```yaml
http:
  alias: ["example.com", "www.example.com"]
  hosted_zone: HostedZoneID1
```

або кілька псевдонімів, деякі з яких використовують верхньорівневу зону хостингу:

```yaml
http:
  hosted_zone: HostedZoneID1
  alias:
    - name: example.com
    - name: www.example.com
    - name: something-different.com
      hosted_zone: HostedZoneID2
```
