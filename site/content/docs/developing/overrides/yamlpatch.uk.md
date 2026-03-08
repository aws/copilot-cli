---
title: Перевизначення YAML Patch
---

# Перевизначення YAML Patch

{% include 'overrides-intro.uk.md' %}

## Коли слід використовувати YAML Patch замість перевизначення CDK?

Обидва варіанти є механізмом "break the glass" для доступу та налаштування функціональності, яка не відображається в Copilot [маніфестах](../../../manifest/overview/).

Ми рекомендуємо використовувати YAML patch замість [AWS Cloud Development Kit (CDK) overrides](../cdk/), якщо 1) ви не хочете мати залежність від інших інструментів та фреймворків (таких як [Node.js](https://nodejs.org) та [CDK](https://docs.aws.amazon.com/cdk/v2/guide/home.html)), або 2) вам потрібно написати лише кілька модифікацій.

## Як почати

Ви можете розширити ваш шаблон CloudFormation за допомогою YAML patches, виконавши команду `copilot [noun] override`. Наприклад, ви можете виконати `copilot svc override` для оновлення шаблону Load Balanced Web Service. Команда згенерує приклад файлу `cfn.patches.yml` у теці `copilot/[name]/overrides`.

## Як це працює?

Синтаксис `cfn.patches.yml` відповідає [RFC6902: JSON Patch](https://www.rfc-editor.org/rfc/rfc6902). Зараз, CLI підтримує три операції: `add`, `remove` та `replace`. Ось приклад файлу `cfn.patches.yml`:

```yaml
- op: add
  path: /Mappings
  value:
    ContainerSettings:
      test: { Cpu: 256, Mem: 512 }
      prod: { Cpu: 1024, Mem: 1024}
- op: remove
  path: /Resources/TaskRole
- op: replace
  path: /Resources/TaskDefinition/Properties/ContainerDefinitions/1/Essential
  value: false
- op: add
  path: /Resources/Service/Properties/ServiceConnectConfiguration/Services/0/ClientAliases/-
  value:
    Port: !Ref TargetPort
    DnsName: yamlpatchiscool
```

Кожен патч застосовується послідовно до шаблону CloudFormation. Отриманий шаблон стає ціллю наступного патча. Оцінка продовжується, доки всі патчі не будуть успішно застосовані або не буде виявлено помилку.

### Оцінка шляху

Поле `path` патчу відповідає синтаксису [RFC6901: JSON Pointer](https://www.rfc-editor.org/rfc/rfc6901).

- Кожне значення `path` розділяється символом `/`, і оцінка зупиняється, коли досягається цільова властивість CloudFormation.
- Якщо цільовий шлях є масивом, токен посилання повинен бути або:
  - символами, що складаються з цифр, починаючи з 0.
  - точно один символ `-`, коли операція є `add`, для додавання до масиву.

## Додаткові приклади

Щоб додати нову властивість до наявного ресурсу:

```yaml
- op: add
  path: /Resources/LogGroup/Properties/Tags
  value:
    - Key: keyname
      Value: value1
```

Щоб додати нову властивість у певний індекс масиву:

```yaml
- op: add
  path: /Resources/TaskDefinition/Properties/ContainerDefinitions/0/EnvironmentFiles/0
  value: arn:aws:s3:::bucket_name/key_name
```

Щоб додати новий елемент в кінець масиву:

```yaml
- op: add
  path: /Resources/TaskRole/Properties/Policies/-
  value:
    PolicyName: DynamoDBReader
    PolicyDocument:
      Version: "2012-10-17"
      Statement:
        - Effect: Allow
          Action:
            - dynamodb:Get*
          Resource: '*'
```

Щоб замінити значення наявної властивості:

```yaml
- op: replace
  path: /Resources/LogGroup/Properties/RetentionInDays
  value: 60
```

Щоб видалити елемент з масиву, потрібно вказати точний індекс:

```yaml
- op: remove
  path: /Resources/ExecutionRole/Properties/Policies/0/PolicyDocument/Statement/1/Action/0
```

Щоб видалити весь ресурс:

```yaml
- op: remove
  path: /Resources/ExecutionRole
```
